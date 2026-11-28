package items

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/schedule"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type AffectedItem struct {
	ID        string  `json:"id"`
	StartDate *string `json:"start_date"`
	DueDate   *string `json:"due_date"`
	Version   int     `json:"version"`
}

func schedulingField(fields map[string]json.RawMessage) bool {
	for _, key := range []string{"start_date", "due_date", "duration_days", "is_milestone"} {
		if _, ok := fields[key]; ok {
			return true
		}
	}
	return false
}

func loadCalendar(ctx context.Context, tx pgx.Tx) (schedule.Calendar, error) {
	var weekdays []int
	var holidays []time.Time
	err := tx.QueryRow(ctx, `
SELECT working_days, ARRAY(SELECT day FROM work_schedule_exceptions)
FROM work_schedules WHERE id=1`).Scan(&weekdays, &holidays)
	if err != nil {
		return schedule.Calendar{}, err
	}
	return schedule.NewCalendar(weekdays, holidays)
}

func dateField(raw json.RawMessage, key string) (*time.Time, error) {
	var date *string
	if json.Unmarshal(raw, &date) != nil {
		return nil, InvalidField{key, key + " must be a date or null"}
	}
	if date == nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.DateOnly, *date)
	if err != nil || parsed.Year() < 1 {
		return nil, InvalidField{key, key + " must be YYYY-MM-DD"}
	}
	return &parsed, nil
}

// An explicitly supplied date remains authoritative. A duration edit derives
// the other endpoint when the caller supplied or retained an anchor.
func prepareSchedulePatch(ctx context.Context, tx pgx.Tx, id string, fields map[string]json.RawMessage, recalculate bool) (schedule.Calendar, bool, error) {
	var before schedule.Dates
	var wasMilestone bool
	err := tx.QueryRow(ctx, `SELECT start_date,due_date,duration_days,is_milestone FROM items WHERE id=$1::uuid`, id).
		Scan(&before.Start, &before.Finish, &before.Duration, &wasMilestone)
	if err != nil {
		return schedule.Calendar{}, false, err
	}
	next, milestone := before, wasMilestone
	startRaw, hasStart := fields["start_date"]
	dueRaw, hasDue := fields["due_date"]
	durationRaw, hasDuration := fields["duration_days"]
	milestoneRaw, hasMilestone := fields["is_milestone"]
	if hasStart {
		next.Start, err = dateField(startRaw, "start_date")
		if err != nil {
			return schedule.Calendar{}, false, err
		}
	}
	if hasDue {
		next.Finish, err = dateField(dueRaw, "due_date")
		if err != nil {
			return schedule.Calendar{}, false, err
		}
	}
	if hasDuration {
		var days *int32
		if json.Unmarshal(durationRaw, &days) != nil || (days != nil && *days < 0) {
			return schedule.Calendar{}, false, Invalid("duration_days must be a non-negative integer or null")
		}
		next.Duration = nil
		if days != nil {
			value := int(*days)
			next.Duration = &value
		}
	}
	if hasMilestone {
		var value *bool
		if json.Unmarshal(milestoneRaw, &value) != nil || value == nil {
			return schedule.Calendar{}, false, Invalid("is_milestone must be a boolean")
		}
		milestone = *value
		if milestone && !hasDuration {
			zero := 0
			next.Duration = &zero
		}
	}
	if !recalculate && sameValue(before.Start, next.Start) && sameValue(before.Finish, next.Finish) &&
		sameValue(before.Duration, next.Duration) && wasMilestone == milestone {
		return schedule.Calendar{}, false, nil
	}
	if milestone && hasStart && hasDue && next.Start != nil && next.Finish != nil && !next.Start.Equal(*next.Finish) {
		return schedule.Calendar{}, false, Invalid("a milestone's start and finish must be the same day")
	}
	cal, err := loadCalendar(ctx, tx)
	if err != nil {
		return schedule.Calendar{}, false, err
	}
	if next.Duration != nil && (hasDuration || milestone && (hasMilestone || hasStart != hasDue)) {
		span := max(0, *next.Duration-1)
		switch {
		case !hasDue && next.Start != nil:
			day := *next.Start
			if *next.Duration != 0 {
				day, err = cal.Offset(day, span)
				if err != nil {
					return cal, false, scheduleError(err)
				}
			}
			fields["due_date"], _ = json.Marshal(day.Format(time.DateOnly))
		case !hasStart && next.Finish != nil:
			day := *next.Finish
			if *next.Duration != 0 {
				// An explicitly supplied nonworking finish remains authoritative.
				if !cal.IsWorkday(day) {
					span++
				}
				day, err = cal.Offset(day, -span)
				if err != nil {
					return cal, false, scheduleError(err)
				}
			}
			fields["start_date"], _ = json.Marshal(day.Format(time.DateOnly))
		}
	} else if !hasDuration && (hasStart || hasDue) && next.Start != nil && next.Finish != nil && !milestone {
		fields["duration_days"], _ = json.Marshal(max(1, cal.Duration(*next.Start, *next.Finish)))
	}
	return cal, true, nil
}

