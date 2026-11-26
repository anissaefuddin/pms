package items

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRestoreSpaceDeleted = errors.New("an original Space has been deleted")

type RecycleFilter struct {
	Type, Sort, Cursor string
	Limit              int
}

type deletedItem struct {
	ID, Key, Title, Type string
	DeletedAt            time.Time
	Body                 json.RawMessage
}

func Recycle(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, filter RecycleFilter) (Page, error) {
	key := "(-extract(epoch FROM i.deleted_at))::numeric"
	switch filter.Sort {
	case "", "deleted_at":
	case "title":
		key = `lower(i.title) COLLATE "C"`
	default:
		return Page{}, Invalid("unknown Recycle bin sort")
	}
	types := []string{}
	if filter.Type != "" {
		if filter.Type != "project" && filter.Type != "folder" && filter.Type != "task" {
			return Page{}, Invalid("unknown item type")
		}
		types = append(types, filter.Type)
	}
	_, cursor, scope, err := validateFilter(actor, "recycle:"+filter.Sort, false,
		&ListFilter{Types: types, Limit: filter.Limit, Cursor: filter.Cursor})
	if err != nil {
		return Page{}, err
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback(ctx)
	spaces, err := policy.RecycleSpaces(ctx, tx, actor)
	if err != nil {
		return Page{}, err
	}
	spaceNames := map[string]string{}
	for _, space := range spaces {
		if space.Caps.Has(policy.View) {
			spaceNames[space.ID] = space.Name
		}
	}
	page := Page{Items: []json.RawMessage{}}
	var last listCursor
	for {
		// Keep scanning until there are enough authorized rows for the page and its cursor.
		batchSize := max(128, filter.Limit+1)
		compare := "$3::text COLLATE \"C\""
		if filter.Sort != "title" {
			compare = "$3::numeric"
		}
		rows, err := tx.Query(ctx, `
SELECT i.id::text,(`+key+`)::text,i.title,i.type::text,i.deleted_at,
jsonb_strip_nulls(jsonb_build_object('id',i.id,'seq',i.seq,'title',i.title,'type',i.type,'version',i.version,
 'created_at',i.created_at,'updated_at',i.updated_at,'deleted_at',i.deleted_at,'assignees','[]'::jsonb,
 'deleted_by',CASE WHEN u.id IS NOT NULL THEN jsonb_build_object('id',u.id,'name',concat_ws(' ',u.first_name,u.last_name)) END))
FROM items i LEFT JOIN users u ON u.id=i.deleted_by
WHERE i.deleted_at IS NOT NULL AND i.deleted_root_id=i.id AND ($1='' OR i.type::text=$1)
AND ($2::uuid IS NULL OR (`+key+`,i.id)>(`+compare+`,$2::uuid))
ORDER BY `+key+`,i.id LIMIT $4`, filter.Type, cursorID(cursor), cursorKey(cursor), batchSize)
		if err != nil {
			return Page{}, err
		}
		candidates, err := pgx.CollectRows(rows, pgx.RowToStructByPos[deletedItem])
		if err != nil {
			return Page{}, err
		}
		ids := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			ids = append(ids, candidate.ID)
		}
		caps, err := policy.RecycleItemsCaps(ctx, tx, actor, ids, true)
		if err != nil {
			return Page{}, err
		}
		for _, candidate := range candidates {
			if !caps[candidate.ID].Has(policy.View) {
				continue
			}
			if len(page.Items) == filter.Limit {
				raw, err := json.Marshal(last)
				if err != nil {
					return Page{}, err
				}
				next := base64.RawURLEncoding.EncodeToString(raw)
				page.NextCursor = &next
				return page, tx.Commit(ctx)
			}
			body, err := recycleBody(ctx, tx, candidate, actor, caps[candidate.ID], spaceNames)
			if err != nil {
				return Page{}, err
			}
			page.Items = append(page.Items, body)
			last = listCursor{Scope: scope, ID: candidate.ID, Key: &candidate.Key}
		}
		if len(candidates) < batchSize {
			break
		}
		tail := candidates[len(candidates)-1]
		cursor = &listCursor{Scope: scope, ID: tail.ID, Key: &tail.Key}
	}
	return page, tx.Commit(ctx)
}

