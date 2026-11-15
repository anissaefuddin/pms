package items

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SearchResults struct {
	Items    []json.RawMessage `json:"items"`
	Comments []json.RawMessage `json:"comments"`
	Users    []json.RawMessage `json:"users"`
}

// SearchOptions are the extra filters of the full search page (NAV-07); the zero value changes nothing.
type SearchOptions struct {
	AssignedToMe bool
	StatusGroup  string // "", "active", "completed", "deferred" or "cancelled"
	TypeKey      string // "" or the key of an item type (S8.1.5)
	Sort         string // "" or "relevance" ranks by match, "date_modified" by recency
	Offset       int    // visible hits to skip in each group, for paging (RT-37)
}

func Search(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, query, spaceID, containerID string, limit int, opts SearchOptions) (SearchResults, error) {
	out := SearchResults{Items: []json.RawMessage{}, Comments: []json.RawMessage{}, Users: []json.RawMessage{}}
	query = strings.TrimSpace(query)
	if !utf8.ValidString(query) || strings.ContainsRune(query, 0) || utf8.RuneCountInString(query) < 1 || utf8.RuneCountInString(query) > 200 || limit < 1 || limit > 50 {
		return out, Invalid("query must contain 1 to 200 characters and limit must be between 1 and 50")
	}
	order := "m.rank DESC, m.id"
	switch opts.Sort {
	case "", "relevance":
	case "date_modified":
		order = "m.ts DESC, m.id"
	default:
		return out, Invalid("unknown search sort")
	}
	if opts.Offset < 0 || opts.Offset > 10000 {
		return out, Invalid("offset must be between 0 and 10000")
	}
	switch opts.StatusGroup {
	case "", "active", "completed", "deferred", "cancelled":
	default:
		return out, Invalid("unknown status group")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)

	spaces, container, err := searchScope(ctx, tx, actor, spaceID, containerID)
	if err != nil {
		return out, err
	}
	var prefix *string
	if err := tx.QueryRow(ctx, `SELECT string_agg(quote_literal(word) || ':*', ' & ')
FROM unnest(tsvector_to_array(to_tsvector('simple', $1))) AS word`, query).Scan(&prefix); err != nil {
		return out, err
	}
	if prefix == nil {
		return out, nil
	}

	// The scope check sits inside each branch, so ranking only touches rows the user may see.
	rows, err := tx.Query(ctx, fmt.Sprintf(`
WITH RECURSIVE subtree(item_id, space_id) AS (
  SELECT l.item_id, l.space_id FROM item_locations l
  WHERE l.item_id=$3::uuid AND l.space_id=ANY(ARRAY(SELECT unnest($2::text[])::uuid)) AND l.deleted_root_id IS NULL
  UNION
  SELECT l.item_id, l.space_id FROM subtree p
  JOIN item_locations l ON l.parent_item_id=p.item_id AND l.space_id=p.space_id
  JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
  WHERE l.deleted_root_id IS NULL
), matched AS (
  SELECT i.id AS id, i.id AS item_id, i.type::text AS kind, ts_rank(i.search, $1::tsquery) AS rank, i.updated_at AS ts
  FROM items i WHERE i.deleted_at IS NULL AND i.search @@ $1::tsquery AND %[1]s AND %[3]s
  UNION ALL
  SELECT c.id, c.item_id, 'comment', ts_rank(c.search, $1::tsquery), c.created_at
  FROM comments c JOIN items i ON i.id=c.item_id AND i.deleted_at IS NULL
  WHERE c.deleted_at IS NULL AND c.search @@ $1::tsquery AND %[2]s AND %[3]s
)
SELECT m.id::text, m.item_id::text, m.kind FROM matched m
ORDER BY %[4]s`, scope("i.id"), scope("c.item_id"), `($4::boolean IS FALSE OR EXISTS (SELECT 1 FROM item_assignees a WHERE a.item_id=i.id AND a.user_id=$6::uuid))
  AND ($5::text='' OR EXISTS (SELECT 1 FROM statuses st WHERE st.id=i.status_id AND st.grp::text=$5))
  AND ($7::text='' OR i.type_id IN (SELECT id FROM item_type_defs WHERE key=$7))`, order),
		*prefix, spaces, container, opts.AssignedToMe, opts.StatusGroup, actor.ID, opts.TypeKey)
	if err != nil {
		return out, err
	}
	type hit struct{ ID, ItemID, Kind string }
	hits, err := pgx.CollectRows(rows, pgx.RowToStructByPos[hit])
	if err != nil {
		return out, err
	}
	ids, commentIDs := []string{}, []string{}
	tasks, containers := 0, 0 // accepted per group
	seen := map[string]int{}  // visible hits per group, to skip the first Offset of them
	groupOf := func(kind string) string {
		if kind == "task" || kind == "comment" {
			return kind
		}
		return "container"
	}
	room := func(group string) bool {
		switch group {
		case "task":
			return tasks < limit
		case "comment":
			return len(commentIDs) < limit
		}
		return containers < limit
	}
	for start := 0; start < len(hits) && (room("task") || room("container") || room("comment")); start += 128 {
		batch := hits[start:min(start+128, len(hits))]
		check := make([]string, 0, len(batch))
		for _, hit := range batch {
			if room(groupOf(hit.Kind)) {
				check = append(check, hit.ItemID)
			}
		}
		if len(check) == 0 {
			continue
		}
		caps, err := policy.ItemsCaps(ctx, tx, actor, check, nil)
		if err != nil {
			return out, err
		}
		for _, hit := range batch {
			group := groupOf(hit.Kind)
			if !room(group) || !caps[hit.ItemID].Has(policy.View) {
				continue
			}
			seen[group]++
			if seen[group] <= opts.Offset {
				continue
			}
			switch group {
			case "task":
				tasks++
				ids = append(ids, hit.ID)
			case "comment":
				commentIDs = append(commentIDs, hit.ID)
			default:
				containers++
				ids = append(ids, hit.ID)
			}
		}
	}

	details, err := readDetails(ctx, tx, actor, ids)
	if err != nil {
		return out, err
	}
	paths, err := ancestorPaths(ctx, tx, actor, spaces, ids)
	if err != nil {
		return out, err
	}
	for _, detail := range details {
		body := detail.Body
		var m map[string]json.RawMessage
		if json.Unmarshal(body, &m) == nil {
			var id string
			_ = json.Unmarshal(m["id"], &id)
			if path := paths[id]; len(path) != 0 {
				m["path"], _ = json.Marshal(path)
				body, _ = json.Marshal(m)
			}
		}
		out.Items = append(out.Items, body)
	}
	if len(commentIDs) != 0 {
		rows, err = tx.Query(ctx, `
SELECT jsonb_build_object('comment_id', c.id, 'item_id', i.id, 'item_title', i.title, 'snippet',
  replace(replace(ts_headline('simple', c.body_text, $2::tsquery,
   'StartSel=⟨,StopSel=⟩,MaxFragments=1,MaxWords=25,MinWords=10'), '⟨', ''), '⟩', ''))
FROM unnest($1::text[]) WITH ORDINALITY AS selected(id, position)
JOIN comments c ON c.id=selected.id::uuid JOIN items i ON i.id=c.item_id ORDER BY selected.position`, commentIDs, *prefix)
		if err != nil {
			return out, err
		}
		out.Comments, err = pgx.CollectRows(rows, pgx.RowTo[json.RawMessage])
		if err != nil {
			return out, err
		}
	}
	if policy.CanBrowseDirectory(actor.Type) {
		out.Users, err = searchUsers(ctx, tx, *prefix, spaces, container, limit)
		if err != nil {
			return out, err
		}
	}
	return out, tx.Commit(ctx)
}

