package items

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ListFilter struct {
	Types, StatusGroups    []string
	Assignee, Sort, Cursor string
	Fields                 []string // "<custom field id>:<value>", ANDed
	Limit                  int
}

type Page struct {
	Items      []json.RawMessage `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}

type listCursor struct {
	Scope [32]byte
	ID    string
	Key   *string
}

var sortKeys = map[string]string{
	"manual":     "min(l.rank COLLATE \"C\")",
	"title":      "lower(i.title)",
	"status":     "lpad(st.position::text, 12, '0')",
	"due_date":   "i.due_date::text",
	"start_date": "i.start_date::text",
	// High first, then the earliest finish date; Items without one sort last within their level.
	"priority":     "(CASE i.importance WHEN 'high' THEN '0' WHEN 'normal' THEN '1' ELSE '2' END)||'|'||COALESCE(i.due_date::text,'9999-99-99')",
	"predecessors": "lpad((SELECT count(*) FROM item_dependencies d WHERE d.successor_id=i.id)::text, 8, '0')",
	"created_at":   "to_char(i.created_at AT TIME ZONE 'UTC','YYYY-MM-DD HH24:MI:SS.US')",
	"updated_at":   "to_char(i.updated_at AT TIME ZONE 'UTC','YYYY-MM-DD HH24:MI:SS.US')",
}

func init() { sortKeys["importance"] = sortKeys["priority"] }

func List(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, container string, children bool, filter ListFilter) (Page, error) {
	id, err := uuid.Parse(container)
	if err != nil {
		return Page{}, ErrNotFound
	}
	container = id.String()
	key, cursor, scope, err := validateFilter(actor, container, children, &filter)
	if err != nil {
		return Page{}, err
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback(ctx)
	if err := checkTypeKeys(ctx, tx, filter.Types); err != nil {
		return Page{}, err
	}
	spaceIDs, parentID, err := visibleContainer(ctx, tx, actor, container, children)
	if err != nil {
		return Page{}, err
	}
	fieldKey, fieldTerms, fieldArgs, err := resolveFields(ctx, tx, spaceIDs, filter)
	if err != nil {
		return Page{}, err
	}
	if fieldKey != "" {
		key = fieldKey
	}
	rows, err := tx.Query(ctx, `
WITH candidates AS (
 SELECT i.id, `+key+` AS key
 FROM item_locations l JOIN items i ON i.id=l.item_id
 LEFT JOIN statuses st ON st.id=i.status_id
 WHERE l.space_id=ANY(ARRAY(SELECT unnest($1::text[])::uuid))
   AND l.parent_item_id IS NOT DISTINCT FROM $2::uuid
   AND l.deleted_root_id IS NULL AND i.deleted_at IS NULL
   AND (COALESCE(cardinality($3::text[]),0)=0 OR i.type_id IN (SELECT id FROM item_type_defs WHERE key=ANY($3)))
   AND (COALESCE(cardinality($4::text[]),0)=0 OR st.grp::text=ANY($4))
   AND ($5::text='' OR CASE WHEN $5='none' THEN NOT EXISTS (
     SELECT 1 FROM item_assignees a WHERE a.item_id=i.id
   ) ELSE EXISTS (
     SELECT 1 FROM item_assignees a WHERE a.item_id=i.id AND a.user_id::text=$5
   ) END)`+fieldTerms+`
 GROUP BY i.id,st.id
)
SELECT id::text,key FROM candidates
WHERE $6::uuid IS NULL OR
  (key IS NULL,COALESCE(key,'') COLLATE "C",id) >
  ($7::text IS NULL,COALESCE($7,'') COLLATE "C",$6::uuid)
ORDER BY key COLLATE "C" NULLS LAST,id LIMIT $8`,
		append([]any{spaceIDs, parentID, filter.Types, filter.StatusGroups, filter.Assignee, cursorID(cursor), cursorKey(cursor), filter.Limit + 1}, fieldArgs...)...)
	if err != nil {
		return Page{}, err
	}
	candidates, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (listCursor, error) {
		p := listCursor{Scope: scope}
		err := row.Scan(&p.ID, &p.Key)
		return p, err
	})
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: []json.RawMessage{}}
	if len(candidates) > filter.Limit {
		candidates = candidates[:filter.Limit]
		data, err := json.Marshal(candidates[len(candidates)-1])
		if err != nil {
			return Page{}, err
		}
		encoded := base64.RawURLEncoding.EncodeToString(data)
		page.NextCursor = &encoded
	}
	ids := make([]string, 0, len(candidates))
	for _, row := range candidates {
		ids = append(ids, row.ID)
	}
	details, err := readDetails(ctx, tx, actor, ids)
	if err != nil {
		return Page{}, err
	}
	for _, item := range details {
		page.Items = append(page.Items, item.Body)
	}
	return page, tx.Commit(ctx)
}

// checkTypeKeys refuses a type filter that names an unknown item type key (built-in or custom).
func checkTypeKeys(ctx context.Context, tx pgx.Tx, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	var known int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM item_type_defs WHERE key=ANY($1::text[])", keys).Scan(&known); err != nil {
		return err
	}
	distinct := map[string]bool{}
	for _, key := range keys {
		distinct[key] = true
	}
	if known != len(distinct) {
		return Invalid("unknown item type")
	}
	return nil
}

func visibleContainer(ctx context.Context, tx pgx.Tx, actor policy.Actor, id string, children bool) ([]string, *string, error) {
	if !children {
		space, _, err := policy.SpaceAccessFor(ctx, tx, actor, id)
		if err != nil {
			return nil, nil, err
		}
		if !space.Caps.Has(policy.View) {
			return nil, nil, ErrNotFound
		}
		return []string{id}, nil, nil
	}
	rows, err := tx.Query(ctx, "SELECT DISTINCT space_id::text FROM item_locations WHERE item_id=$1::uuid AND deleted_root_id IS NULL", id)
	if err != nil {
		return nil, nil, err
	}
	spaces, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, nil, err
	}
	visible := spaces[:0]
	for _, space := range spaces {
		caps, err := policy.ItemCapsInSpace(ctx, tx, actor, id, space)
		if err != nil {
			return nil, nil, err
		}
		if caps.Has(policy.View) {
			visible = append(visible, space)
		}
	}
	if len(visible) == 0 {
		return nil, nil, ErrNotFound
	}
	return visible, &id, nil
}

func validateFilter(actor policy.Actor, id string, children bool, filter *ListFilter) (string, *listCursor, [32]byte, error) {
	var scope [32]byte
	if filter.Limit < 1 || filter.Limit > 200 {
		return "", nil, scope, Invalid("limit must be between 1 and 200")
	}
	if filter.Sort == "" {
		filter.Sort = "manual"
	}
	key, ok := sortKeys[filter.Sort]
	if !ok && strings.HasPrefix(filter.Sort, "field:") {
		key, ok = "NULL::text", true // replaced by resolveFields
	}
	if !ok {
		return "", nil, scope, Invalid("unknown sort")
	}
	if len(filter.Types) > 20 {
		return "", nil, scope, Invalid("at most 20 item types")
	}
	for _, group := range filter.StatusGroups {
		if group != "active" && group != "completed" && group != "deferred" && group != "cancelled" {
			return "", nil, scope, Invalid("unknown status group")
		}
	}
	switch filter.Assignee {
	case "me":
		filter.Assignee = actor.ID
	case "none", "":
	default:
		user, err := uuid.Parse(filter.Assignee)
		if err != nil {
			return "", nil, scope, Invalid("assignee must be a UUID, me or none")
		}
		filter.Assignee = user.String()
	}
	raw := filter.Cursor
	filter.Cursor = ""
	data, _ := json.Marshal(filter)
	namespace := "root"
	if children {
		namespace = "children"
	}
	scope = sha256.Sum256([]byte(strings.Join([]string{actor.ID, namespace, id, string(data)}, ":")))
	if raw == "" {
		return key, nil, scope, nil
	}
	if len(raw) > 8192 {
		return "", nil, scope, Invalid("invalid cursor")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	var c listCursor
	if err != nil || json.Unmarshal(decoded, &c) != nil || c.Scope != scope {
		return "", nil, scope, Invalid("invalid cursor")
	}
	if _, err := uuid.Parse(c.ID); err != nil {
		return "", nil, scope, Invalid("invalid cursor")
	}
	return key, &c, scope, nil
}

func cursorID(c *listCursor) *string {
	if c == nil {
		return nil
	}
	return &c.ID
}

func cursorKey(c *listCursor) *string {
	if c == nil {
		return nil
	}
	return c.Key
}

// ListShared lists directly granted Items that are absent from every visible Space.
func ListShared(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, filter ListFilter) (Page, error) {
	_, cursor, scope, err := validateFilter(actor, "shared", false, &filter)
	if err != nil {
		return Page{}, err
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback(ctx)
	spaces, err := policy.VisibleSpaces(ctx, tx, actor, "")
	if err != nil {
		return Page{}, err
	}
	visible := make([]string, 0, len(spaces))
	for _, space := range spaces {
		visible = append(visible, space.ID)
	}
	rows, err := tx.Query(ctx, `
SELECT i.id::text FROM items i
WHERE i.deleted_at IS NULL AND ($3::uuid IS NULL OR i.id>$3::uuid)
AND EXISTS (SELECT 1 FROM shares sh WHERE sh.target_type='item' AND sh.target_id=i.id
 AND sh.principal_type='user' AND sh.principal_id=$1::uuid)
AND EXISTS (SELECT 1 FROM item_locations l JOIN spaces s ON s.id=l.space_id
 WHERE l.item_id=i.id AND l.deleted_root_id IS NULL AND s.deleted_at IS NULL AND s.kind='shared')
AND NOT EXISTS (SELECT 1 FROM item_locations l WHERE l.item_id=i.id AND l.deleted_root_id IS NULL
 AND l.space_id=ANY(ARRAY(SELECT unnest($2::text[])::uuid)))
ORDER BY i.id LIMIT $4`, actor.ID, visible, cursorID(cursor), filter.Limit+1)
	if err != nil {
		return Page{}, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: []json.RawMessage{}}
	if len(ids) > filter.Limit {
		ids = ids[:filter.Limit]
		data, err := json.Marshal(listCursor{Scope: scope, ID: ids[len(ids)-1]})
		if err != nil {
			return Page{}, err
		}
		value := base64.RawURLEncoding.EncodeToString(data)
		page.NextCursor = &value
	}
	details, err := readDetails(ctx, tx, actor, ids)
	if err != nil {
		return Page{}, err
	}
	for _, item := range details {
		page.Items = append(page.Items, item.Body)
	}
	return page, tx.Commit(ctx)
}
