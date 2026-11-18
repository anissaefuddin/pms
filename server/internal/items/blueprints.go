package items

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxBlueprintNodes = 200
	// maxBlueprintDepth counts levels below the root. Deeper items make the save fail; they are never cut off silently.
	maxBlueprintDepth = 30
)

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
	if err := requireSpace(ctx, tx, actor, policy.CreateBlueprint, spaceID); err != nil {
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
  WHERE cardinality(t.path) <= 30
)
SELECT t.id, t.parent, cardinality(t.path) FROM t JOIN items i ON i.id=t.id AND i.deleted_at IS NULL ORDER BY t.path`, sourceID)
	if err != nil {
		return nil, err
	}
	type edge struct {
		id, parent uuid.UUID
		depth      int
	}
	var edges []edge
	for rows.Next() {
		var e edge
		var parent *uuid.UUID
		if err := rows.Scan(&e.id, &parent, &e.depth); err != nil {
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
	restricted, err := restrictedFieldIDs(ctx, tx)
	if err != nil {
		return nil, err
	}
	for _, e := range edges {
		parent, ok := index[e.parent]
		if _, seen := index[e.id]; seen || !caps[e.id.String()].Has(policy.View) || (len(tree.Nodes) > 0 && !ok) {
			continue
		}
		if len(tree.Nodes) == maxBlueprintNodes {
			return nil, Invalid("a blueprint can hold at most 200 items")
		}
		if e.depth > maxBlueprintDepth {
			return nil, Invalid("a blueprint can hold at most 30 levels below its root item")
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
		for key := range n.Fields { // restricted values never enter a blueprint, so no reader can restore them
			if restricted[key] {
				delete(n.Fields, key)
			}
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
	// A blueprint lives in one Space: its content may only come from items that Space already holds.
	var inSpace int
	if err := tx.QueryRow(ctx, `SELECT count(DISTINCT item_id) FROM item_locations
WHERE space_id=$1 AND deleted_root_id IS NULL AND item_id=ANY($2::uuid[])`, spaceID, kept).Scan(&inSpace); err != nil {
		return nil, err
	}
	if inSpace != len(kept) {
		return nil, Invalid("every item of a blueprint must belong to the Space it is saved in")
	}
	// An item shared on its own may reach people who cannot see the Space, so its content must not be copied.
	var shared int
	if err := tx.QueryRow(ctx, `SELECT count(DISTINCT target_id) FROM shares WHERE target_type='item' AND target_id=ANY($1::uuid[])`, kept).Scan(&shared); err != nil {
		return nil, err
	}
	if shared > 0 {
		return nil, Invalid("items with their own shares cannot be saved as a blueprint; remove those shares first")
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
WITH b AS (INSERT INTO blueprints(space_id,name,tree,created_by,scope) VALUES($1,btrim($2),$3,$4::uuid,
  (SELECT CASE WHEN kind='personal' THEN 'account' ELSE 'space' END FROM spaces WHERE id=$1)) RETURNING *)
SELECT jsonb_build_object('id',id,'space_id',space_id,'name',name,'item_type',tree->'nodes'->0->>'type',
  'item_count',jsonb_array_length(tree->'nodes'),'created_at',created_at,'scope',scope,'version',version,'updated_at',updated_at) FROM b`, spaceID, name, tree, actor.ID).Scan(&out)
	if pgCode(err) == "23505" {
		return nil, InvalidField{"name", "a blueprint with that name already exists in this Space"}
	}
	if err != nil {
		return nil, err
	}
	var head struct{ ID string }
	if err := json.Unmarshal(out, &head); err != nil {
		return nil, err
	}
	return out, auditBlueprint(ctx, tx, actor, "blueprint.save", head.ID, map[string]any{"space_id": spaceID.String(), "item_count": len(tree.Nodes)})
}

// auditBlueprint writes the trail inside the caller's transaction, so a change and its event succeed or fail together.
// The details carry counts and ids only, never item content.
func auditBlueprint(ctx context.Context, tx pgx.Tx, actor policy.Actor, action, blueprintID string, details map[string]any) error {
	return Audit(ctx, tx, actor, action, "blueprint", blueprintID, details)
}

