package items

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
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
	keys := make([]string, 0, len(fields))
	for key := range fields {
		need := policy.Edit
		if key == "status_id" {
			need = policy.ChangeStatus
		}
		if !caps.Has(need) {
			return Detail{}, ErrForbidden
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var current int
	var kind string
	if err := tx.QueryRow(ctx, "SELECT version, type::text FROM items WHERE id=$1::uuid AND deleted_at IS NULL FOR UPDATE", id).Scan(&current, &kind); err != nil {
		return Detail{}, err
	}
	if current != version {
		return Detail{}, ErrConflict
	}
	before, err := snapshot(ctx, tx, id)
	if err != nil {
		return Detail{}, err
	}
	if err := applyFields(ctx, tx, id, kind, fields, keys, true); err != nil {
		return Detail{}, err
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
	return readDetail(ctx, tx, actor, id)
}

func applyFields(ctx context.Context, tx pgx.Tx, id, kind string, fields map[string]json.RawMessage, keys []string, incrementVersion bool) error {
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
	var replaceAssignees bool
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
			var date *string
			if json.Unmarshal(raw, &date) != nil {
				return InvalidField{key, key + " must be a date or null"}
			}
			if date != nil {
				if parsed, err := time.Parse("2006-01-02", *date); err != nil || parsed.Year() < 1 {
					return InvalidField{key, key + " must be YYYY-MM-DD"}
				}
			}
			set(key, "::text::date", date)
		case "duration_days":
			var days *int32
			if json.Unmarshal(raw, &days) != nil || (days != nil && *days < 0) {
				return Invalid("duration_days must be a non-negative integer or null")
			}
			set(key, "", days)
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
				var completed bool
				err := tx.QueryRow(ctx, `
SELECT next.grp='completed' FROM statuses next, items i
LEFT JOIN statuses current ON current.id=i.status_id
WHERE i.id=$2::uuid AND next.id=$1
  AND next.workflow_id=COALESCE(current.workflow_id,
     (SELECT id FROM workflows WHERE is_default))`, *target, id).Scan(&completed)
				if errors.Is(err, pgx.ErrNoRows) {
					return Invalid("status must belong to the Item workflow")
				}
				if err != nil {
					return err
				}
				if completed {
					sets = append(sets, "completed_at=COALESCE(completed_at,now())")
				} else {
					sets = append(sets, "completed_at=NULL")
				}
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
		return setAssignees(ctx, tx, id, assignees)
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
	if _, err := tx.Exec(ctx, "DELETE FROM item_assignees WHERE item_id=$1::uuid", id); err != nil {
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