// searchScope keeps project traversal on live placements where the actor can see the project.
func searchScope(ctx context.Context, tx pgx.Tx, actor policy.Actor, spaceID, containerID string) ([]string, *string, error) {
	if spaceID != "" {
		id, err := uuid.Parse(spaceID)
		if err != nil {
			return nil, nil, ErrNotFound
		}
		spaceID = id.String()
		space, _, err := policy.SpaceAccessFor(ctx, tx, actor, spaceID)
		if err != nil {
			return nil, nil, err
		}
		if !space.Caps.Has(policy.View) {
			return nil, nil, ErrNotFound
		}
	}
	if containerID == "" {
		if spaceID != "" {
			return []string{spaceID}, nil, nil
		}
		return nil, nil, nil
	}
	id, err := uuid.Parse(containerID)
	if err != nil {
		return nil, nil, ErrNotFound
	}
	containerID = id.String()
	rows, err := tx.Query(ctx, `SELECT DISTINCT l.space_id::text
FROM item_locations l JOIN items i ON i.id=l.item_id
WHERE l.item_id=$1 AND l.deleted_root_id IS NULL AND i.deleted_at IS NULL AND i.type IN ('project','folder')
  AND ($2::text='' OR l.space_id::text=$2)`, id, spaceID)
	if err != nil {
		return nil, nil, err
	}
	placements, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, nil, err
	}
	visible := []string{}
	for _, space := range placements {
		caps, err := policy.ItemCapsInSpace(ctx, tx, actor, containerID, space)
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
	return visible, &containerID, nil
}

