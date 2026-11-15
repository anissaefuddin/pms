package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// workflowJSON renders one workflow row `w` in the shape of the OpenAPI Workflow schema.
const workflowJSON = `jsonb_build_object('id',w.id,'name',w.name,'is_default',w.is_default,
 'statuses',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',s.id,'name',s.name,'color',s.color,'group',s.grp,
   'auto_assign',s.auto_assign,'lock_transitions',s.lock_transitions,'required_fields',s.required_fields,'actor_rule',s.actor_rule,'require_children_done',s.require_children_done,'approval_approver_ids',s.approval_approver_ids,'approved_status_id',s.approved_status_id,'rejected_status_id',s.rejected_status_id,
   'allowed_next',COALESCE((SELECT jsonb_agg(t.to_status_id ORDER BY t.to_status_id) FROM status_transitions t WHERE t.from_status_id=s.id),'[]'::jsonb))
   ORDER BY s.position) FROM statuses s WHERE s.workflow_id=w.id),'[]'::jsonb))`

// Status colors are a fixed palette of token names (D-35), separate from the accent hues.
var statusPalette = []string{"slate", "blue", "sky", "teal", "green", "lime", "amber", "orange", "red", "pink", "purple", "indigo"}

const maxStatusesPerWorkflow = 30

type wfProblem struct {
	status        int
	code, message string
}

func (p wfProblem) Error() string { return p.message }

func (s *Server) workflowRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/workflows", s.requireUser(s.createWorkflow))
	mux.HandleFunc("PATCH /api/v1/workflows/{workflowId}", s.requireUser(s.renameWorkflow))
	mux.HandleFunc("DELETE /api/v1/workflows/{workflowId}", s.requireUser(s.deleteWorkflow))
	mux.HandleFunc("POST /api/v1/workflows/{workflowId}/statuses", s.requireUser(s.createStatus))
	mux.HandleFunc("PUT /api/v1/workflows/{workflowId}/status-order", s.requireUser(s.reorderStatuses))
	mux.HandleFunc("PATCH /api/v1/statuses/{statusId}", s.requireUser(s.updateStatus))
	mux.HandleFunc("DELETE /api/v1/statuses/{statusId}", s.requireUser(s.deleteStatus))
}

// workflowWrite runs an admin-only change in one transaction and answers with the changed workflow.
func (s *Server) workflowWrite(w http.ResponseWriter, r *http.Request, created bool, work func(context.Context, pgx.Tx) (uuid.UUID, error)) {
	if actorOf(r).Type != policy.Admin {
		writeProblem(w, 403, "forbidden", "administrator access is required")
		return
	}
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		writeError(w, 500, "could not save the workflow")
		return
	}
	defer tx.Rollback(ctx)
	id, err := work(ctx, tx)
	if err == nil {
		err = checkWorkflow(ctx, tx, id)
	}
	var out json.RawMessage
	if err == nil {
		err = tx.QueryRow(ctx, "SELECT "+workflowJSON+" FROM workflows w WHERE w.id=$1", id).Scan(&out)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	status := 200
	if created {
		status = 201
	}
	writeJSON(w, status, out)
}

func writeWorkflowError(w http.ResponseWriter, err error) {
	var problem wfProblem
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &problem):
		writeProblem(w, problem.status, problem.code, problem.message)
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, 404, "not found")
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		writeProblem(w, 409, "name_taken", "that name is already used")
	default:
		writeError(w, 500, "could not save the workflow")
	}
}

// checkWorkflow keeps the two invariants the rest of the app relies on: new Items start in the first
// status, so it must be Active, and an Item must be able to complete, so a Completed status must exist.
func checkWorkflow(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	var firstActive, hasCompleted bool
	var count int
	if err := tx.QueryRow(ctx, `
SELECT COALESCE((SELECT grp='active' FROM statuses WHERE workflow_id=$1 ORDER BY position LIMIT 1),false),
       EXISTS(SELECT 1 FROM statuses WHERE workflow_id=$1 AND grp='completed'),
       (SELECT count(*) FROM statuses WHERE workflow_id=$1)`, id).Scan(&firstActive, &hasCompleted, &count); err != nil {
		return err
	}
	switch {
	case !firstActive:
		return wfProblem{422, "validation_failed", "the first status must be in the Active group"}
	case !hasCompleted:
		return wfProblem{422, "validation_failed", "the workflow needs at least one Completed status"}
	case count > maxStatusesPerWorkflow:
		return wfProblem{422, "validation_failed", "a workflow can have at most 30 statuses"}
	}
	return nil
}

