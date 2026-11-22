// Package overview builds the Workspace Overview read model: cross-Space KPIs and a per-Space summary.
//
// Access rule (ARCHITECTURE §6.2, D-17.1): only Spaces that policy.VisibleSpaces returns for the actor are counted or
// listed. An item is counted once, by its ID, however many locations it has. Dates are taken in the actor's time zone and
// converted to UTC inside SQL. Nothing here loads items into memory: the database does the counting.
package overview

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/spaces"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Options are the request parameters after validation.
type Options struct {
	SpaceID         string // optional: restrict to one Space the actor can see
	IncludeArchived bool   // archived Spaces are excluded by default
	Sort            string // name | overdue | progress | last_activity
	Limit           int
	Cursor          string
}

// KPIs are the counts for the whole accessible scope.
type KPIs struct {
	AccessibleSpaces  int         `json:"accessible_spaces"`
	ActiveProjects    int         `json:"active_projects"`
	OverdueTasks      int         `json:"overdue_tasks"`
	CompletedThisWeek int         `json:"completed_this_week"`
	AtRiskProjects    Unavailable `json:"at_risk_projects"`
}

// Unavailable marks a metric whose definition is not approved yet. It never carries a number.
type Unavailable struct {
	Status string `json:"status"`
}

// SpaceRow is one Space in the summary table.
type SpaceRow struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Icon         string     `json:"icon_key,omitempty"`
	Archived     bool       `json:"archived"`
	Projects     int        `json:"project_count"`
	Tasks        int        `json:"task_count"`
	Done         int        `json:"done_task_count"`
	Progress     *int       `json:"progress_percent"` // null when the Space has no tasks
	Overdue      int        `json:"overdue_count"`
	Health       string     `json:"health"` // empty | attention | on_track
	LastActivity *time.Time `json:"last_activity"`
	Access       string     `json:"effective_access"` // space_admin | full | editor | limited | read
}

// Result is the response body.
type Result struct {
	GeneratedAt time.Time  `json:"generated_at"`
	Timezone    string     `json:"timezone"`
	KPIs        KPIs       `json:"kpis"`
	Spaces      []SpaceRow `json:"spaces"`
	NextCursor  *string    `json:"next_cursor"`
	Total       int        `json:"total"`
}

// Build returns the overview for the actor. An unknown or invisible Space is spaces.ErrNotFound.
func Build(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, opts Options) (Result, error) {
	tz := userTimezone(ctx, pool, actor.ID)
	byID, ids, err := scope(ctx, pool, actor, opts.SpaceID, opts.IncludeArchived)
	if err != nil {
		return Result{}, err
	}
	res := Result{GeneratedAt: time.Now().UTC(), Timezone: tz, Spaces: []SpaceRow{}}
	res.KPIs.AtRiskProjects = Unavailable{Status: "NOT_AVAILABLE"}
	res.KPIs.AccessibleSpaces = len(ids)
	if len(ids) == 0 {
		return res, nil
	}
	if err := kpis(ctx, pool, tz, ids, &res.KPIs); err != nil {
		return Result{}, err
	}
	rows, err := perSpace(ctx, pool, tz, ids, byID)
	if err != nil {
		return Result{}, err
	}
	sortRows(rows, opts.Sort)
	res.Total = len(rows)
	page, next, err := paginate(rows, opts.Cursor, opts.Limit)
	if err != nil {
		return Result{}, err
	}
	res.Spaces = page
	res.NextCursor = next
	return res, nil
}

// scope returns the shared Spaces the actor may see, keyed by ID and in query order. Personal Spaces stay out of the
// overview. Archived Spaces are left out unless asked for. An unknown or invisible Space is spaces.ErrNotFound.
func scope(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID string, includeArchived bool) (map[string]policy.SpaceAccess, []string, error) {
	visible, err := policy.VisibleSpaces(ctx, pool, actor, spaceID)
	if err != nil {
		return nil, nil, err
	}
	if spaceID != "" && len(visible) == 0 {
		return nil, nil, spaces.ErrNotFound
	}
	byID := map[string]policy.SpaceAccess{}
	var ids []string
	for _, a := range visible {
		if a.Kind != "shared" || (a.Archived && !includeArchived) {
			continue
		}
		byID[a.ID] = a
		ids = append(ids, a.ID)
	}
	return byID, ids, nil
}

func userTimezone(ctx context.Context, pool *pgxpool.Pool, userID string) string {
	var tz string
	if err := pool.QueryRow(ctx, "SELECT timezone FROM users WHERE id=$1::uuid", userID).Scan(&tz); err != nil || tz == "" {
		return "UTC"
	}
	var probe string
	if err := pool.QueryRow(ctx, "SELECT (now() AT TIME ZONE $1)::text", tz).Scan(&probe); err != nil {
		return "UTC" // an unknown zone must not break the page
	}
	return tz
}

// dayWindow is shared by the KPI and per-Space queries: "today" and the start of the week in the actor's zone.
const dayWindow = `WITH t AS (
  SELECT (now() AT TIME ZONE $2)::date AS today,
         date_trunc('week', now() AT TIME ZONE $2) AT TIME ZONE $2 AS week_start)`

