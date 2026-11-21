package overview

import (
	"context"
	"sort"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Timeline and agenda read models. Both work from due dates only, so the window is compared as plain dates: no UTC
// conversion is needed. "Today" comes from the actor's time zone. Access is the same as the overview: visible shared Spaces.

const (
	maxRangeRows   = 2000 // rows read per request; a larger range sets truncated
	timelineMaxDay = 90
	agendaMaxDay   = 31
)

// TimelineOptions are validated by the handler. From is YYYY-MM-DD; empty means today.
type TimelineOptions struct {
	From            string
	Days            int
	SpaceID         string
	IncludeArchived bool
}

// TimelineItem is a Task with a due date in the range.
type TimelineItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	DueDate string `json:"due_date"`
	Status  string `json:"status_group"`
}

// TimelineProject is a Project with a due date in the range. Its Tasks are the direct children in the same Space that
// are also due in the range. A Project's due date is shown as its milestone.
type TimelineProject struct {
	TimelineItem
	Tasks []TimelineItem `json:"tasks"`
}

// TimelineSpace groups one Space's rows. UnassignedTasks are due in the range but have no Project in the range.
type TimelineSpace struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Icon            string            `json:"icon_key,omitempty"`
	Access          string            `json:"effective_access"`
	Projects        []TimelineProject `json:"projects"`
	UnassignedTasks []TimelineItem    `json:"unassigned_tasks"`
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

// BuildTimeline groups the actor's dated Projects and Tasks by Space.
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
	rows, truncated, err := readRange(ctx, pool, ids, from, opts.Days, "")
	if err != nil {
		return TimelineResult{}, err
	}
	res.Truncated = truncated

	// Pass 1: Projects, so that Tasks can attach to them in pass 2.
	type key struct{ space, item string }
	spaceIdx := map[string]int{}
	projIdx := map[key][2]int{} // space+item -> [space index, project index]
	for _, r := range rows {
		if r.typ != "project" {
			continue
		}
		if _, seen := projIdx[key{r.spaceID, r.itemID}]; seen {
			continue
		}
		si := spaceFor(&res, spaceIdx, r.spaceID, byID)
		res.Spaces[si].Projects = append(res.Spaces[si].Projects, TimelineProject{TimelineItem: item(r)})
		projIdx[key{r.spaceID, r.itemID}] = [2]int{si, len(res.Spaces[si].Projects) - 1}
	}
	// Pass 2: Tasks go under their Project in the same Space, otherwise into the Space's unassigned list.
	for _, r := range rows {
		if r.typ != "task" {
			continue
		}
		si := spaceFor(&res, spaceIdx, r.spaceID, byID)
		if p, ok := projIdx[key{r.spaceID, r.parentID}]; ok && r.parentID != "" {
			res.Spaces[p[0]].Projects[p[1]].Tasks = append(res.Spaces[p[0]].Projects[p[1]].Tasks, item(r))
			continue
		}
		res.Spaces[si].UnassignedTasks = append(res.Spaces[si].UnassignedTasks, item(r))
	}
	sort.SliceStable(res.Spaces, func(i, j int) bool { return res.Spaces[i].Name < res.Spaces[j].Name })
	return res, nil
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

func item(r rangeRow) TimelineItem {
	return TimelineItem{ID: r.itemID, Title: r.title, DueDate: r.due, Status: r.status}
}

// spaceFor returns the index of a Space in res, adding it on first use.
func spaceFor(res *TimelineResult, idx map[string]int, id string, byID map[string]policy.SpaceAccess) int {
	if i, ok := idx[id]; ok {
		return i
	}
	a := byID[id]
	res.Spaces = append(res.Spaces, TimelineSpace{ID: id, Name: a.Name, Icon: a.IconKey, Access: accessLabel(a),
		Projects: []TimelineProject{}, UnassignedTasks: []TimelineItem{}})
	idx[id] = len(res.Spaces) - 1
	return idx[id]
}

// Limits exposed to the handler.
const (
	TimelineMaxDays = timelineMaxDay
	AgendaMaxDays   = agendaMaxDay
)
