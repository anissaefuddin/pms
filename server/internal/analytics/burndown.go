package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BurndownDays is how many days a burndown widget shows, ending today.
const BurndownDays = 30

// Burndown returns, for each of the last BurndownDays days, how much work was still open at the end of that day: tasks that
// existed by then and were not yet completed. Metric count counts tasks, effort_minutes sums their effort.
// shortcut: a task completed now counts as completed from its completed_at day, and a task open today counts as open on every day
// since it was created, so a task that was finished and reopened is not shown as finished in between; upgrade with a status history.
func Burndown(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, q Query) (Result, error) {
	if err := q.Validate(); err != nil {
		return Result{}, err
	}
	if q.ReportingOn != "tasks" || (q.Metric != "count" && q.Metric != "effort_minutes") || q.GroupBy != "" || q.BreakBy != "" {
		return Result{}, items.Invalid("a burndown reports on tasks with the count or effort_minutes metric, without grouping")
	}
	q.Filters.StatusGroups = nil // the chart is about the open and the completed ones together
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Result{}, mapErr(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", Timeout.Milliseconds())); err != nil {
		return Result{}, err
	}
	hits, err := candidates(ctx, tx, actor, q)
	if err != nil {
		return Result{}, mapErr(err)
	}
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.itemID
	}
	visible := map[string]bool{}
	for start := 0; start < len(ids); start += 1000 {
		end := min(start+1000, len(ids))
		caps, err := policy.ItemsCaps(ctx, tx, actor, ids[start:end], nil)
		if err != nil {
			return Result{}, mapErr(err)
		}
		for _, id := range ids[start:end] {
			visible[id] = caps[id].Has(policy.View)
		}
	}
	rows, err := tx.Query(ctx, `SELECT i.id::text,i.created_at::date,CASE WHEN st.grp='completed' THEN coalesce(i.completed_at,i.status_changed_at)::date END
FROM items i LEFT JOIN statuses st ON st.id=i.status_id WHERE i.id=ANY(ARRAY(SELECT unnest($1::text[])::uuid))`, ids)
	if err != nil {
		return Result{}, mapErr(err)
	}
	type span struct{ created, done *time.Time }
	spans := map[string]span{}
	for rows.Next() {
		var id string
		var s span
		if err := rows.Scan(&id, &s.created, &s.done); err != nil {
			rows.Close()
			return Result{}, mapErr(err)
		}
		spans[id] = s
	}
	if err := rows.Err(); err != nil {
		return Result{}, mapErr(err)
	}
	var today time.Time
	if err := tx.QueryRow(ctx, "SELECT current_date").Scan(&today); err != nil {
		return Result{}, mapErr(err)
	}
	out := Result{Rows: make([]Row, 0, BurndownDays)}
	for i := BurndownDays - 1; i >= 0; i-- {
		day := today.AddDate(0, 0, -i)
		var open float64
		for _, h := range hits {
			s := spans[h.itemID]
			if !visible[h.itemID] || s.created == nil || s.created.After(day) || (s.done != nil && !s.done.After(day)) {
				continue
			}
			open += h.value
		}
		key := day.Format(time.DateOnly)
		out.Rows = append(out.Rows, Row{Group: Label{Key: key, Label: key}, Value: open})
		out.Total = open
	}
	return out, nil
}