func recycleBody(ctx context.Context, tx pgx.Tx, item deletedItem, actor policy.Actor, caps policy.Caps, spaceNames map[string]string) (json.RawMessage, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(item.Body, &body); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT space_id::text FROM item_locations WHERE item_id=$1::uuid AND deleted_root_id=$1::uuid ORDER BY 1`, item.ID)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	type location struct {
		SpaceID   string `json:"space_id"`
		SpaceName string `json:"space_name"`
	}
	locations := []location{}
	for _, id := range ids {
		if name, visible := spaceNames[id]; visible {
			locations = append(locations, location{id, name})
		}
	}
	body["locations"], err = json.Marshal(locations)
	if err != nil {
		return nil, err
	}
	body["my_capabilities"], err = json.Marshal(caps.List())
	if err != nil {
		return nil, err
	}
	if err := canRestore(ctx, tx, actor, item.ID, caps); err != nil {
		switch {
		case errors.Is(err, ErrRestoreSpaceDeleted):
			body["restore_blocked_reason"] = json.RawMessage(`"space_deleted"`)
		case errors.Is(err, ErrForbidden):
			body["restore_blocked_reason"] = json.RawMessage(`"forbidden"`)
		default:
			return nil, err
		}
	}
	return json.Marshal(body)
}

// Parent edges marked by this operation will become live together; independently deleted
// or missing parents do not prevent recovery to the original Space root.
const missingRestoreParent = `l.parent_item_id IS NOT NULL AND NOT EXISTS (
 SELECT 1 FROM item_locations p JOIN items pi ON pi.id=p.item_id
 WHERE p.item_id=l.parent_item_id AND p.space_id=l.space_id
 AND ((pi.deleted_at IS NULL AND p.deleted_root_id IS NULL)
 OR (p.deleted_root_id=$1::uuid AND (pi.deleted_at IS NULL OR pi.deleted_root_id=$1::uuid)))
)`

func canRestore(ctx context.Context, tx pgx.Tx, actor policy.Actor, id string, caps policy.Caps) error {
	if !caps.Has(policy.Delete) {
		return ErrForbidden
	}
	var deadSpace bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM item_locations l JOIN spaces s ON s.id=l.space_id
 WHERE l.deleted_root_id=$1::uuid AND s.deleted_at IS NOT NULL)`, id).Scan(&deadSpace); err != nil {
		return err
	}
	if deadSpace {
		return ErrRestoreSpaceDeleted
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT l.space_id::text FROM item_locations l
 WHERE l.deleted_root_id=$1::uuid AND `+missingRestoreParent, id)
	if err != nil {
		return err
	}
	spaces, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, id := range spaces {
		access, _, err := policy.SpaceAccessFor(ctx, tx, actor, id)
		if err != nil {
			return err
		}
		if !access.Caps.Has(policy.Create) {
			return ErrForbidden
		}
	}
	return nil
}

func Restore(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, id string) (Detail, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return Detail{}, ErrNotFound
	}
	id = parsed.String()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback(ctx)
	var found string
	err = tx.QueryRow(ctx, `SELECT id::text FROM items WHERE id=$1 AND deleted_at IS NOT NULL AND deleted_root_id=id FOR UPDATE`, parsed).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, ErrNotFound
	}
	if err != nil {
		return Detail{}, writeError(err)
	}
	caps, err := policy.RecycleItemsCaps(ctx, tx, actor, []string{id}, true)
	if err != nil {
		return Detail{}, err
	}
	if !caps[id].Has(policy.View) {
		return Detail{}, ErrNotFound
	}
	if err := canRestore(ctx, tx, actor, id, caps[id]); err != nil {
		return Detail{}, err
	}

	// A place whose parent is gone moves to the Space root, which is a Move: DEC-10 applies to it too.
	if err := canRestoreAssignments(ctx, tx, actor, parsed); err != nil {
		return Detail{}, writeError(err)
	}

	// Coalesce multiple missing parents in a Space to one root placement, keeping a root that already exists.
	if _, err := tx.Exec(ctx, `WITH fallback AS (
 SELECT l.ctid AS tid,l.item_id,l.space_id,row_number() OVER (PARTITION BY l.item_id,l.space_id ORDER BY l.rank COLLATE "C",l.parent_item_id) AS rn
 FROM item_locations l WHERE l.deleted_root_id=$1::uuid AND `+missingRestoreParent+`
)
DELETE FROM item_locations l USING fallback f
WHERE l.ctid=f.tid AND (f.rn>1 OR EXISTS (
 SELECT 1 FROM item_locations root WHERE root.item_id=f.item_id AND root.space_id=f.space_id AND root.parent_item_id IS NULL
))`, id); err != nil {
		return Detail{}, writeError(err)
	}
	rows, err := tx.Query(ctx, `UPDATE item_locations l SET
 parent_item_id=CASE WHEN `+missingRestoreParent+` THEN NULL ELSE l.parent_item_id END,
 deleted_root_id=NULL WHERE l.deleted_root_id=$1::uuid RETURNING l.item_id::text`, id)
	if err != nil {
		return Detail{}, writeError(err)
	}
	placed, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return Detail{}, writeError(err)
	}
	rows, err = tx.Query(ctx, `UPDATE items SET deleted_at=NULL,deleted_by=NULL,deleted_root_id=NULL,version=version+1
 WHERE deleted_root_id=$1::uuid RETURNING id::text`, id)
	if err != nil {
		return Detail{}, writeError(err)
	}
	restored, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return Detail{}, writeError(err)
	}
	var changed []string
	changed = append(changed, restored...)
	for _, itemID := range placed {
		if !slices.Contains(changed, itemID) {
			changed = append(changed, itemID)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE items SET version=version+1
 WHERE id=ANY(ARRAY(SELECT unnest($1::text[])::uuid)) AND deleted_at IS NULL
 AND NOT (id=ANY(ARRAY(SELECT unnest($2::text[])::uuid)))`, changed, restored); err != nil {
		return Detail{}, writeError(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO activity(item_id,actor_id,action,changes)
 SELECT id,$2::uuid,'restore',jsonb_build_object('deleted_at',jsonb_build_object('to',NULL))
 FROM items WHERE id=ANY(ARRAY(SELECT unnest($1::text[])::uuid)) AND deleted_at IS NULL`, changed, actor.ID); err != nil {
		return Detail{}, writeError(err)
	}
	out, err := readDetail(ctx, tx, actor, id)
	if err != nil {
		return Detail{}, writeError(err)
	}
	return out, writeError(tx.Commit(ctx))
}

func canRestoreAssignments(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID) error {
	others, err := carriesOthers(ctx, tx, id, actor.ID)
	if err != nil || !others {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT l.space_id, l.parent_item_id FROM item_locations l WHERE l.deleted_root_id=$1::uuid AND `+missingRestoreParent, id.String())
	if err != nil {
		return err
	}
	moved, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Location])
	if err != nil {
		return err
	}
	for _, from := range moved {
		before, err := boundary(ctx, tx, from)
		if err != nil {
			return err
		}
		if err := mayCarryInto(ctx, tx, actor, Location{SpaceID: from.SpaceID}, map[string]bool{before: true}); err != nil {
			return err
		}
	}
	return nil
}
