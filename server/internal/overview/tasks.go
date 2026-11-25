package overview

import (
	"context"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TasksOptions narrow the cross-Space Task list. Every filter is optional and they combine with AND. Empty slices mean
// "any". Spaces the actor cannot see are dropped silently. Preset is "" (none), "overdue" or "completed_week" and uses
// the same rules as the Summary figures, so a figure and the list behind it always agree.
type TasksOptions struct {
	Preset          string
	Spaces          []string
	Assignees       []string
	Statuses        []string // status groups: active, deferred, completed, cancelled
	DueFrom, DueTo  string   // YYYY-MM-DD, inclusive; empty means open-ended
	Query           string   // title contains, case-insensitive
	Limit           int
	IncludeArchived bool
}

// TaskRow is one Task in one Space. A Task in two visible Spaces appears once.
type TaskRow struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	DueDate     *string    `json:"due_date"`
	StartDate   *string    `json:"start_date"`
	Status      string     `json:"status_group"`
	StatusName  string     `json:"status_name"`
	CompletedAt *string    `json:"completed_at"`
	SpaceID     string     `json:"space_id"`
	SpaceName   string     `json:"space_name"`
	Overdue     bool       `json:"overdue"`
	Assignees   []Assignee `json:"assignees"`
}

// Assignee is a person assigned to a Task.
type Assignee struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TasksResult is the response body. Today is the actor's date, used for the overdue flag.
type TasksResult struct {
	Timezone  string    `json:"timezone"`
	Today     string    `json:"today"`
	Items     []TaskRow `json:"items"`
	Truncated bool      `json:"truncated"`
}

// BuildTasks lists Tasks across the actor's visible Spaces, filtered as requested, ordered by due date.
func BuildTasks(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, opts TasksOptions) (TasksResult, error) {
	tz := userTimezone(ctx, pool, actor.ID)
	byID, ids, err := scope(ctx, pool, actor, "", opts.IncludeArchived)
	if err != nil {
		return TasksResult{}, err
	}
	today, err := startDate(ctx, pool, tz, "")
	if err != nil {
		return TasksResult{}, err
	}
	res := TasksResult{Timezone: tz, Today: today, Items: []TaskRow{}}
	ids = filterIDs(ids, opts.Spaces)
	if len(ids) == 0 {
		return res, nil
	}
	limit := opts.Limit
	if limit <= 0 || limit > maxRangeRows {
		limit = 200
	}
	rs, err := pool.Query(ctx, `
WITH t AS (SELECT (now() AT TIME ZONE $2)::date AS today, date_trunc('week', now() AT TIME ZONE $2) AT TIME ZONE $2 AS week_start)
SELECT x.id, x.title, x.due_date, x.start_date, x.grp, x.status_name, x.completed_at, x.space_id, x.overdue
FROM (
  SELECT DISTINCT ON (i.id) i.id::text AS id, i.title, i.due_date::text AS due_date, i.start_date::text AS start_date,
         COALESCE(s.grp::text,'') AS grp, COALESCE(s.name,'') AS status_name, i.completed_at::text AS completed_at,
         l.space_id::text AS space_id, COALESCE(i.due_date < t.today, false) AS overdue, i.due_date AS due, i.title AS sort_title
  FROM t, items i
  JOIN item_locations l ON l.item_id=i.id AND l.deleted_root_id IS NULL AND l.space_id = ANY($1::uuid[])
  LEFT JOIN statuses s ON s.id=i.status_id
  WHERE i.deleted_at IS NULL AND i.type='task'
    AND ($3 = '' OR ($3='overdue' AND COALESCE(s.grp::text,'')='active' AND i.due_date < t.today)
                 OR ($3='completed_week' AND COALESCE(s.grp::text,'')='completed' AND i.completed_at >= t.week_start AND i.completed_at <= now()))
    AND (COALESCE(cardinality($4::text[]), 0) = 0 OR COALESCE(s.grp::text,'') = ANY($4::text[]))
    AND (COALESCE(cardinality($5::uuid[]), 0) = 0 OR EXISTS (SELECT 1 FROM item_assignees a WHERE a.item_id=i.id AND a.user_id = ANY($5::uuid[])))
    AND (NULLIF($6,'') IS NULL OR i.due_date >= NULLIF($6,'')::date)
    AND (NULLIF($7,'') IS NULL OR i.due_date <= NULLIF($7,'')::date)
    AND ($8 = '' OR i.title ILIKE '%' || $8 || '%')
  ORDER BY i.id, l.space_id
) x
ORDER BY x.due ASC NULLS LAST, x.sort_title, x.id
LIMIT $9`, ids, tz, opts.Preset, opts.Statuses, opts.Assignees, opts.DueFrom, opts.DueTo, opts.Query, limit+1)
	if err != nil {
		return TasksResult{}, err
	}
	defer rs.Close()
	for rs.Next() {
		var r TaskRow
		if err := rs.Scan(&r.ID, &r.Title, &r.DueDate, &r.StartDate, &r.Status, &r.StatusName, &r.CompletedAt, &r.SpaceID, &r.Overdue); err != nil {
			return TasksResult{}, err
		}
		r.SpaceName = byID[r.SpaceID].Name
		r.Assignees = []Assignee{}
		res.Items = append(res.Items, r)
	}
	if err := rs.Err(); err != nil {
		return TasksResult{}, err
	}
	if len(res.Items) > limit {
		res.Items, res.Truncated = res.Items[:limit], true
	}
	if err := loadAssignees(ctx, pool, res.Items); err != nil {
		return TasksResult{}, err
	}
	return res, nil
}

// filterIDs keeps the visible Space ids that were asked for. An empty request keeps them all.
func filterIDs(visible []string, wanted []string) []string {
	if len(wanted) == 0 {
		return visible
	}
	want := map[string]bool{}
	for _, id := range wanted {
		want[id] = true
	}
	var out []string
	for _, id := range visible {
		if want[id] {
			out = append(out, id)
		}
	}
	return out
}

// loadAssignees fills in each Task's assignees in one query.
func loadAssignees(ctx context.Context, pool *pgxpool.Pool, items []TaskRow) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	at := map[string]int{}
	for i, r := range items {
		ids[i] = r.ID
		at[r.ID] = i
	}
	rs, err := pool.Query(ctx, `
SELECT a.item_id::text, u.id::text, COALESCE(NULLIF(btrim(u.first_name || ' ' || u.last_name), ''), u.email::text)
FROM item_assignees a JOIN users u ON u.id = a.user_id
WHERE a.item_id = ANY($1::uuid[])
ORDER BY 3, 2`, ids)
	if err != nil {
		return err
	}
	defer rs.Close()
	for rs.Next() {
		var item string
		var who Assignee
		if err := rs.Scan(&item, &who.ID, &who.Name); err != nil {
			return err
		}
		items[at[item]].Assignees = append(items[at[item]].Assignees, who)
	}
	return rs.Err()
}