func lockWorkflow(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	var locked uuid.UUID
	return tx.QueryRow(ctx, "SELECT id FROM workflows WHERE id=$1 FOR UPDATE", id).Scan(&locked)
}

// lockStatusWorkflow locks the workflow that owns a status, then the status itself.
func lockStatusWorkflow(ctx context.Context, tx pgx.Tx, statusID uuid.UUID) (uuid.UUID, string, error) {
	var workflow uuid.UUID
	if err := tx.QueryRow(ctx, "SELECT workflow_id FROM statuses WHERE id=$1", statusID).Scan(&workflow); err != nil {
		return workflow, "", err
	}
	if err := lockWorkflow(ctx, tx, workflow); err != nil {
		return workflow, "", err
	}
	var group string
	err := tx.QueryRow(ctx, "SELECT grp::text FROM statuses WHERE id=$1 AND workflow_id=$2 FOR UPDATE", statusID, workflow).Scan(&group)
	return workflow, group, err
}

func sortedIDs(ids []uuid.UUID) []uuid.UUID {
	out := slices.Clone(ids)
	slices.SortFunc(out, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	return out
}

func pathID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return id, pgx.ErrNoRows
	}
	return id, nil
}

func cleanName(value string, max int) (string, error) {
	value = strings.TrimSpace(value)
	if n := utf8.RuneCountInString(value); n < 1 || n > max || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return "", wfProblem{422, "validation_failed", "name must be 1 to " + strconv.Itoa(max) + " characters without control characters"}
	}
	return value, nil
}

func validGroup(group string) error {
	if !slices.Contains([]string{"active", "completed", "deferred", "cancelled"}, group) {
		return wfProblem{422, "validation_failed", "group must be active, completed, deferred, or cancelled"}
	}
	return nil
}

func validColor(color string) error {
	if !slices.Contains(statusPalette, color) {
		return wfProblem{422, "validation_failed", "color must be one of: " + strings.Join(statusPalette, ", ")}
	}
	return nil
}

