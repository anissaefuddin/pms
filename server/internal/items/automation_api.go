package items

import (
	"context"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/schedule"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The automation engine uses the same calendar and the same location rules as people do; these wrappers only expose them.

// LoadCalendar is the instance work schedule, the one Gantt and the work schedule page use.
func LoadCalendar(ctx context.Context, tx pgx.Tx) (schedule.Calendar, error) {
	return loadCalendar(ctx, tx)
}

// ReplaceLocationsTx sets an item's locations inside the caller's transaction, under the actor's rights.
func ReplaceLocationsTx(ctx context.Context, tx pgx.Tx, actor policy.Actor, itemID string, want []Location) (Detail, error) {
	id, err := uuid.Parse(itemID)
	if err != nil {
		return Detail{}, ErrNotFound
	}
	return replaceLocations(ctx, tx, actor, itemID, id, want)
}

// WorkspaceZone is the workspace's time zone for evaluating deadlines. An unknown or missing name falls back to UTC, so a bad setting
// never stops automation; the caller gets UTC and the rule's dates are still consistent.
func WorkspaceZone(ctx context.Context, tx pgx.Tx) *time.Location {
	var name string
	if err := tx.QueryRow(ctx, "SELECT timezone FROM work_schedules WHERE id=1").Scan(&name); err != nil {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}