func searchUsers(ctx context.Context, tx pgx.Tx, prefix string, spaces []string, container *string, limit int) ([]json.RawMessage, error) {
	rows, err := tx.Query(ctx, `SELECT id::text, user_type::text, concat_ws(' ', first_name, last_name)
FROM users WHERE status='active' AND search @@ $1::tsquery
ORDER BY ts_rank(search, $1::tsquery) DESC, lower(first_name), lower(last_name), id`, prefix)
	if err != nil {
		return nil, err
	}
	type match struct{ ID, Type, Name string }
	matches, err := pgx.CollectRows(rows, pgx.RowToStructByPos[match])
	if err != nil {
		return nil, err
	}
	out := []json.RawMessage{}
	for _, user := range matches {
		visible := spaces == nil
		candidate := policy.Actor{ID: user.ID, Type: policy.UserType(user.Type)}
		for _, space := range spaces {
			if container == nil {
				access, _, err := policy.SpaceAccessFor(ctx, tx, candidate, space)
				if err != nil {
					return nil, err
				}
				visible = access.Caps.Has(policy.View)
			} else {
				caps, err := policy.ItemCapsInSpace(ctx, tx, candidate, *container, space)
				if err != nil {
					return nil, err
				}
				visible = caps.Has(policy.View)
			}
			if visible {
				break
			}
		}
		if visible {
			encoded, err := json.Marshal(map[string]string{"id": user.ID, "name": user.Name})
			if err != nil {
				return nil, err
			}
			out = append(out, encoded)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

// scope is the visibility predicate for the item id in column: anywhere in the user's Spaces, or inside the chosen container.
func scope(column string) string {
	return `($2::text[] IS NULL
  OR ($3::uuid IS NULL AND EXISTS (
    SELECT 1 FROM item_locations l
    WHERE l.item_id=` + column + ` AND l.space_id=ANY(ARRAY(SELECT unnest($2::text[])::uuid)) AND l.deleted_root_id IS NULL
  ))
  OR ($3::uuid IS NOT NULL AND EXISTS (SELECT 1 FROM subtree p WHERE p.item_id=` + column + `)))`
}

type PathStep struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// ancestorPaths gives, for each hit, the chain of ancestors from the outermost down to its parent, taken from the
// first live placement (by Space, then parent) inside the search scope. The chain stops above the first ancestor the
// actor cannot view, so a hidden parent is neither named nor counted. shortcut: only the first placement of a multi-location item.
func ancestorPaths(ctx context.Context, tx pgx.Tx, actor policy.Actor, spaces, ids []string) (map[string][]PathStep, error) {
	out := map[string][]PathStep{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := tx.Query(ctx, `
WITH RECURSIVE first_place AS (
  SELECT DISTINCT ON (l.item_id) l.item_id, l.space_id, l.parent_item_id
  FROM item_locations l
  WHERE l.item_id=ANY(ARRAY(SELECT unnest($1::text[])::uuid)) AND l.deleted_root_id IS NULL
    AND (COALESCE(cardinality($2::text[]),0)=0 OR l.space_id=ANY(ARRAY(SELECT unnest($2::text[])::uuid)))
  ORDER BY l.item_id, l.space_id, l.parent_item_id NULLS FIRST
), up(item_id, space_id, anc, depth) AS (
  SELECT item_id, space_id, parent_item_id, 1 FROM first_place WHERE parent_item_id IS NOT NULL
  UNION ALL
  SELECT up.item_id, up.space_id, l.parent_item_id, up.depth+1
  FROM up JOIN LATERAL (SELECT l.parent_item_id FROM item_locations l
    WHERE l.item_id=up.anc AND l.space_id=up.space_id AND l.deleted_root_id IS NULL
    ORDER BY l.parent_item_id NULLS FIRST LIMIT 1) l ON true
  WHERE up.depth < 20 AND l.parent_item_id IS NOT NULL
)
SELECT up.item_id::text, up.space_id::text, a.id::text, a.title, a.type::text, up.depth
FROM up JOIN items a ON a.id=up.anc AND a.deleted_at IS NULL ORDER BY up.item_id, up.depth`, ids, spaces)
	if err != nil {
		return nil, err
	}
	type step struct {
		Item, Space, ID, Title, Type string
		Depth                        int
	}
	steps, err := pgx.CollectRows(rows, pgx.RowToStructByPos[step])
	if err != nil {
		return nil, err
	}
	all := make([]string, 0, len(steps))
	for _, st := range steps {
		all = append(all, st.ID)
	}
	if len(all) == 0 {
		return out, nil
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, all, nil)
	if err != nil {
		return nil, err
	}
	blocked := map[string]bool{}
	nearest := map[string][]PathStep{} // nearest ancestor first, until the first hidden one
	for _, st := range steps {
		if blocked[st.Item] {
			continue
		}
		if !caps[st.ID].Has(policy.View) {
			blocked[st.Item] = true
			continue
		}
		nearest[st.Item] = append(nearest[st.Item], PathStep{ID: st.ID, Title: st.Title, Type: st.Type})
	}
	for id, chain := range nearest {
		slices.Reverse(chain)
		out[id] = chain
	}
	return out, nil
}
