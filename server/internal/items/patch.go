package items

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/schedule"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrForbidden = errors.New("item operation forbidden")
	ErrConflict  = errors.New("item version conflict")
)

type Invalid string

func (e Invalid) Error() string { return string(e) }

type InvalidField struct {
	Field   string
	Message string
}

func (e InvalidField) Error() string { return e.Message }

// Patch reads and writes under one snapshot. A concurrent update that
// invalidates the snapshot is surfaced as a version conflict.
func Patch(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, id string, version int, fields map[string]json.RawMessage, batch *notifications.Batch) (Detail, error) {
	itemID, err := uuid.Parse(id)
	if err != nil {
		return Detail{}, ErrNotFound
	}
	id = itemID.String()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback(ctx)
	item, err := patch(ctx, tx, actor, id, version, fields, batch)
	if err == nil {
		err = tx.Commit(ctx)
	}
	return item, writeError(err)
}

func writeError(err error) error {
	switch pgCode(err) {
	case "40001", "40P01":
		return ErrConflict
	case "23514":
		return Invalid("dates or milestone duration are inconsistent")
	}
	return err
}

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func patch(ctx context.Context, tx pgx.Tx, actor policy.Actor, id string, version int, fields map[string]json.RawMessage, batch *notifications.Batch) (Detail, error) {
	caps, err := policy.ItemCaps(ctx, tx, actor, id)
	if err != nil {
		return Detail{}, err
	}
	if !caps.Has(policy.View) {
		return Detail{}, ErrNotFound
	}
	if len(fields) == 0 {
		return Detail{}, Invalid("at least one editable field is required")
	}
	for key := range fields {
		need := policy.Edit
		if key == "status_id" {
			need = policy.ChangeStatus
		}
		if !caps.Has(need) {
			return Detail{}, ErrForbidden
		}
	}

	var current int
	var kind string
	if err := tx.QueryRow(ctx, "SELECT version, type::text FROM items WHERE id=$1::uuid AND deleted_at IS NULL FOR UPDATE", id).Scan(&current, &kind); err != nil {
		return Detail{}, err
	}
	if current != version {
		return Detail{}, ErrConflict
	}
	var recalculate bool
	if raw, ok := fields["recalculate_duration"]; ok {
		if json.Unmarshal(raw, &recalculate) != nil || !recalculate || schedulingField(fields) {
			return Detail{}, InvalidField{"recalculate_duration", "send true without date, duration, or milestone fields"}
		}
		delete(fields, "recalculate_duration")
	}
	before, err := snapshot(ctx, tx, id)
	if err != nil {
		return Detail{}, err
	}
	var cal schedule.Calendar
	var dateChanged bool
	if schedulingField(fields) {
		cal, dateChanged, err = prepareSchedulePatch(ctx, tx, id, fields, false)
		if err != nil {
			return Detail{}, err
		}
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if err := applyFields(ctx, tx, actor, id, kind, fields, keys, true); err != nil {
		return Detail{}, err
	}
	if recalculate {
		days, err := effortDuration(ctx, tx, id)
		if err != nil {
			return Detail{}, err
		}
		raw, _ := json.Marshal(days)
		dates := map[string]json.RawMessage{"duration_days": raw}
		cal, dateChanged, err = prepareSchedulePatch(ctx, tx, id, dates, true)
		if err != nil {
			return Detail{}, err
		}
		keys = keys[:0]
		for key := range dates {
			keys = append(keys, key)
		}
		if err := applyFields(ctx, tx, actor, id, kind, dates, keys, false); err != nil {
			return Detail{}, err
		}
	}
	var affected []AffectedItem
	if dateChanged {
		affected, err = reschedule(ctx, tx, cal, actor.ID, id, true)
		if err != nil {
			return Detail{}, err
		}
	}
	after, err := snapshot(ctx, tx, id)
	if err != nil {
		return Detail{}, err
	}
	if err := recordChange(ctx, tx, id, actor.ID, "update", before, after); err != nil {
		return Detail{}, err
	}
	if err := notifyChanges(ctx, tx, batch, actor, id, before, after, false); err != nil {
		return Detail{}, err
	}
	item, err := readDetail(ctx, tx, actor, id)
	if err != nil || len(affected) == 0 {
		return item, err
	}
	item.ScheduleChanges = affected
	visible, err := visibleAffected(ctx, tx, actor, affected, id)
	if err != nil || len(visible) == 0 {
		return item, err
	}
	encoded, err := json.Marshal(visible)
	if err != nil {
		return Detail{}, err
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(item.Body, &body); err != nil {
		return Detail{}, err
	}
	body["affected_items"] = encoded
	item.Body, err = json.Marshal(body)
	return item, err
}

func applyFields(ctx context.Context, tx pgx.Tx, actor policy.Actor, id, kind string, fields map[string]json.RawMessage, keys []string, incrementVersion bool) error {
	sets := []string{}
	if incrementVersion {
		sets = append(sets, "version=version+1")
	}
	args := []any{id}
	set := func(column, cast string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s=$%d%s", column, len(args), cast))
	}
	var assignees []uuid.UUID
	var replaceAssignees, autoAssign bool
	for _, key := range keys {
		raw := fields[key]
		switch key {
		case "title":
			var title string
			if json.Unmarshal(raw, &title) != nil {
				return InvalidField{"title", "title must be a string"}
			}
			title = strings.TrimSpace(title)
			if n := utf8.RuneCountInString(title); n < 1 || n > 500 || strings.ContainsRune(title, 0) {
				return InvalidField{"title", "title must be 1 to 500 characters"}
			}
			set(key, "", title)
		case "start_date", "due_date":
			date, err := dateField(raw, key)
			if err != nil {
				return err
			}
			set(key, "::date", date)
		case "duration_days":
			var days *int32
			if json.Unmarshal(raw, &days) != nil || (days != nil && *days < 0) {
				return Invalid("duration_days must be a non-negative integer or null")
			}
			set(key, "", days)
		case "effort_minutes":
			var minutes *int32
			if json.Unmarshal(raw, &minutes) != nil || (minutes != nil && *minutes < 0) {
				return InvalidField{key, "effort_minutes must be a non-negative integer or null"}
			}
			set(key, "", minutes)
		case "progress_method":
			var method string
			if kind != "project" || json.Unmarshal(raw, &method) != nil || !validProgressMethod(method) {
				return InvalidField{key, "project progress method must be task_count, weighted_duration, or average_percent"}
			}
			set(key, "", method)
		case "progress_percent":
			var percent *int16
			if kind != "task" || json.Unmarshal(raw, &percent) != nil || (percent != nil && (*percent < 0 || *percent > 100)) {
				return InvalidField{key, "task progress must be a whole percentage from 0 to 100 or null"}
			}
			set(key, "", percent)
		case "assignee_allocations":
			// Applied after any membership change so retained and new assignees can both be updated.
		case "importance":
			var level string
			if json.Unmarshal(raw, &level) != nil || (level != "high" && level != "normal" && level != "low") {
				return InvalidField{key, "importance must be high, normal, or low"}
			}
			set(key, "", level)
		case "is_milestone":
			var value *bool
			if json.Unmarshal(raw, &value) != nil || value == nil {
				return Invalid("is_milestone must be a boolean")
			}
			set(key, "", *value)
			if _, explicit := fields["duration_days"]; *value && !explicit {
				sets = append(sets, "duration_days=0")
			}
		case "status_id", "owner_id":
			var target *uuid.UUID
			if json.Unmarshal(raw, &target) != nil || target == nil || *target == uuid.Nil {
				return Invalid(key + " must be a UUID")
			}
			if key == "owner_id" {
				if kind != "project" {
					return Invalid("owner_id applies only to projects")
				}
				if err := lockActiveUser(ctx, tx, *target); err != nil {
					return err
				}
			} else {
				var completed, auto, allowed bool
				err := tx.QueryRow(ctx, `
SELECT next.grp='completed', next.auto_assign,
  (current.id IS NULL OR current.id=next.id OR NOT current.lock_transitions
   OR EXISTS(SELECT 1 FROM status_transitions t WHERE t.from_status_id=current.id AND t.to_status_id=next.id))
FROM statuses next, items i
LEFT JOIN statuses current ON current.id=i.status_id
WHERE i.id=$2::uuid AND next.id=$1
  AND next.workflow_id=COALESCE(current.workflow_id,
     (SELECT id FROM workflows WHERE is_default))`, *target, id).Scan(&completed, &auto, &allowed)
				if errors.Is(err, pgx.ErrNoRows) {
					return Invalid("status must belong to the Item workflow")
				}
				if err != nil {
					return err
				}
				// Transitions limit edits of an existing Item, not the status it is created with.
				if !allowed && incrementVersion {
					return InvalidField{"status_id", "this workflow does not allow that status change"}
				}
				// Entering a status with auto-assign makes the person who changed it an assignee (applied after assignee_ids).
				autoAssign = autoAssign || (auto && incrementVersion)
				if completed {
					sets = append(sets, "completed_at=COALESCE(completed_at,now())")
				} else {
					sets = append(sets, "completed_at=NULL")
				}
				sets = append(sets, "status_changed_at=CASE WHEN status_id IS DISTINCT FROM '"+target.String()+"'::uuid THEN now() ELSE status_changed_at END")
			}
			set(key, "", *target)
		case "assignee_ids":
			if json.Unmarshal(raw, &assignees) != nil || assignees == nil {
				return Invalid("assignee_ids must be an array of UUIDs")
			}
			replaceAssignees = true
			for _, uid := range assignees {
				if err := lockActiveUser(ctx, tx, uid); err != nil {
					return err
				}
			}
		case "fields":
			if !incrementVersion {
				return Invalid("custom field values are set through PATCH after item creation")
			}
			updated, err := updateCustomFieldValues(ctx, tx, actor, id, raw)
			if err != nil {
				return err
			}
			set(key, "::jsonb", updated)
		case "description":
			plain, err := descriptionText(raw)
			if err != nil {
				return err
			}
			set(key, "::jsonb", raw)
			set("description_text", "", plain)
		default:
			return Invalid("unknown editable field: " + key)
		}
	}
	if len(sets) != 0 {
		if _, err := tx.Exec(ctx, "UPDATE items SET "+strings.Join(sets, ",")+" WHERE id=$1::uuid", args...); err != nil {
			return err
		}
	}
	if replaceAssignees {
		if err := setAssignees(ctx, tx, id, assignees); err != nil {
			return err
		}
	}
	if autoAssign {
		for _, table := range []string{"item_assignees", "item_followers"} {
			if _, err := tx.Exec(ctx, "INSERT INTO "+table+"(item_id,user_id) VALUES($1::uuid,$2::uuid) ON CONFLICT DO NOTHING", id, actor.ID); err != nil {
				return err
			}
		}
	}
	if raw, ok := fields["assignee_allocations"]; ok {
		return updateAllocations(ctx, tx, id, raw)
	}
	return nil
}

func lockActiveUser(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	var active bool
	err := tx.QueryRow(ctx, "SELECT status='active' FROM users WHERE id=$1 FOR SHARE", id).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !active) {
		return Invalid("assigned users must be active")
	}
	return err
}

func setAssignees(ctx context.Context, tx pgx.Tx, id string, assignees []uuid.UUID) error {
	if _, err := tx.Exec(ctx, "DELETE FROM item_assignees WHERE item_id=$1::uuid AND user_id<>ALL($2::uuid[])", id, assignees); err != nil {
		return err
	}
	for _, table := range []string{"item_assignees", "item_followers"} {
		if _, err := tx.Exec(ctx, "INSERT INTO "+table+"(item_id,user_id) SELECT $1::uuid, unnest($2::uuid[]) ON CONFLICT DO NOTHING", id, assignees); err != nil {
			return err
		}
	}
	return nil
}

func snapshot(ctx context.Context, tx pgx.Tx, id string) (json.RawMessage, error) {
	var out json.RawMessage
	err := tx.QueryRow(ctx, `
SELECT to_jsonb(i) - ARRAY['search','description_text','updated_at','version']
  || jsonb_build_object('assignee_ids', COALESCE((
       SELECT jsonb_agg(user_id ORDER BY user_id) FROM item_assignees WHERE item_id=i.id
     ), '[]'::jsonb))
  || COALESCE((
       SELECT jsonb_build_object('assignee_allocations', jsonb_object_agg(user_id,
           jsonb_build_object('unit', allocation_unit, 'value', allocation_value)))
       FROM item_assignees WHERE item_id=i.id AND allocation_unit IS NOT NULL
       HAVING count(*) > 0
     ), '{}'::jsonb)
FROM items i WHERE i.id=$1::uuid`, id).Scan(&out)
	return out, err
}

func recordChange(ctx context.Context, tx pgx.Tx, id, actor, action string, before, after json.RawMessage) error {
	_, err := tx.Exec(ctx, `
INSERT INTO activity(item_id,actor_id,action,changes)
SELECT $1::uuid,$2::uuid,$3,COALESCE(jsonb_object_agg(COALESCE(a.key,b.key),
  jsonb_build_object('from',a.value,'to',b.value)), '{}')
FROM jsonb_each($4::jsonb) a FULL JOIN jsonb_each($5::jsonb) b USING (key)
WHERE a.value IS DISTINCT FROM b.value`, id, actor, action, before, after)
	return err
}