func kpis(ctx context.Context, pool *pgxpool.Pool, tz string, ids []string, out *KPIs) error {
	err := pool.QueryRow(ctx, dayWindow+`
SELECT
  count(DISTINCT i.id) FILTER (WHERE i.type='project' AND s.grp='active')::int,
  count(DISTINCT i.id) FILTER (WHERE i.type='task' AND s.grp='active' AND i.due_date < t.today)::int,
  count(DISTINCT i.id) FILTER (WHERE i.type='task' AND s.grp='completed' AND i.completed_at >= t.week_start AND i.completed_at <= now())::int
FROM t, items i
JOIN item_locations l ON l.item_id=i.id AND l.deleted_root_id IS NULL AND l.space_id = ANY($1::uuid[])
LEFT JOIN statuses s ON s.id=i.status_id
WHERE i.deleted_at IS NULL`, ids, tz).Scan(&out.ActiveProjects, &out.OverdueTasks, &out.CompletedThisWeek)
	return err
}

type counts struct {
	projects, tasks, done, overdue int
}

func perSpace(ctx context.Context, pool *pgxpool.Pool, tz string, ids []string, byID map[string]policy.SpaceAccess) ([]SpaceRow, error) {
	rows, err := pool.Query(ctx, dayWindow+`
SELECT l.space_id::text,
  count(DISTINCT i.id) FILTER (WHERE i.type='project' AND s.grp='active')::int,
  count(DISTINCT i.id) FILTER (WHERE i.type='task')::int,
  count(DISTINCT i.id) FILTER (WHERE i.type='task' AND s.grp='completed')::int,
  count(DISTINCT i.id) FILTER (WHERE i.type='task' AND s.grp='active' AND i.due_date < t.today)::int
FROM t, item_locations l
JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
LEFT JOIN statuses s ON s.id=i.status_id
WHERE l.deleted_root_id IS NULL AND l.space_id = ANY($1::uuid[])
GROUP BY l.space_id`, ids, tz)
	if err != nil {
		return nil, err
	}
	stats := map[string]counts{}
	for rows.Next() {
		var id string
		var c counts
		if err := rows.Scan(&id, &c.projects, &c.tasks, &c.done, &c.overdue); err != nil {
			rows.Close()
			return nil, err
		}
		stats[id] = c
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	last, err := lastActivity(ctx, pool, ids)
	if err != nil {
		return nil, err
	}
	out := make([]SpaceRow, 0, len(ids))
	for _, id := range ids {
		a, c := byID[id], stats[id]
		row := SpaceRow{ID: id, Name: a.Name, Icon: a.IconKey, Archived: a.Archived, Projects: c.projects, Tasks: c.tasks,
			Done: c.done, Overdue: c.overdue, LastActivity: last[id], Access: accessLabel(a)}
		if c.tasks > 0 {
			p := c.done * 100 / c.tasks
			row.Progress = &p
		}
		switch {
		case c.tasks == 0 && c.projects == 0:
			row.Health = "empty"
		case c.overdue > 0:
			row.Health = "attention"
		default:
			row.Health = "on_track"
		}
		out = append(out, row)
	}
	return out, nil
}

func lastActivity(ctx context.Context, pool *pgxpool.Pool, ids []string) (map[string]*time.Time, error) {
	rows, err := pool.Query(ctx, `
SELECT l.space_id::text, max(a.created_at) FROM activity a
JOIN item_locations l ON l.item_id=a.item_id AND l.deleted_root_id IS NULL AND l.space_id = ANY($1::uuid[])
GROUP BY l.space_id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]*time.Time{}
	for rows.Next() {
		var id string
		var at time.Time
		if err := rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		out[id] = &at
	}
	return out, rows.Err()
}

// accessLabel names the effective role from the actor's capabilities, the same way the Space settings page does.
func accessLabel(a policy.SpaceAccess) string {
	switch {
	case a.IsAdmin:
		return "space_admin"
	case a.Caps.Has(policy.Share):
		return "full"
	case a.Caps.Has(policy.Edit) && a.Caps.Has(policy.Create):
		return "editor"
	case a.Caps.Has(policy.ChangeStatus):
		return "limited"
	default:
		return "read"
	}
}

func sortRows(rows []SpaceRow, key string) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch key {
		case "overdue":
			if a.Overdue != b.Overdue {
				return a.Overdue > b.Overdue
			}
		case "progress":
			pa, pb := -1, -1
			if a.Progress != nil {
				pa = *a.Progress
			}
			if b.Progress != nil {
				pb = *b.Progress
			}
			if pa != pb {
				return pa < pb
			}
		case "last_activity":
			if a.LastActivity == nil || b.LastActivity == nil {
				if (a.LastActivity == nil) != (b.LastActivity == nil) {
					return a.LastActivity != nil
				}
			} else if !a.LastActivity.Equal(*b.LastActivity) {
				return a.LastActivity.After(*b.LastActivity)
			}
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})
}

// paginate uses an offset cursor. The Space list is bounded by what the actor can see, so an in-memory page is cheap.
func paginate(rows []SpaceRow, cursor string, limit int) ([]SpaceRow, *string, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	offset := 0
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, nil, spaces.ValidationError{Message: "cursor is not valid"}
		}
		n, err := strconv.Atoi(string(raw))
		if err != nil || n < 0 {
			return nil, nil, spaces.ValidationError{Message: "cursor is not valid"}
		}
		offset = n
	}
	if offset > len(rows) {
		offset = len(rows)
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	page := rows[offset:end]
	if end >= len(rows) {
		return page, nil, nil
	}
	next := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d", end)))
	return page, &next, nil
}
