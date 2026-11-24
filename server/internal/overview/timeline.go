package overview

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Timeline and agenda read models. Both work from due dates only, so the window is compared as plain dates: no UTC
// conversion is needed. "Today" comes from the actor's time zone. Access is the same as the overview: visible shared Spaces.

const (
	maxRangeRows   = 2000 // rows read per request; a larger range sets truncated
	timelineMaxDay = 400  // a calendar year plus room for a custom range
	agendaMaxDay   = 31
)

// TimelineOptions are validated by the handler. From is YYYY-MM-DD; empty means today.
type TimelineOptions struct {
	From            string
	Days            int
	SpaceID         string
	IncludeArchived bool
}

// TimelineNode is one item in a Space's real hierarchy: Space > Folder or Project > Task > Subtask. Items due in the
// range are marked InRange; their ancestors are included without a due date so the structure stays intact.
type TimelineNode struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Type      string         `json:"type"`
	StartDate *string        `json:"start_date"`
	DueDate   *string        `json:"due_date"`
	Status    string         `json:"status_group"`
	InRange   bool           `json:"in_range"`
	Children  []TimelineNode `json:"children"`
}

// TimelineSpace groups one Space's tree. Nodes are the Space's top-level items (those without a parent in the Space).
type TimelineSpace struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Icon   string         `json:"icon_key,omitempty"`
	Access string         `json:"effective_access"`
	Nodes  []TimelineNode `json:"nodes"`
}

// TimelineResult is the response body. From and To are inclusive dates.
type TimelineResult struct {
	Timezone  string          `json:"timezone"`
	From      string          `json:"from"`
	To        string          `json:"to"`
	Spaces    []TimelineSpace `json:"spaces"`
	Truncated bool            `json:"truncated"`
}

// rangeRow is one item location in the window.
type rangeRow struct {
	spaceID, itemID, parentID, typ, title, due, status string
}

// chainRow is one item location in a Space that is either due in the range or an ancestor of one.
type chainRow struct {
	spaceID, itemID, parentID, typ, title, start, due, status string
}

// BuildTimeline groups the actor's dated Projects and Tasks by Space, following each item's real parents.
func BuildTimeline(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, opts TimelineOptions) (TimelineResult, error) {
	tz := userTimezone(ctx, pool, actor.ID)
	byID, ids, err := scope(ctx, pool, actor, opts.SpaceID, opts.IncludeArchived)
	if err != nil {
		return TimelineResult{}, err
	}
	from, err := startDate(ctx, pool, tz, opts.From)
	if err != nil {
		return TimelineResult{}, err
	}
	res := TimelineResult{Timezone: tz, From: from, To: addDays(from, opts.Days-1), Spaces: []TimelineSpace{}}
	if len(ids) == 0 {
		return res, nil
	}
	rows, truncated, err := readTimelineRows(ctx, pool, ids, from, opts.Days)
	if err != nil {
		return TimelineResult{}, err
	}
	res.Truncated = truncated
	inRange := map[string]bool{}
	var due []string
	for _, r := range rows {
		inRange[r[0]+"|"+r[1]] = true
		due = append(due, r[1])
	}
	chain, err := readChain(ctx, pool, ids, due)
	if err != nil {
		return TimelineResult{}, err
	}

	// One node per (Space, item). A node's parent is its parent_item_id inside the same Space; a missing parent
	// means the node is top level in that Space.
	type built struct {
		node   TimelineNode
		parent string
		kids   []string
	}
	nodes := map[string]*built{}
	for _, c := range chain {
		key := c.spaceID + "|" + c.itemID
		b := &built{node: TimelineNode{ID: c.itemID, Title: c.title, Type: c.typ, Status: c.status, InRange: inRange[key], Children: []TimelineNode{}}}
		if c.start != "" {
			st := c.start
			b.node.StartDate = &st
		}
		if c.due != "" {
			d := c.due
			b.node.DueDate = &d
		}
		if c.parentID != "" {
			b.parent = c.spaceID + "|" + c.parentID
		}
		nodes[key] = b
	}
	roots := map[string][]string{} // space -> top-level keys
	for key, b := range nodes {
		if _, ok := nodes[b.parent]; ok && b.parent != "" {
			nodes[b.parent].kids = append(nodes[b.parent].kids, key)
			continue
		}
		space := key[:strings.IndexByte(key, '|')]
		roots[space] = append(roots[space], key)
	}

	// Build the nested values from the leaves up, guarding against a cycle in bad data.
	var build func(key string, seen map[string]bool) TimelineNode
	build = func(key string, seen map[string]bool) TimelineNode {
		b := nodes[key]
		n := b.node
		seen[key] = true
		kids := append([]string(nil), b.kids...)
		sort.SliceStable(kids, func(i, j int) bool { return lessNode(nodes[kids[i]].node, nodes[kids[j]].node) })
		for _, k := range kids {
			if !seen[k] {
				n.Children = append(n.Children, build(k, seen))
			}
		}
		return n
	}
	for _, id := range ids {
		a := byID[id]
		space := TimelineSpace{ID: id, Name: a.Name, Icon: a.IconKey, Access: accessLabel(a), Nodes: []TimelineNode{}}
		top := roots[id]
		sort.SliceStable(top, func(i, j int) bool { return lessNode(nodes[top[i]].node, nodes[top[j]].node) })
		for _, key := range top {
			space.Nodes = append(space.Nodes, build(key, map[string]bool{}))
		}
		res.Spaces = append(res.Spaces, space)
	}
	sort.SliceStable(res.Spaces, func(i, j int) bool { return res.Spaces[i].Name < res.Spaces[j].Name })
	return res, nil
}

