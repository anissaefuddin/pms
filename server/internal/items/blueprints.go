package items

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxBlueprintNodes = 200

// blueprintNode is one stored item. Parent is the index of an earlier node (-1 for the root).
// Offsets are calendar days from the tree's earliest date. Comments, attachments, approvals and
// shares are not copied.
type blueprintNode struct {
	Parent        int                        `json:"parent"`
	Type          string                     `json:"type"`
	Title         string                     `json:"title"`
	Description   json.RawMessage            `json:"description,omitempty"`
	Importance    string                     `json:"importance"`
	StatusID      *uuid.UUID                 `json:"status_id,omitempty"`
	AssigneeIDs   []uuid.UUID                `json:"assignee_ids"`
	StartOffset   *int                       `json:"start_offset,omitempty"`
	DueOffset     *int                       `json:"due_offset,omitempty"`
	DurationDays  *int                       `json:"duration_days,omitempty"`
	IsMilestone   bool                       `json:"is_milestone"`
	EffortMinutes *int                       `json:"effort_minutes,omitempty"`
	Fields        map[string]json.RawMessage `json:"fields,omitempty"`
}

type blueprintDependency struct {
	Predecessor int    `json:"predecessor"`
	Successor   int    `json:"successor"`
	Type        string `json:"type"`
	Lag         int    `json:"lag"`
}

type blueprintTree struct {
	Nodes        []blueprintNode       `json:"nodes"`
	Dependencies []blueprintDependency `json:"dependencies"`
}

// SaveBlueprint copies an item and the descendants the actor can see into a blueprint of the Space.
func SaveBlueprint(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID uuid.UUID, name string, sourceID uuid.UUID) (json.RawMessage, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	out, err := saveBlueprint(ctx, tx, actor, spaceID, name, sourceID)
	if err == nil {
		err = tx.Commit(ctx)
	}
	return out, writeError(err)
}

