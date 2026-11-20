package items

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

// GuardError is a refused status change: every unmet requirement is listed, and nothing was written (the caller's
// transaction rolls back). Clients show Problems next to the controls that can fix them.
type GuardError struct{ Problems []InvalidField }

func (e GuardError) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = p.Message
	}
	return "status change blocked: " + strings.Join(parts, "; ")
}

// GuardFields are the built-in fields a status can require; a custom field is required by its id.
var GuardFields = []string{"due_date", "start_date", "assignees", "description", "effort_minutes"}

var guardLabels = map[string]string{"due_date": "a due date", "start_date": "a start date", "assignees": "an assignee", "description": "a description", "effort_minutes": "an effort estimate"}

// ValidGuardField reports whether key can appear in a status's required_fields.
func ValidGuardField(key string) bool { return slices.Contains(GuardFields, key) || validFieldID(key) }

// checkStatusGuard runs after the rest of the PATCH was applied inside the transaction, so a change that sets the required
// fields and the status together passes. It is the single evaluator for every path that moves an item to a status.
func checkStatusGuard(ctx context.Context, tx pgx.Tx, actor policy.Actor, itemID, statusID string) error {
	var required []string
	var rule string
	var childrenDone bool
	if err := tx.QueryRow(ctx, "SELECT required_fields, actor_rule, require_children_done FROM statuses WHERE id=$1::uuid", statusID).Scan(&required, &rule, &childrenDone); err != nil {
		return err
	}
	if len(required) == 0 && rule == "anyone" && !childrenDone {
		return nil
	}
	var due, start, desc, effort bool
	var assignee bool
	var fields map[string]json.RawMessage
	if err := tx.QueryRow(ctx, `SELECT i.due_date IS NOT NULL, i.start_date IS NOT NULL, i.description_text<>'', i.effort_minutes IS NOT NULL,
 EXISTS(SELECT 1 FROM item_assignees a WHERE a.item_id=i.id), COALESCE(i.fields,'{}')
 FROM items i WHERE i.id=$1::uuid`, itemID).Scan(&due, &start, &desc, &effort, &assignee, &fields); err != nil {
		return err
	}
	have := map[string]bool{"due_date": due, "start_date": start, "description": desc, "effort_minutes": effort, "assignees": assignee}
	hidden, err := HiddenFieldIDs(ctx, tx, actor, GrantView)
	if err != nil {
		return err
	}
	var problems []InvalidField
	for _, key := range required {
		ok := have[key]
		label := guardLabels[key]
		if validFieldID(key) {
			v, present := fields[key]
			ok = present && string(v) != "null" && string(v) != `""` && string(v) != "[]"
			label = "the required field"
			if !hiddenFor(hidden, key) { // a field the mover cannot read is not named, so its name is not disclosed
				_ = tx.QueryRow(ctx, "SELECT 'the field \"'||name||'\"' FROM custom_fields WHERE id=$1::uuid", key).Scan(&label)
			}
		}
		if !ok {
			field := key
			if validFieldID(key) {
				field = "fields." + key
			}
			problems = append(problems, InvalidField{field, "this status needs " + label})
		}
	}
	if childrenDone { // WF-05
		var open int
		if err := tx.QueryRow(ctx, `SELECT count(DISTINCT c.id) FROM item_locations l JOIN items c ON c.id=l.item_id AND c.deleted_at IS NULL
JOIN statuses cs ON cs.id=c.status_id WHERE l.parent_item_id=$1::uuid AND l.deleted_root_id IS NULL AND cs.grp NOT IN ('completed','cancelled')`, itemID).Scan(&open); err != nil {
			return err
		}
		if open > 0 {
			problems = append(problems, InvalidField{"subitems", "all subitems must be completed or cancelled first (" + strconv.Itoa(open) + " still open)"})
		}
	}
	switch rule {
	case "assignee":
		var mine bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM item_assignees WHERE item_id=$1::uuid AND user_id=$2::uuid)", itemID, actor.ID).Scan(&mine); err != nil {
			return err
		}
		if !mine && actor.Type != policy.Admin {
			problems = append(problems, InvalidField{"status_id", "only an assignee or an administrator can move an item into this status"})
		}
	case "admin":
		if actor.Type != policy.Admin {
			problems = append(problems, InvalidField{"status_id", "only an administrator can move an item into this status"})
		}
	}
	if len(problems) > 0 {
		return GuardError{problems}
	}
	return nil
}

// OnStatusEntered is called inside the PATCH transaction after the guard passed and the item really changed status.
// The HTTP layer registers the automatic approval here (WF-03B); the items package cannot import it.
var OnStatusEntered func(ctx context.Context, tx pgx.Tx, actor policy.Actor, itemID, statusID string, batch *notifications.Batch) error

func hiddenFor(hidden []string, id string) bool { return fieldHidden(hidden, id) }