// lessNode orders siblings by due date (undated last), then title, then id.
func lessNode(a, b TimelineNode) bool {
	switch {
	case a.DueDate == nil && b.DueDate != nil:
		return false
	case a.DueDate != nil && b.DueDate == nil:
		return true
	case a.DueDate != nil && b.DueDate != nil && *a.DueDate != *b.DueDate:
		return *a.DueDate < *b.DueDate
	case a.Title != b.Title:
		return a.Title < b.Title
	}
	return a.ID < b.ID
}

// readChain loads the due items and every ancestor they have inside the same visible Spaces.
func readChain(ctx context.Context, pool *pgxpool.Pool, ids, due []string) ([]chainRow, error) {
	if len(due) == 0 {
		return nil, nil
	}
	rs, err := pool.Query(ctx, `
WITH RECURSIVE chain AS (
  SELECT l.space_id, l.item_id, l.parent_item_id
  FROM item_locations l
  WHERE l.deleted_root_id IS NULL AND l.space_id = ANY($1::uuid[]) AND l.item_id = ANY($2::uuid[])
  UNION
  SELECT p.space_id, p.item_id, p.parent_item_id
  FROM chain c
  JOIN item_locations p ON p.space_id = c.space_id AND p.item_id = c.parent_item_id AND p.deleted_root_id IS NULL
)
SELECT c.space_id::text, c.item_id::text, COALESCE(c.parent_item_id::text, ''), i.type::text, i.title,
       COALESCE(i.start_date::text, ''), COALESCE(i.due_date::text, ''), COALESCE(s.grp::text, '')
FROM chain c
JOIN items i ON i.id = c.item_id AND i.deleted_at IS NULL
LEFT JOIN statuses s ON s.id = i.status_id`, ids, due)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []chainRow
	for rs.Next() {
		var r chainRow
		if err := rs.Scan(&r.spaceID, &r.itemID, &r.parentID, &r.typ, &r.title, &r.start, &r.due, &r.status); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

// readTimelineRows lists [space, item] pairs whose span (start or due, to due) overlaps the window. A dated item
// without a start is a single day on its due date. Rows past maxRangeRows set truncated.
func readTimelineRows(ctx context.Context, pool *pgxpool.Pool, ids []string, from string, days int) ([][2]string, bool, error) {
	rs, err := pool.Query(ctx, `
SELECT l.space_id::text, i.id::text
FROM item_locations l
JOIN items i ON i.id = l.item_id AND i.deleted_at IS NULL
LEFT JOIN statuses s ON s.id = i.status_id
WHERE l.deleted_root_id IS NULL AND l.space_id = ANY($1::uuid[])
  AND i.type IN ('project', 'task') AND i.due_date IS NOT NULL
  AND i.due_date >= $2::date
  AND COALESCE(i.start_date, i.due_date) < ($2::date + $3::int)
  AND COALESCE(s.grp::text, 'active') NOT IN ('completed', 'cancelled')
ORDER BY i.due_date, i.id
LIMIT $4`, ids, from, days, maxRangeRows+1)
	if err != nil {
		return nil, false, err
	}
	defer rs.Close()
	var out [][2]string
	for rs.Next() {
		var r [2]string
		if err := rs.Scan(&r[0], &r[1]); err != nil {
			return nil, false, err
		}
		out = append(out, r)
	}
	if err := rs.Err(); err != nil {
		return nil, false, err
	}
	if len(out) > maxRangeRows {
		return out[:maxRangeRows], true, nil
	}
	return out, false, nil
}

// AgendaOptions are validated by the handler. Kind is "" (any), "project" or "task".
type AgendaOptions struct {
	From            string
	Days            int
	SpaceID         string
	Kind            string
	Limit           int
	IncludeArchived bool
}

// AgendaItem is one due Project or Task. A Task that is past due is flagged Overdue.
type AgendaItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	DueDate   string `json:"due_date"`
	Status    string `json:"status_group"`
	SpaceID   string `json:"space_id"`
	SpaceName string `json:"space_name"`
	Overdue   bool   `json:"overdue"`
}

// AgendaResult is the response body. OverdueCount counts all past-due active Tasks, whatever the range.
type AgendaResult struct {
	Timezone     string       `json:"timezone"`
	Today        string       `json:"today"`
	From         string       `json:"from"`
	To           string       `json:"to"`
	Items        []AgendaItem `json:"items"`
	OverdueCount int          `json:"overdue_count"`
	Truncated    bool         `json:"truncated"`
}

// BuildAgenda lists due Projects and Tasks in the range, ordered by date. Completed and cancelled items are left out.
// An item in two visible Spaces appears once per Space, because each location is its own row.
func BuildAgenda(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, opts AgendaOptions) (AgendaResult, error) {
	tz := userTimezone(ctx, pool, actor.ID)
	byID, ids, err := scope(ctx, pool, actor, opts.SpaceID, opts.IncludeArchived)
	if err != nil {
		return AgendaResult{}, err
	}
	today, err := startDate(ctx, pool, tz, "")
	if err != nil {
		return AgendaResult{}, err
	}
	from, err := startDate(ctx, pool, tz, opts.From)
	if err != nil {
		return AgendaResult{}, err
	}
	res := AgendaResult{Timezone: tz, Today: today, From: from, To: addDays(from, opts.Days-1), Items: []AgendaItem{}}
	if len(ids) == 0 {
		return res, nil
	}
	if err := pool.QueryRow(ctx, `
SELECT count(DISTINCT i.id)::int FROM item_locations l
JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
JOIN statuses s ON s.id=i.status_id
WHERE l.deleted_root_id IS NULL AND l.space_id = ANY($1::uuid[]) AND i.type='task' AND s.grp='active' AND i.due_date < $2::date`,
		ids, today).Scan(&res.OverdueCount); err != nil {
		return AgendaResult{}, err
	}
	rows, truncated, err := readRange(ctx, pool, ids, from, opts.Days, opts.Kind)
	if err != nil {
		return AgendaResult{}, err
	}
	res.Truncated = truncated
	if opts.Limit > 0 && len(rows) > opts.Limit {
		rows, res.Truncated = rows[:opts.Limit], true
	}
	for _, r := range rows {
		res.Items = append(res.Items, AgendaItem{ID: r.itemID, Title: r.title, Type: r.typ, DueDate: r.due, Status: r.status,
			SpaceID: r.spaceID, SpaceName: byID[r.spaceID].Name, Overdue: r.typ == "task" && r.due < today})
	}
	return res, nil
}

// readRange loads Projects and Tasks due in [from, from+days) that are not completed or cancelled, ordered by date.
// kind narrows to one type when set. Rows past maxRangeRows are dropped and truncated is returned true.
func readRange(ctx context.Context, pool *pgxpool.Pool, ids []string, from string, days int, kind string) ([]rangeRow, bool, error) {
	rs, err := pool.Query(ctx, `
SELECT l.space_id::text, i.id::text, COALESCE(l.parent_item_id::text, ''), i.type::text, i.title, i.due_date::text, COALESCE(s.grp::text, '')
FROM item_locations l
JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
LEFT JOIN statuses s ON s.id=i.status_id
WHERE l.deleted_root_id IS NULL AND l.space_id = ANY($1::uuid[])
  AND i.type IN ('project','task') AND i.due_date IS NOT NULL
  AND i.due_date >= $2::date AND i.due_date < ($2::date + $3::int)
  AND COALESCE(s.grp::text, 'active') NOT IN ('completed','cancelled')
  AND ($4 = '' OR i.type::text = $4)
ORDER BY i.due_date, i.title, i.id
LIMIT $5`, ids, from, days, kind, maxRangeRows+1)
	if err != nil {
		return nil, false, err
	}
	defer rs.Close()
	var out []rangeRow
	for rs.Next() {
		var r rangeRow
		if err := rs.Scan(&r.spaceID, &r.itemID, &r.parentID, &r.typ, &r.title, &r.due, &r.status); err != nil {
			return nil, false, err
		}
		out = append(out, r)
	}
	if err := rs.Err(); err != nil {
		return nil, false, err
	}
	if len(out) > maxRangeRows {
		return out[:maxRangeRows], true, nil
	}
	return out, false, nil
}

// startDate returns the given YYYY-MM-DD, or today in the actor's zone when empty. The handler checks the format.
func startDate(ctx context.Context, pool *pgxpool.Pool, tz, value string) (string, error) {
	if value != "" {
		return value, nil
	}
	var today string
	err := pool.QueryRow(ctx, "SELECT (now() AT TIME ZONE $1)::date::text", tz).Scan(&today)
	return today, err
}

func addDays(date string, n int) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, n).Format("2006-01-02")
}

// Limits exposed to the handler.
const (
	TimelineMaxDays = timelineMaxDay
	AgendaMaxDays   = agendaMaxDay
)