func saveBlueprint(ctx context.Context, tx pgx.Tx, actor policy.Actor, spaceID uuid.UUID, name string, sourceID uuid.UUID) (json.RawMessage, error) {
	if err := requireSpace(ctx, tx, actor, policy.Edit, spaceID); err != nil {
		return nil, err
	}
	if n := len([]rune(name)); n < 1 || n > 120 {
		return nil, InvalidField{"name", "name must be 1 to 120 characters"}
	}
	rows, err := tx.Query(ctx, `
WITH RECURSIVE t(id, parent, path) AS (
  SELECT $1::uuid, NULL::uuid, (ARRAY[]::text[]) COLLATE "C"
  UNION ALL
  SELECT l.item_id, t.id, t.path || ((l.rank COLLATE "C") || l.item_id::text)
  FROM t JOIN item_locations l ON l.parent_item_id=t.id AND l.deleted_root_id IS NULL
  JOIN items c ON c.id=l.item_id AND c.deleted_at IS NULL
  WHERE cardinality(t.path) < 30
)
SELECT t.id, t.parent FROM t JOIN items i ON i.id=t.id AND i.deleted_at IS NULL ORDER BY t.path`, sourceID)
	if err != nil {
		return nil, err
	}
	type edge struct{ id, parent uuid.UUID }
	var edges []edge
	for rows.Next() {
		var e edge
		var parent *uuid.UUID
		if err := rows.Scan(&e.id, &parent); err != nil {
			return nil, err
		}
		if parent != nil {
			e.parent = *parent
		}
		edges = append(edges, e)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	if len(edges) == 0 {
		return nil, ErrNotFound
	}
	ids := make([]string, len(edges))
	for i, e := range edges {
		ids[i] = e.id.String()
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		return nil, err
	}
	if !caps[ids[0]].Has(policy.View) {
		return nil, ErrNotFound
	}

	index := map[uuid.UUID]int{}
	var tree blueprintTree
	var base *time.Time
	type dates struct{ start, due *time.Time }
	var spans []dates
	for _, e := range edges {
		parent, ok := index[e.parent]
		if _, seen := index[e.id]; seen || !caps[e.id.String()].Has(policy.View) || (len(tree.Nodes) > 0 && !ok) {
			continue
		}
		if len(tree.Nodes) == maxBlueprintNodes {
			return nil, Invalid("a blueprint can hold at most 200 items")
		}
		if len(tree.Nodes) == 0 {
			parent = -1
		}
		n := blueprintNode{Parent: parent, AssigneeIDs: []uuid.UUID{}}
		var d dates
		var description json.RawMessage
		if err := tx.QueryRow(ctx, `
SELECT type::text, title, description, importance, status_id, start_date::timestamptz, due_date::timestamptz, duration_days, is_milestone, effort_minutes, fields,
  COALESCE((SELECT array_agg(user_id) FROM item_assignees a JOIN users u ON u.id=a.user_id AND u.status='active' WHERE a.item_id=i.id),'{}')
FROM items i WHERE id=$1`, e.id).Scan(&n.Type, &n.Title, &description, &n.Importance, &n.StatusID, &d.start, &d.due,
			&n.DurationDays, &n.IsMilestone, &n.EffortMinutes, &n.Fields, &n.AssigneeIDs); err != nil {
			return nil, err
		}
		if description != nil && string(description) != "null" {
			n.Description = description
		}
		for _, day := range []*time.Time{d.start, d.due} {
			if day != nil && (base == nil || day.Before(*base)) {
				base = day
			}
		}
		index[e.id] = len(tree.Nodes)
		tree.Nodes = append(tree.Nodes, n)
		spans = append(spans, d)
	}
	offset := func(day *time.Time) *int {
		if day == nil {
			return nil
		}
		n := int(day.Sub(*base).Hours() / 24)
		return &n
	}
	for i, d := range spans {
		tree.Nodes[i].StartOffset, tree.Nodes[i].DueOffset = offset(d.start), offset(d.due)
	}
	kept := make([]string, 0, len(tree.Nodes))
	for id := range index {
		kept = append(kept, id.String())
	}
	deps, err := tx.Query(ctx, `
SELECT predecessor_id, successor_id, type, lag FROM item_dependencies
WHERE predecessor_id=ANY($1::uuid[]) AND successor_id=ANY($1::uuid[]) ORDER BY id`, kept)
	if err != nil {
		return nil, err
	}
	for deps.Next() {
		var p, s uuid.UUID
		var d blueprintDependency
		if err := deps.Scan(&p, &s, &d.Type, &d.Lag); err != nil {
			return nil, err
		}
		d.Predecessor, d.Successor = index[p], index[s]
		tree.Dependencies = append(tree.Dependencies, d)
	}
	if deps.Err() != nil {
		return nil, deps.Err()
	}
	if tree.Dependencies == nil {
		tree.Dependencies = []blueprintDependency{}
	}
	var out json.RawMessage
	err = tx.QueryRow(ctx, `
WITH b AS (INSERT INTO blueprints(space_id,name,tree,created_by) VALUES($1,btrim($2),$3,$4::uuid) RETURNING *)
SELECT jsonb_build_object('id',id,'space_id',space_id,'name',name,'item_type',tree->'nodes'->0->>'type',
  'item_count',jsonb_array_length(tree->'nodes'),'created_at',created_at) FROM b`, spaceID, name, tree, actor.ID).Scan(&out)
	if pgCode(err) == "23505" {
		return nil, InvalidField{"name", "a blueprint with that name already exists in this Space"}
	}
	return out, err
}

// requireSpace: no access is 404, access without the capability is 403.
func requireSpace(ctx context.Context, tx pgx.Tx, actor policy.Actor, need policy.Capability, spaceID uuid.UUID) error {
	decision, _, err := policy.CanOnSpace(ctx, tx, actor, need, spaceID.String())
	switch {
	case err != nil:
		return err
	case decision == policy.NotFound:
		return ErrNotFound
	case decision == policy.Forbidden:
		return ErrForbidden
	}
	return nil
}

// ListBlueprints returns the blueprints of a Space that the actor may view.
func ListBlueprints(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID uuid.UUID) (json.RawMessage, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := requireSpace(ctx, tx, actor, policy.View, spaceID); err != nil {
		return nil, err
	}
	var out json.RawMessage
	err = tx.QueryRow(ctx, `
SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'space_id',space_id,'name',name,'item_type',tree->'nodes'->0->>'type',
  'item_count',jsonb_array_length(tree->'nodes'),'created_at',created_at) ORDER BY lower(name),id),'[]'::jsonb)
FROM blueprints WHERE space_id=$1`, spaceID).Scan(&out)
	return out, err
}

func DeleteBlueprint(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, id uuid.UUID) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var spaceID uuid.UUID
	err = tx.QueryRow(ctx, "SELECT space_id FROM blueprints WHERE id=$1", id).Scan(&spaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	if err == nil {
		err = requireSpace(ctx, tx, actor, policy.Edit, spaceID)
	}
	if err == nil {
		_, err = tx.Exec(ctx, "DELETE FROM blueprints WHERE id=$1", id)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	return writeError(err)
}

// BlueprintUse is what the caller chooses when creating items from a blueprint.
type BlueprintUse struct {
	Locations []Location `json:"locations"`
	Title     *string    `json:"title"`
	StartDate *string    `json:"start_date"` // anchors the relative dates; without it the new items have no dates
}

// UseBlueprint creates the whole tree in one transaction and returns the root item.
// shortcut: custom-field values that no longer fit the target Space are dropped, and dependent dates are not re-scheduled; revisit if blueprints carry rules.
func UseBlueprint(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, id uuid.UUID, use BlueprintUse, batch *notifications.Batch) (Detail, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback(ctx)
	item, err := useBlueprint(ctx, tx, actor, id, use, batch)
	if err == nil {
		err = tx.Commit(ctx)
	}
	return item, writeError(err)
}

func useBlueprint(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID, use BlueprintUse, batch *notifications.Batch) (Detail, error) {
	var spaceID uuid.UUID
	var tree blueprintTree
	err := tx.QueryRow(ctx, "SELECT space_id, tree FROM blueprints WHERE id=$1", id).Scan(&spaceID, &tree)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, ErrNotFound
	}
	if err != nil {
		return Detail{}, err
	}
	if err := requireSpace(ctx, tx, actor, policy.View, spaceID); err != nil {
		return Detail{}, err
	}
	if len(use.Locations) == 0 {
		return Detail{}, InvalidField{"locations", "at least one location is required"}
	}
	var anchor *time.Time
	if use.StartDate != nil {
		if anchor, err = dateField(marshalJSON(use.StartDate), "start_date"); err != nil {
			return Detail{}, err
		}
	}
	// Assignees who were deactivated since the blueprint was saved are skipped.
	active := map[uuid.UUID]bool{}
	rows, err := tx.Query(ctx, "SELECT id FROM users WHERE status='active'")
	if err != nil {
		return Detail{}, err
	}
	for rows.Next() {
		var u uuid.UUID
		if err := rows.Scan(&u); err != nil {
			return Detail{}, err
		}
		active[u] = true
	}
	if rows.Err() != nil {
		return Detail{}, rows.Err()
	}

	created := make([]string, len(tree.Nodes))
	for i, n := range tree.Nodes {
		locations := use.Locations
		if n.Parent >= 0 {
			parent := uuid.MustParse(created[n.Parent])
			locations = []Location{{SpaceID: use.Locations[0].SpaceID, ParentID: &parent}}
		}
		title := n.Title
		if i == 0 && use.Title != nil {
			title = *use.Title
		}
		fields := map[string]json.RawMessage{"type": marshalJSON(n.Type), "title": marshalJSON(title), "locations": marshalJSON(locations)}
		if n.Description != nil {
			fields["description"] = n.Description
		}
		fields["importance"] = marshalJSON(n.Importance)
		assignees := []uuid.UUID{}
		for _, u := range n.AssigneeIDs {
			if active[u] {
				assignees = append(assignees, u)
			}
		}
		fields["assignee_ids"] = marshalJSON(assignees)
		if n.DurationDays != nil {
			fields["duration_days"] = marshalJSON(n.DurationDays)
		}
		if n.IsMilestone {
			fields["is_milestone"] = marshalJSON(true)
		}
		if n.EffortMinutes != nil {
			fields["effort_minutes"] = marshalJSON(n.EffortMinutes)
		}
		if anchor != nil {
			for key, offset := range map[string]*int{"start_date": n.StartOffset, "due_date": n.DueOffset} {
				if offset != nil {
					fields[key] = marshalJSON(anchor.AddDate(0, 0, *offset).Format(time.DateOnly))
				}
			}
		}
		item, err := create(ctx, tx, actor, fields, batch)
		if err != nil {
			return Detail{}, err
		}
		var head struct{ ID string }
		if err := json.Unmarshal(item.Body, &head); err != nil {
			return Detail{}, err
		}
		created[i] = head.ID
		if err := restoreStatusAndFields(ctx, tx, actor, head.ID, n); err != nil {
			return Detail{}, err
		}
	}
	for _, d := range tree.Dependencies {
		if _, err := tx.Exec(ctx, "INSERT INTO item_dependencies(predecessor_id,successor_id,type,lag) VALUES($1,$2,$3,$4)",
			created[d.Predecessor], created[d.Successor], d.Type, d.Lag); err != nil {
			return Detail{}, err
		}
	}
	return readDetail(ctx, tx, actor, created[0])
}

// restoreStatusAndFields applies what create() does not accept: the saved status (only if the new item's
// workflow has it) and custom-field values (only those the target Space still accepts).
func restoreStatusAndFields(ctx context.Context, tx pgx.Tx, actor policy.Actor, id string, n blueprintNode) error {
	if n.StatusID != nil {
		var ok bool
		if err := tx.QueryRow(ctx, `
SELECT EXISTS(SELECT 1 FROM statuses s, items i JOIN statuses cur ON cur.id=i.status_id
 WHERE i.id=$1::uuid AND s.id=$2 AND s.workflow_id=cur.workflow_id)`, id, n.StatusID).Scan(&ok); err != nil {
			return err
		}
		if ok {
			if err := applyFields(ctx, tx, actor, id, n.Type, map[string]json.RawMessage{"status_id": marshalJSON(n.StatusID)}, []string{"status_id"}, false); err != nil {
				return err
			}
		}
	}
	for key, value := range n.Fields {
		updated, err := updateCustomFieldValues(ctx, tx, actor, id, marshalJSON(map[string]json.RawMessage{key: value}))
		if errors.As(err, new(InvalidField)) {
			continue
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "UPDATE items SET fields=$2::jsonb WHERE id=$1::uuid", id, updated); err != nil {
			return err
		}
	}
	return nil
}

func marshalJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