func sameValue[T comparable](a, b *T) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func scheduleError(err error) error {
	if errors.Is(err, schedule.ErrDateRange) {
		return Invalid(err.Error())
	}
	return err
}

// reschedule changes the live successor closure and considers all live
// incoming dependencies for each item in it, including the edited item.
func reschedule(ctx context.Context, tx pgx.Tx, cal schedule.Calendar, actor, root string, rootAlreadyVersioned bool) ([]AffectedItem, error) {
	rows, err := tx.Query(ctx, `
WITH RECURSIVE reachable(id) AS (
    SELECT $1::uuid
    UNION
    SELECT next.successor_id
    FROM reachable r CROSS JOIN LATERAL (
        SELECT d.successor_id FROM item_dependencies d
        WHERE d.predecessor_id=r.id AND EXISTS (
            SELECT 1 FROM items s
            JOIN items pr ON pr.id=d.predecessor_id
            JOIN item_locations l ON l.item_id=s.id AND l.deleted_root_id IS NULL
            JOIN spaces sp ON sp.id=l.space_id AND sp.deleted_at IS NULL
            WHERE s.id=d.successor_id AND s.deleted_at IS NULL AND `+sameOwnerSQL("pr", "s")+`
        )
        OFFSET 0 -- Keep the indexed successor lookup tied to the current item.
    ) next
)
SELECT p.id::text, p.start_date, p.due_date, p.duration_days,
       s.id::text, s.start_date, s.due_date, s.duration_days, d.type, d.lag
FROM reachable r JOIN item_dependencies d ON d.successor_id=r.id
JOIN items s ON s.id=d.successor_id
JOIN items p ON p.id=d.predecessor_id AND p.deleted_at IS NULL
WHERE EXISTS (
    SELECT 1 FROM item_locations l JOIN spaces sp ON sp.id=l.space_id AND sp.deleted_at IS NULL
    WHERE l.item_id=p.id AND l.deleted_root_id IS NULL
) AND `+sameOwnerSQL("p", "s")+`
ORDER BY p.id,s.id`, root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	dates := map[string]schedule.Dates{}
	var links []schedule.Link
	for rows.Next() {
		var link schedule.Link
		var from, to schedule.Dates
		if err := rows.Scan(&link.From, &from.Start, &from.Finish, &from.Duration,
			&link.To, &to.Start, &to.Finish, &to.Duration, &link.Type, &link.Lag); err != nil {
			return nil, err
		}
		dates[link.From], dates[link.To] = from, to
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	changed, err := cal.Propagate(dates, links)
	if err != nil || len(changed) == 0 {
		return nil, scheduleError(err)
	}
	starts := make([]pgtype.Date, len(changed))
	finishes := make([]pgtype.Date, len(changed))
	for i, id := range changed {
		d := dates[id]
		if d.Start != nil {
			starts[i] = pgtype.Date{Time: *d.Start, Valid: true}
		}
		if d.Finish != nil {
			finishes[i] = pgtype.Date{Time: *d.Finish, Valid: true}
		}
	}
	alreadyVersioned := ""
	if rootAlreadyVersioned {
		alreadyVersioned = root
	}
	rows, err = tx.Query(ctx, `
WITH changes AS (
    SELECT i.id,i.start_date AS old_start,i.due_date AS old_due,v.start_date,v.due_date,v.ordinality
    FROM unnest($1::text[],$2::date[],$3::date[]) WITH ORDINALITY v(id,start_date,due_date,ordinality)
    JOIN items i ON i.id=v.id::uuid
), updated AS (
    UPDATE items i SET start_date=c.start_date,due_date=c.due_date,
        version=i.version+CASE WHEN i.id::text=$4 THEN 0 ELSE 1 END
    FROM changes c WHERE i.id=c.id
    RETURNING i.id,i.start_date,i.due_date,i.version,c.old_start,c.old_due,c.ordinality
), audit AS (
    INSERT INTO activity(item_id,actor_id,action,changes,auto)
    SELECT id,$5::uuid,'update',
        CASE WHEN old_start IS DISTINCT FROM start_date
            THEN jsonb_build_object('start_date',jsonb_build_object('from',old_start,'to',start_date)) ELSE '{}'::jsonb END
        || CASE WHEN old_due IS DISTINCT FROM due_date
            THEN jsonb_build_object('due_date',jsonb_build_object('from',old_due,'to',due_date)) ELSE '{}'::jsonb END
    ,true FROM updated WHERE id::text<>$4
)
SELECT id::text,to_char(start_date,'YYYY-MM-DD'),to_char(due_date,'YYYY-MM-DD'),version
FROM updated ORDER BY ordinality`, changed, starts, finishes, alreadyVersioned, actor)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[AffectedItem])
}

func visibleAffected(ctx context.Context, tx pgx.Tx, actor policy.Actor, affected []AffectedItem, exclude string) ([]AffectedItem, error) {
	ids := make([]string, 0, len(affected))
	for _, item := range affected {
		if item.ID != exclude {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		return nil, err
	}
	visible := make([]AffectedItem, 0, len(ids))
	for _, item := range affected {
		if item.ID != exclude && caps[item.ID].Has(policy.View) {
			visible = append(visible, item)
		}
	}
	return visible, nil
}