// Audit writes one event in the caller's transaction. Details must hold ids and counts, never answers or item content.
func Audit(ctx context.Context, tx pgx.Tx, actor policy.Actor, action, targetType, targetID string, details map[string]any) error {
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_events(actor_id,actor_name,action,target_type,target_id,details)
VALUES(NULLIF($1,'')::uuid, COALESCE((SELECT email::text FROM users WHERE id=NULLIF($1,'')::uuid),''), $2, $3, $4, $5)`,
		actor.ID, action, targetType, targetID, raw)
	return err
}

// restrictedFieldIDs lists the custom fields that carry grants. Their values are never copied into or restored from a blueprint.
func restrictedFieldIDs(ctx context.Context, tx pgx.Tx) (map[string]bool, error) {
	rows, err := tx.Query(ctx, "SELECT DISTINCT field_id::text FROM custom_field_grants")
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
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
  'item_count',jsonb_array_length(tree->'nodes'),'created_at',created_at,'scope',scope,'version',version,'updated_at',updated_at) ORDER BY lower(name),id),'[]'::jsonb)
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
		err = requireSpace(ctx, tx, actor, policy.ManageBlueprint, spaceID)
	}
	if err == nil {
		_, err = tx.Exec(ctx, "DELETE FROM blueprints WHERE id=$1", id)
	}
	if err == nil {
		err = auditBlueprint(ctx, tx, actor, "blueprint.delete", id.String(), map[string]any{"space_id": spaceID.String()})
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
	// FinishDate anchors the latest date of the tree instead (the last item finishes on it). Give one of the two, not both.
	FinishDate *string `json:"finish_date"`
	// Overrides replace the blueprint's defaults on the root item (a form's answers win over the blueprint).
	Overrides map[string]json.RawMessage `json:"-"`
}

// UseBlueprintTx is UseBlueprint inside the caller's transaction.
func UseBlueprintTx(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID, use BlueprintUse, batch *notifications.Batch) (Detail, error) {
	return useBlueprint(ctx, tx, actor, id, use, batch)
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
	if err := requireSpace(ctx, tx, actor, policy.UseBlueprint, spaceID); err != nil {
		return Detail{}, err
	}
	if len(use.Locations) == 0 {
		return Detail{}, InvalidField{"locations", "at least one location is required"}
	}
	if err := validateBlueprintTree(tree); err != nil { // refused before anything is written
		return Detail{}, err
	}
	anchor, err := blueprintAnchor(tree, use.StartDate, use.FinishDate)
	if err != nil {
		return Detail{}, err
	}
	restricted, err := restrictedFieldIDs(ctx, tx)
	if err != nil {
		return Detail{}, err
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
		start, due := scheduled(n, anchor)
		if start != nil {
			fields["start_date"] = marshalJSON(*start)
		}
		if due != nil {
			fields["due_date"] = marshalJSON(*due)
		}
		if i == 0 {
			for k, v := range use.Overrides {
				fields[k] = v
			}
			if _, ok := use.Overrides["status_id"]; ok {
				n.StatusID = nil
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
		if err := restoreStatusAndFields(ctx, tx, actor, head.ID, n, restricted); err != nil {
			return Detail{}, err
		}
	}
	for _, d := range tree.Dependencies {
		if _, err := tx.Exec(ctx, "INSERT INTO item_dependencies(predecessor_id,successor_id,type,lag) VALUES($1,$2,$3,$4)",
			created[d.Predecessor], created[d.Successor], d.Type, d.Lag); err != nil {
			return Detail{}, err
		}
	}
	if err := auditBlueprint(ctx, tx, actor, "blueprint.use", id.String(), map[string]any{"space_id": spaceID.String(), "item_count": len(created)}); err != nil {
		return Detail{}, err
	}
	return readDetail(ctx, tx, actor, created[0])
}

// validateBlueprintTree refuses a stored tree that could not have been saved: bad parent order, bad dependency
// indexes, too many items, or too many levels. Checked before any item is created.
func validateBlueprintTree(tree blueprintTree) error {
	n := len(tree.Nodes)
	if n == 0 || n > maxBlueprintNodes {
		return Invalid("a blueprint must hold 1 to 200 items")
	}
	depth := make([]int, n)
	for i, node := range tree.Nodes {
		if (i == 0 && node.Parent != -1) || (i > 0 && (node.Parent < 0 || node.Parent >= i)) {
			return Invalid("the blueprint hierarchy is invalid")
		}
		if i > 0 {
			depth[i] = depth[node.Parent] + 1
		}
		if depth[i] > maxBlueprintDepth {
			return Invalid("a blueprint can hold at most 30 levels below its root item")
		}
	}
	for _, d := range tree.Dependencies {
		if d.Predecessor < 0 || d.Predecessor >= n || d.Successor < 0 || d.Successor >= n || d.Predecessor == d.Successor {
			return Invalid("the blueprint dependencies are invalid")
		}
	}
	return nil
}

// restoreStatusAndFields applies what create() does not accept: the saved status (only if the new item's
// workflow has it) and custom-field values (only those the target Space still accepts).
func restoreStatusAndFields(ctx context.Context, tx pgx.Tx, actor policy.Actor, id string, n blueprintNode, restricted map[string]bool) error {
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
		if restricted[key] { // trees saved before this rule may still hold a restricted value
			continue
		}
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

// blueprintAnchor turns the caller's start_date or finish_date into the day the tree's earliest offset falls on.
// Nil means undated. Shared by use and preview, so a preview shows exactly what use will create.
func blueprintAnchor(tree blueprintTree, startDate, finishDate *string) (*time.Time, error) {
	if startDate != nil && finishDate != nil {
		return nil, Invalid("give start_date or finish_date, not both")
	}
	if startDate != nil {
		return dateField(marshalJSON(startDate), "start_date")
	}
	if finishDate == nil {
		return nil, nil
	}
	finish, err := dateField(marshalJSON(finishDate), "finish_date")
	if err != nil || finish == nil {
		return nil, err
	}
	var last *int // the latest offset in the tree, measured from its earliest date
	for _, n := range tree.Nodes {
		for _, o := range []*int{n.StartOffset, n.DueOffset} {
			if o != nil && (last == nil || *o > *last) {
				last = o
			}
		}
	}
	if last == nil {
		return nil, nil
	}
	start := finish.AddDate(0, 0, -*last)
	return &start, nil
}

// scheduled returns the start and due day of a node under an anchor (nil when the node or the anchor has no date).
func scheduled(n blueprintNode, anchor *time.Time) (start, due *string) {
	if anchor == nil {
		return nil, nil
	}
	format := func(offset *int) *string {
		if offset == nil {
			return nil
		}
		day := anchor.AddDate(0, 0, *offset).Format(time.DateOnly)
		return &day
	}
	return format(n.StartOffset), format(n.DueOffset)
}

// BlueprintEdit changes a blueprint's name and/or tree. Version is the version the caller read; a different one is a conflict.
type BlueprintEdit struct {
	Name    *string         `json:"name"`
	Tree    json.RawMessage `json:"tree"`
	Version *int            `json:"version"`
}

// EditBlueprint applies an edit in one transaction. The caller needs manage_blueprint in the blueprint's Space.
// The tree is validated like a saved one, and restricted fields are dropped, so an edit cannot store what a save could not.
func EditBlueprint(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, id uuid.UUID, in BlueprintEdit) (json.RawMessage, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	out, err := editBlueprint(ctx, tx, actor, id, in)
	if err == nil {
		err = tx.Commit(ctx)
	}
	return out, writeError(err)
}

func editBlueprint(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID, in BlueprintEdit) (json.RawMessage, error) {
	var spaceID uuid.UUID
	var version int
	err := tx.QueryRow(ctx, "SELECT space_id, version FROM blueprints WHERE id=$1 FOR UPDATE", id).Scan(&spaceID, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := requireSpace(ctx, tx, actor, policy.ManageBlueprint, spaceID); err != nil {
		return nil, err
	}
	if in.Version == nil {
		return nil, InvalidField{"version", "send the version you read"}
	}
	if *in.Version != version {
		return nil, ErrConflict
	}
	if in.Name == nil && in.Tree == nil {
		return nil, Invalid("send a name or a tree to change")
	}
	var name *string
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if n := len([]rune(trimmed)); n < 1 || n > 120 {
			return nil, InvalidField{"name", "name must be 1 to 120 characters"}
		}
		name = &trimmed
	}
	var tree *blueprintTree
	if in.Tree != nil {
		var parsed blueprintTree
		if err := json.Unmarshal(in.Tree, &parsed); err != nil {
			return nil, Invalid("tree must be an object with nodes and dependencies")
		}
		if err := validateBlueprintTree(parsed); err != nil {
			return nil, err
		}
		if err := validateEditedNodes(parsed); err != nil {
			return nil, err
		}
		if err := validateEditedGraph(parsed); err != nil {
			return nil, err
		}
		for i, n := range parsed.Nodes {
			if n.StatusID == nil {
				continue
			}
			var known bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM statuses WHERE id=$1)", *n.StatusID).Scan(&known); err != nil {
				return nil, err
			}
			if !known {
				return nil, InvalidField{"tree", fmt.Sprintf("item %d has an unknown status", i+1)}
			}
		}
		restricted, err := restrictedFieldIDs(ctx, tx)
		if err != nil {
			return nil, err
		}
		for i := range parsed.Nodes {
			for key := range parsed.Nodes[i].Fields {
				if restricted[key] {
					delete(parsed.Nodes[i].Fields, key)
				}
			}
		}
		tree = &parsed
	}
	var raw json.RawMessage
	if tree != nil {
		if raw, err = json.Marshal(tree); err != nil {
			return nil, err
		}
	}
	var out json.RawMessage
	err = tx.QueryRow(ctx, `
UPDATE blueprints SET name=COALESCE($2,name), tree=COALESCE($3::jsonb,tree), version=version+1, updated_at=now() WHERE id=$1
RETURNING jsonb_build_object('id',id,'space_id',space_id,'name',name,'item_type',tree->'nodes'->0->>'type',
  'item_count',jsonb_array_length(tree->'nodes'),'created_at',created_at,'scope',scope,'version',version,'updated_at',updated_at)`,
		id, name, raw).Scan(&out)
	if pgCode(err) == "23505" {
		return nil, InvalidField{"name", "a blueprint with that name already exists in this Space"}
	}
	if err != nil {
		return nil, err
	}
	changed := []string{}
	if name != nil {
		changed = append(changed, "name")
	}
	if tree != nil {
		changed = append(changed, "tree")
	}
	if err := Audit(ctx, tx, actor, "blueprint.edit", "blueprint", id.String(), map[string]any{"space_id": spaceID.String(), "changed": changed}); err != nil {
		return nil, err
	}
	return out, nil
}

// validateEditedNodes checks what a saved tree already satisfies by construction: known types, importance, dates, and dependency kinds.
func validateEditedNodes(tree blueprintTree) error {
	for _, n := range tree.Nodes {
		if !slices.Contains([]string{"task", "project", "folder"}, n.Type) {
			return Invalid("a blueprint item must be a task, project, or folder")
		}
		if title := strings.TrimSpace(n.Title); title == "" || len([]rune(title)) > 500 {
			return InvalidField{"title", "every blueprint item needs a title of 1 to 500 characters"}
		}
		if !slices.Contains([]string{"high", "normal", "low"}, n.Importance) {
			return Invalid("importance must be high, normal, or low")
		}
		if n.StartOffset != nil && n.DueOffset != nil && *n.DueOffset < *n.StartOffset {
			return Invalid("an item cannot finish before it starts")
		}
		if (n.DurationDays != nil && *n.DurationDays < 0) || (n.EffortMinutes != nil && *n.EffortMinutes < 0) {
			return Invalid("duration and effort cannot be negative")
		}
	}
	for _, d := range tree.Dependencies {
		if !validDependencyType(d.Type) {
			return Invalid("a dependency type must be FS, SS, FF, or SF")
		}
	}
	return nil
}

// PreviewNode is one item of a preview: what use would create, with its dates, and nothing is written.
type PreviewNode struct {
	Title     string  `json:"title"`
	Type      string  `json:"type"`
	Parent    int     `json:"parent"`
	StartDate *string `json:"start_date"`
	DueDate   *string `json:"due_date"`
}

// PreviewBlueprint computes what UseBlueprint would create, with the same anchor and date rules. It needs use_blueprint.
func PreviewBlueprint(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, id uuid.UUID, use BlueprintUse) ([]PreviewNode, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var spaceID uuid.UUID
	var tree blueprintTree
	err = tx.QueryRow(ctx, "SELECT space_id, tree FROM blueprints WHERE id=$1", id).Scan(&spaceID, &tree)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, writeError(err)
	}
	if err := requireSpace(ctx, tx, actor, policy.UseBlueprint, spaceID); err != nil {
		return nil, writeError(err)
	}
	if err := validateBlueprintTree(tree); err != nil {
		return nil, writeError(err)
	}
	anchor, err := blueprintAnchor(tree, use.StartDate, use.FinishDate)
	if err != nil {
		return nil, writeError(err)
	}
	out := make([]PreviewNode, len(tree.Nodes))
	for i, n := range tree.Nodes {
		title := n.Title
		if i == 0 && use.Title != nil {
			title = *use.Title
		}
		start, due := scheduled(n, anchor)
		out[i] = PreviewNode{Title: title, Type: n.Type, Parent: n.Parent, StartDate: start, DueDate: due}
	}
	return out, nil
}

// GetBlueprint returns one blueprint with its tree. It needs view on the blueprint's Space. Restricted fields are removed
// on the way out too, so a tree saved before the rule existed does not expose them.
func GetBlueprint(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, id uuid.UUID) (json.RawMessage, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var spaceID uuid.UUID
	var tree blueprintTree
	var out json.RawMessage
	err = tx.QueryRow(ctx, `SELECT space_id, tree, jsonb_build_object('id',id,'space_id',space_id,'name',name,'item_type',tree->'nodes'->0->>'type',
  'item_count',jsonb_array_length(tree->'nodes'),'created_at',created_at,'scope',scope,'version',version,'updated_at',updated_at)
FROM blueprints WHERE id=$1`, id).Scan(&spaceID, &tree, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, writeError(err)
	}
	if err := requireSpace(ctx, tx, actor, policy.View, spaceID); err != nil {
		return nil, writeError(err)
	}
	restricted, err := restrictedFieldIDs(ctx, tx)
	if err != nil {
		return nil, writeError(err)
	}
	for i := range tree.Nodes {
		for key := range tree.Nodes[i].Fields {
			if restricted[key] {
				delete(tree.Nodes[i].Fields, key)
			}
		}
	}
	treeJSON, err := json.Marshal(tree)
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, err
	}
	doc["tree"] = treeJSON
	return json.Marshal(doc)
}

// validateEditedGraph refuses a dependency twice and any loop of dependencies: the schedule cannot follow a cycle.
func validateEditedGraph(tree blueprintTree) error {
	seen := map[[2]int]bool{}
	adj := make([][]int, len(tree.Nodes))
	indeg := make([]int, len(tree.Nodes))
	for _, d := range tree.Dependencies {
		key := [2]int{d.Predecessor, d.Successor}
		if seen[key] {
			return Invalid("the same dependency appears twice")
		}
		seen[key] = true
		adj[d.Predecessor] = append(adj[d.Predecessor], d.Successor)
		indeg[d.Successor]++
	}
	queue := []int{}
	for i, n := range indeg {
		if n == 0 {
			queue = append(queue, i)
		}
	}
	placed := 0
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		placed++
		for _, w := range adj[v] {
			if indeg[w]--; indeg[w] == 0 {
				queue = append(queue, w)
			}
		}
	}
	if placed != len(tree.Nodes) {
		return Invalid("dependencies would form a loop")
	}
	return nil
}
