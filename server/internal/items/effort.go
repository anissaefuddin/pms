package items

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/anissaefuddin/pms/server/internal/schedule"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type allocationUpdate struct {
	AssigneeID uuid.UUID       `json:"assignee_id"`
	Allocation json.RawMessage `json:"allocation"`
}

func updateAllocations(ctx context.Context, tx pgx.Tx, id string, raw json.RawMessage) error {
	var updates []allocationUpdate
	const field = "assignee_allocations"
	if json.Unmarshal(raw, &updates) != nil || updates == nil {
		return InvalidField{field, "assignee_allocations must be an array"}
	}
	seen := make(map[uuid.UUID]bool, len(updates))
	for i, update := range updates {
		if update.AssigneeID == uuid.Nil || seen[update.AssigneeID] || update.Allocation == nil {
			return InvalidField{field, "each allocation requires a distinct assignee_id and an allocation value or null"}
		}
		seen[update.AssigneeID] = true
		var allocation *struct {
			Unit  string `json:"unit"`
			Value *int32 `json:"value"`
		}
		if json.Unmarshal(update.Allocation, &allocation) != nil || (allocation != nil &&
			((allocation.Unit != "percent" && allocation.Unit != "minutes_per_day") ||
				allocation.Value == nil || *allocation.Value < 0)) {
			return InvalidField{field, "allocation requires percent or minutes_per_day and a non-negative integer value"}
		}
		updates[i].Allocation, _ = json.Marshal(allocation)
	}
	if len(updates) == 0 {
		return nil
	}
	canonical, err := json.Marshal(updates)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
UPDATE item_assignees a SET allocation_unit=v.allocation->>'unit',
    allocation_value=(v.allocation->>'value')::integer
FROM jsonb_to_recordset($2::jsonb) v(assignee_id uuid, allocation jsonb)
WHERE a.item_id=$1::uuid AND a.user_id=v.assignee_id`, id, canonical)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != int64(len(updates)) {
		return InvalidField{field, "allocation can only be set for a current assignee"}
	}
	return nil
}

func effortDuration(ctx context.Context, tx pgx.Tx, id string) (int32, error) {
	var effort *int32
	var milestone bool
	var hours int
	err := tx.QueryRow(ctx, `
SELECT i.effort_minutes,i.is_milestone,w.daily_hours
FROM items i CROSS JOIN work_schedules w WHERE i.id=$1::uuid AND w.id=1`, id).Scan(&effort, &milestone, &hours)
	if err != nil {
		return 0, err
	}
	if milestone {
		return 0, Invalid("milestones have a fixed duration of zero")
	}
	if effort == nil {
		return 0, InvalidField{"effort_minutes", "set effort before recalculating duration"}
	}
	rows, err := tx.Query(ctx, `
SELECT allocation_unit,allocation_value FROM item_assignees
WHERE item_id=$1::uuid AND allocation_unit IS NOT NULL`, id)
	if err != nil {
		return 0, err
	}
	allocations, err := pgx.CollectRows(rows, pgx.RowToStructByPos[schedule.Allocation])
	if err != nil {
		return 0, err
	}
	days, err := schedule.EffortDuration(*effort, hours, allocations)
	if errors.Is(err, schedule.ErrNoAllocationCapacity) {
		return 0, InvalidField{"assignee_allocations", err.Error()}
	}
	if errors.Is(err, schedule.ErrEffortDurationRange) {
		return 0, InvalidField{"effort_minutes", err.Error()}
	}
	return days, err
}