func (s *Server) createWorkflow(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if actorOf(r).Type != policy.Admin {
		writeProblem(w, 403, "forbidden", "administrator access is required")
		return
	}
	if !decode(w, r, &in) {
		return
	}
	s.workflowWrite(w, r, true, func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
		name, err := cleanName(in.Name, 120)
		if err != nil {
			return uuid.Nil, err
		}
		var id uuid.UUID
		if err := tx.QueryRow(ctx, "INSERT INTO workflows(name) VALUES($1) RETURNING id", name).Scan(&id); err != nil {
			return id, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO statuses(workflow_id,name,grp,color,position) VALUES
($1,'New','active','slate',0),($1,'In Progress','active','blue',1),($1,'Completed','completed','green',2)`, id)
		return id, err
	})
}

func (s *Server) renameWorkflow(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if actorOf(r).Type != policy.Admin {
		writeProblem(w, 403, "forbidden", "administrator access is required")
		return
	}
	if !decode(w, r, &in) {
		return
	}
	s.workflowWrite(w, r, false, func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
		id, err := pathID(r, "workflowId")
		if err != nil {
			return id, err
		}
		name, err := cleanName(in.Name, 120)
		if err != nil {
			return id, err
		}
		tag, err := tx.Exec(ctx, "UPDATE workflows SET name=$2 WHERE id=$1", id, name)
		if err == nil && tag.RowsAffected() == 0 {
			err = pgx.ErrNoRows
		}
		return id, err
	})
}

func (s *Server) deleteWorkflow(w http.ResponseWriter, r *http.Request) {
	if actorOf(r).Type != policy.Admin {
		writeProblem(w, 403, "forbidden", "administrator access is required")
		return
	}
	ctx := r.Context()
	err := func() error {
		id, err := pathID(r, "workflowId")
		if err != nil {
			return err
		}
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if err := lockWorkflow(ctx, tx, id); err != nil {
			return err
		}
		var isDefault, inUse bool
		if err := tx.QueryRow(ctx, `
SELECT w.is_default,
  EXISTS(SELECT 1 FROM items WHERE default_workflow_id=w.id)
  OR EXISTS(SELECT 1 FROM items i JOIN statuses s ON s.id=i.status_id WHERE s.workflow_id=w.id)
FROM workflows w WHERE w.id=$1`, id).Scan(&isDefault, &inUse); err != nil {
			return err
		}
		if isDefault {
			return wfProblem{409, "workflow_in_use", "the default workflow cannot be deleted"}
		}
		if inUse {
			return wfProblem{409, "workflow_in_use", "the workflow is used by projects, folders, or items"}
		}
		if _, err := tx.Exec(ctx, "DELETE FROM workflows WHERE id=$1", id); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}()
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type statusFields struct {
	Name            *string         `json:"name"`
	Group           *string         `json:"group"`
	Color           *string         `json:"color"`
	AutoAssign      *bool           `json:"auto_assign"`
	LockTransitions *bool           `json:"lock_transitions"`
	AllowedNext     *[]uuid.UUID    `json:"allowed_next"`
	RequiredFields  *[]string       `json:"required_fields"`
	ActorRule       *string         `json:"actor_rule"`
	ChildrenDone    *bool           `json:"require_children_done"`
	ApprovalBy      *[]uuid.UUID    `json:"approval_approver_ids"`
	ApprovedStatus  json.RawMessage `json:"approved_status_id"`
	RejectedStatus  json.RawMessage `json:"rejected_status_id"`
}

func (s *Server) createStatus(w http.ResponseWriter, r *http.Request) {
	var in statusFields
	if actorOf(r).Type != policy.Admin {
		writeProblem(w, 403, "forbidden", "administrator access is required")
		return
	}
	if !decode(w, r, &in) {
		return
	}
	s.workflowWrite(w, r, true, func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
		id, err := pathID(r, "workflowId")
		if err != nil {
			return id, err
		}
		if in.Name == nil || in.Group == nil || in.Color == nil {
			return id, wfProblem{422, "validation_failed", "name, group, and color are required"}
		}
		name, err := cleanName(*in.Name, 60)
		if err != nil {
			return id, err
		}
		if err := validGroup(*in.Group); err != nil {
			return id, err
		}
		if err := validColor(*in.Color); err != nil {
			return id, err
		}
		if err := lockWorkflow(ctx, tx, id); err != nil {
			return id, err
		}
		_, err = tx.Exec(ctx, `
INSERT INTO statuses(workflow_id,name,grp,color,position,auto_assign,lock_transitions)
VALUES($1,$2,$3::status_group,$4,(SELECT COALESCE(max(position)+1,0) FROM statuses WHERE workflow_id=$1),$5,$6)`,
			id, name, *in.Group, *in.Color, in.AutoAssign != nil && *in.AutoAssign, in.LockTransitions != nil && *in.LockTransitions)
		return id, err
	})
}

func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request) {
	var in statusFields
	if actorOf(r).Type != policy.Admin {
		writeProblem(w, 403, "forbidden", "administrator access is required")
		return
	}
	if !decode(w, r, &in) {
		return
	}
	s.workflowWrite(w, r, false, func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
		statusID, err := pathID(r, "statusId")
		if err != nil {
			return statusID, err
		}
		if in.Name == nil && in.Group == nil && in.Color == nil && in.AutoAssign == nil && in.LockTransitions == nil && in.AllowedNext == nil && in.RequiredFields == nil && in.ActorRule == nil && in.ChildrenDone == nil && in.ApprovalBy == nil && in.ApprovedStatus == nil && in.RejectedStatus == nil {
			return statusID, wfProblem{422, "validation_failed", "at least one field is required"}
		}
		workflow, oldGroup, err := lockStatusWorkflow(ctx, tx, statusID)
		if err != nil {
			return workflow, err
		}
		if in.Name != nil {
			name, err := cleanName(*in.Name, 60)
			if err != nil {
				return workflow, err
			}
			if _, err := tx.Exec(ctx, "UPDATE statuses SET name=$2 WHERE id=$1", statusID, name); err != nil {
				return workflow, err
			}
		}
		if in.Color != nil {
			if err := validColor(*in.Color); err != nil {
				return workflow, err
			}
			if _, err := tx.Exec(ctx, "UPDATE statuses SET color=$2 WHERE id=$1", statusID, *in.Color); err != nil {
				return workflow, err
			}
		}
		if in.AutoAssign != nil {
			if _, err := tx.Exec(ctx, "UPDATE statuses SET auto_assign=$2 WHERE id=$1", statusID, *in.AutoAssign); err != nil {
				return workflow, err
			}
		}
		if in.LockTransitions != nil {
			if _, err := tx.Exec(ctx, "UPDATE statuses SET lock_transitions=$2 WHERE id=$1", statusID, *in.LockTransitions); err != nil {
				return workflow, err
			}
		}
		if in.RequiredFields != nil {
			unique := slices.Compact(slices.Sorted(slices.Values(*in.RequiredFields)))
			if unique == nil {
				unique = []string{} // NOT NULL column: an empty list clears the requirement
			}
			if len(unique) > 20 || slices.ContainsFunc(unique, func(k string) bool { return !items.ValidGuardField(k) }) {
				return workflow, wfProblem{422, "validation_failed", "required_fields may list due_date, start_date, assignees, description, effort_minutes or custom field ids (at most 20)"}
			}
			if _, err := tx.Exec(ctx, "UPDATE statuses SET required_fields=$2 WHERE id=$1", statusID, unique); err != nil {
				return workflow, err
			}
		}
		if in.ApprovalBy != nil || in.ApprovedStatus != nil || in.RejectedStatus != nil {
			if err := setStatusApproval(ctx, tx, statusID, workflow, in); err != nil {
				return workflow, err
			}
		}
		if in.ChildrenDone != nil {
			if _, err := tx.Exec(ctx, "UPDATE statuses SET require_children_done=$2 WHERE id=$1", statusID, *in.ChildrenDone); err != nil {
				return workflow, err
			}
		}
		if in.ActorRule != nil {
			if !slices.Contains([]string{"anyone", "assignee", "admin"}, *in.ActorRule) {
				return workflow, wfProblem{422, "validation_failed", "actor_rule must be anyone, assignee or admin"}
			}
			if _, err := tx.Exec(ctx, "UPDATE statuses SET actor_rule=$2 WHERE id=$1", statusID, *in.ActorRule); err != nil {
				return workflow, err
			}
		}
		if in.Group != nil && *in.Group != oldGroup {
			if err := validGroup(*in.Group); err != nil {
				return workflow, err
			}
			if _, err := tx.Exec(ctx, "UPDATE statuses SET grp=$2::status_group WHERE id=$1", statusID, *in.Group); err != nil {
				return workflow, err
			}
			// Items already in this status keep a completion time that matches the new group.
			if *in.Group == "completed" {
				_, err = tx.Exec(ctx, "UPDATE items SET completed_at=COALESCE(completed_at,now()) WHERE status_id=$1", statusID)
			} else if oldGroup == "completed" {
				_, err = tx.Exec(ctx, "UPDATE items SET completed_at=NULL WHERE status_id=$1", statusID)
			}
			if err != nil {
				return workflow, err
			}
		}
		if in.AllowedNext != nil {
			next := slices.Compact(sortedIDs(*in.AllowedNext))
			if slices.Contains(next, statusID) {
				return workflow, wfProblem{422, "validation_failed", "a status cannot lead to itself"}
			}
			var found int
			if err := tx.QueryRow(ctx, "SELECT count(*) FROM statuses WHERE workflow_id=$1 AND id=ANY($2::uuid[])", workflow, next).Scan(&found); err != nil {
				return workflow, err
			}
			if found != len(next) {
				return workflow, wfProblem{422, "validation_failed", "allowed_next must list statuses of the same workflow"}
			}
			if _, err := tx.Exec(ctx, "DELETE FROM status_transitions WHERE from_status_id=$1", statusID); err != nil {
				return workflow, err
			}
			if _, err := tx.Exec(ctx, "INSERT INTO status_transitions(from_status_id,to_status_id) SELECT $1,unnest($2::uuid[])", statusID, next); err != nil {
				return workflow, err
			}
		}
		return workflow, nil
	})
}

func (s *Server) deleteStatus(w http.ResponseWriter, r *http.Request) {
	if actorOf(r).Type != policy.Admin {
		writeProblem(w, 403, "forbidden", "administrator access is required")
		return
	}
	s.workflowWrite(w, r, false, func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
		statusID, err := pathID(r, "statusId")
		if err != nil {
			return statusID, err
		}
		var moveTo *uuid.UUID
		if raw := r.URL.Query().Get("move_to"); raw != "" {
			parsed, err := uuid.Parse(raw)
			if err != nil {
				return statusID, wfProblem{422, "validation_failed", "move_to must be a UUID"}
			}
			moveTo = &parsed
		}
		workflow, _, err := lockStatusWorkflow(ctx, tx, statusID)
		if err != nil {
			return workflow, err
		}
		var used int
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM items WHERE status_id=$1", statusID).Scan(&used); err != nil {
			return workflow, err
		}
		if used > 0 {
			if moveTo == nil {
				return workflow, wfProblem{409, "status_in_use", "the status is used by items; choose move_to"}
			}
			if *moveTo == statusID {
				return workflow, wfProblem{422, "validation_failed", "move_to must be a different status"}
			}
			var target string
			if err := tx.QueryRow(ctx, "SELECT grp::text FROM statuses WHERE id=$1 AND workflow_id=$2", *moveTo, workflow).Scan(&target); err != nil {
				return workflow, wfProblem{422, "validation_failed", "move_to must be a status of the same workflow"}
			}
			if _, err := tx.Exec(ctx, `
UPDATE items SET status_id=$2, version=version+1, status_changed_at=now(),
  completed_at=CASE WHEN $3 THEN COALESCE(completed_at,now()) ELSE NULL END
WHERE status_id=$1`, statusID, *moveTo, target == "completed"); err != nil {
				return workflow, err
			}
		}
		_, err = tx.Exec(ctx, "DELETE FROM statuses WHERE id=$1", statusID)
		return workflow, err
	})
}

func (s *Server) reorderStatuses(w http.ResponseWriter, r *http.Request) {
	var in struct {
		StatusIDs []uuid.UUID `json:"status_ids"`
	}
	if actorOf(r).Type != policy.Admin {
		writeProblem(w, 403, "forbidden", "administrator access is required")
		return
	}
	if !decode(w, r, &in) {
		return
	}
	s.workflowWrite(w, r, false, func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
		id, err := pathID(r, "workflowId")
		if err != nil {
			return id, err
		}
		if err := lockWorkflow(ctx, tx, id); err != nil {
			return id, err
		}
		rows, err := tx.Query(ctx, "SELECT id FROM statuses WHERE workflow_id=$1", id)
		if err != nil {
			return id, err
		}
		current, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return id, err
		}
		if len(in.StatusIDs) != len(current) || !slices.Equal(sortedIDs(in.StatusIDs), sortedIDs(current)) {
			return id, wfProblem{422, "validation_failed", "status_ids must list every status of the workflow exactly once"}
		}
		// The (workflow, position) unique constraint is deferred, so positions can be rewritten in one statement.
		_, err = tx.Exec(ctx, `UPDATE statuses SET position=t.ord-1 FROM unnest($1::uuid[]) WITH ORDINALITY AS t(id,ord) WHERE statuses.id=t.id`, in.StatusIDs)
		return id, err
	})
}

// setStatusApproval stores the approval a status starts (WF-03B): up to 20 active approvers and, optionally, the statuses of
// the same workflow an Approved or Rejected result moves the item to.
func setStatusApproval(ctx context.Context, tx pgx.Tx, statusID, workflow uuid.UUID, in statusFields) error {
	bad := func(message string) error { return wfProblem{422, "validation_failed", message} }
	if in.ApprovalBy != nil {
		ids := slices.Compact(slices.SortedFunc(slices.Values(*in.ApprovalBy), func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) }))
		if len(ids) > 20 {
			return bad("a status can name at most 20 approvers")
		}
		var active int
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM users WHERE status='active' AND id=ANY($1)", ids).Scan(&active); err != nil {
			return err
		}
		if active != len(ids) {
			return bad("every approver must be an active user")
		}
		if _, err := tx.Exec(ctx, "UPDATE statuses SET approval_approver_ids=$2 WHERE id=$1", statusID, ids); err != nil {
			return err
		}
	}
	for column, raw := range map[string]json.RawMessage{"approved_status_id": in.ApprovedStatus, "rejected_status_id": in.RejectedStatus} {
		if raw == nil {
			continue
		}
		var target *uuid.UUID
		if json.Unmarshal(raw, &target) != nil {
			return bad(column + " must be a status id or null")
		}
		if target != nil {
			var ok bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM statuses WHERE id=$1 AND workflow_id=$2 AND id<>$3)", *target, workflow, statusID).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				return bad(column + " must be another status of the same workflow")
			}
		}
		if _, err := tx.Exec(ctx, "UPDATE statuses SET "+column+"=$2 WHERE id=$1", statusID, target); err != nil {
			return err
		}
	}
	return nil
}
