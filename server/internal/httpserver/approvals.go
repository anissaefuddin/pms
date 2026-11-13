package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/anissaefuddin/pms/server/internal/events"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const approvalOutcome = `CASE
 WHEN a.policy='any' AND EXISTS(SELECT 1 FROM approval_approvers x WHERE x.approval_id=a.id AND x.decision='approved') THEN 'approved'
 WHEN a.policy='all' AND EXISTS(SELECT 1 FROM approval_approvers x WHERE x.approval_id=a.id AND x.decision='rejected') THEN 'rejected'
 WHEN EXISTS(SELECT 1 FROM approval_approvers x WHERE x.approval_id=a.id)
  AND NOT EXISTS(SELECT 1 FROM approval_approvers x WHERE x.approval_id=a.id AND x.decision IS NULL)
 THEN CASE WHEN a.policy='any' THEN 'rejected' ELSE 'approved' END
 ELSE 'pending' END`

const approvalSelect = `SELECT a.version, jsonb_build_object('id',a.id,'item_id',a.item_id,'description',a.description,'due_date',a.due_date,
 'created_by',jsonb_build_object('id',cu.id,'name',concat_ws(' ',cu.first_name,cu.last_name)),'version',a.version,'created_at',a.created_at,
 'policy',a.policy,'attachment_id',a.attachment_id,'attachment_version',a.attachment_version,'status',` + approvalOutcome + `,
 'approved_count',(SELECT count(*) FROM approval_approvers x WHERE x.approval_id=a.id AND x.decision='approved'),
 'total',(SELECT count(*) FROM approval_approvers x WHERE x.approval_id=a.id),
 'approvers',COALESCE((SELECT jsonb_agg(jsonb_build_object('user',jsonb_build_object('id',u.id,'name',concat_ws(' ',u.first_name,u.last_name)),
     'decision',x.decision,'comment',x.comment,'decided_at',x.decided_at) ORDER BY u.first_name,u.last_name,u.id)
   FROM approval_approvers x JOIN users u ON u.id=x.user_id WHERE x.approval_id=a.id),'[]'::jsonb))
FROM approvals a JOIN users cu ON cu.id=a.created_by `

const maxApprovalsPerItem = 50

func (s *Server) approvalRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/items/{itemId}/approvals", s.requireUser(s.listApprovals))
	mux.HandleFunc("POST /api/v1/items/{itemId}/approvals", s.requireUser(s.writeApproval))
	mux.HandleFunc("GET /api/v1/items/{itemId}/approval-events", s.requireUser(s.listApprovalEvents))
	mux.HandleFunc("PATCH /api/v1/approvals/{approvalId}", s.requireUser(s.writeApproval))
	mux.HandleFunc("DELETE /api/v1/approvals/{approvalId}", s.requireUser(s.writeApproval))
	mux.HandleFunc("PUT /api/v1/approvals/{approvalId}/decision", s.requireUser(s.writeApproval))
}

func (s *Server) listApprovals(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err := approvalCaps(ctx, tx, actorOf(r), id, policy.View); err != nil {
		writeItemError(w, err)
		return
	}
	rows, err := tx.Query(ctx, approvalSelect+"WHERE a.item_id=$1 ORDER BY a.created_at DESC,a.id DESC", id)
	if err != nil {
		writeItemError(w, err)
		return
	}
	type row struct {
		Version int
		Body    json.RawMessage
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
	if err != nil {
		writeItemError(w, err)
		return
	}
	out := make([]json.RawMessage, 0, len(list))
	for _, entry := range list {
		out = append(out, entry.Body)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) listApprovalEvents(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err := approvalCaps(ctx, tx, actorOf(r), id, policy.View); err != nil {
		writeItemError(w, err)
		return
	}
	var out json.RawMessage
	err = tx.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',e.id,'approval_id',e.approval_id,'kind',e.kind,
 'actor',CASE WHEN a.id IS NULL THEN NULL ELSE jsonb_build_object('id',a.id,'name',concat_ws(' ',a.first_name,a.last_name)) END,
 'subject',CASE WHEN s.id IS NULL THEN NULL ELSE jsonb_build_object('id',s.id,'name',concat_ws(' ',s.first_name,s.last_name)) END,
 'decision',e.decision,'comment',e.comment,'item_version',e.item_version,'created_at',e.created_at) ORDER BY e.id DESC),'[]'::jsonb)
FROM approval_events e LEFT JOIN users a ON a.id=e.actor_id LEFT JOIN users s ON s.id=e.subject_id WHERE e.item_id=$1`, id).Scan(&out)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// approvalCaps loads the actor's capabilities on an item and requires one of them; no view access is 404.
func approvalCaps(ctx context.Context, tx pgx.Tx, actor policy.Actor, itemID uuid.UUID, need policy.Capability) (policy.Caps, error) {
	caps, err := policy.ItemCaps(ctx, tx, actor, itemID.String())
	if err != nil {
		return caps, err
	}
	if !caps.Has(policy.View) {
		return caps, items.ErrNotFound
	}
	if !caps.Has(need) {
		return caps, items.ErrForbidden
	}
	return caps, nil
}

func approvalText(value string, field string, max int) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) || utf8.RuneCountInString(value) > max {
		return "", items.InvalidField{Field: field, Message: field + " must be at most 2000 characters"}
	}
	return value, nil
}

func approvalDate(raw json.RawMessage) (*string, error) {
	var value *string
	if json.Unmarshal(raw, &value) != nil {
		return nil, items.InvalidField{Field: "due_date", Message: "due_date must be a date (YYYY-MM-DD) or null"}
	}
	if value != nil {
		if parsed, err := time.Parse(time.DateOnly, *value); err != nil || parsed.Format(time.DateOnly) != *value {
			return nil, items.InvalidField{Field: "due_date", Message: "due_date must be a date (YYYY-MM-DD) or null"}
		}
	}
	return value, nil
}

// checkApprovers requires distinct, active users who can see the item, so a request never goes to someone who cannot open it.
func checkApprovers(ctx context.Context, tx pgx.Tx, ids []uuid.UUID, itemID uuid.UUID, file bool) ([]uuid.UUID, error) {
	unique := slices.Clone(ids)
	slices.SortFunc(unique, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	unique = slices.Compact(unique)
	if len(unique) < 1 || len(unique) > 20 {
		return nil, items.InvalidField{Field: "approver_ids", Message: "choose between 1 and 20 approvers"}
	}
	rows, err := tx.Query(ctx, "SELECT id::text,user_type::text FROM users WHERE status='active' AND id=ANY($1::uuid[])", unique)
	if err != nil {
		return nil, err
	}
	found, err := pgx.CollectRows(rows, pgx.RowToStructByPos[policy.Actor])
	if err != nil {
		return nil, err
	}
	if len(found) != len(unique) {
		return nil, items.InvalidField{Field: "approver_ids", Message: "approvers must be active users"}
	}
	for _, approver := range found {
		caps, err := policy.ItemCaps(ctx, tx, approver, itemID.String())
		if err != nil {
			return nil, err
		}
		// A file approval may name an external reviewer who sees only that version (FIL-04).
		if !caps.Has(policy.View) && !(file && approver.Type == policy.External) {
			return nil, items.InvalidField{Field: "approver_ids", Message: "approvers must be able to view the item"}
		}
	}
	return unique, nil
}

func (s *Server) writeApproval(w http.ResponseWriter, r *http.Request) {
	creating, deleting, deciding := r.Method == http.MethodPost, r.Method == http.MethodDelete, r.Method == http.MethodPut
	rawID := r.PathValue("approvalId")
	if creating {
		rawID = r.PathValue("itemId")
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	version := 0
	if r.Method == http.MethodPatch {
		var ok bool
		if version, ok = ifMatchVersion(w, r); !ok {
			return
		}
	}
	var body struct {
		Policy      *string         `json:"policy"`
		Description *string         `json:"description"`
		DueDate     json.RawMessage `json:"due_date"`
		ApproverIDs *[]uuid.UUID    `json:"approver_ids"`
		Decision    string          `json:"decision"`
		Comment     string          `json:"comment"`
		filePin
	}
	if !deleting && !decode(w, r, &body) {
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	batch := s.notificationBatch()
	var result items.Detail
	var itemID uuid.UUID
	switch {
	case creating:
		itemID = id
		result, err = createApproval(ctx, tx, actorOf(r), id, body.Description, body.DueDate, body.ApproverIDs, body.Policy, body.filePin, batch)
	default:
		if err = tx.QueryRow(ctx, "SELECT item_id FROM approvals WHERE id=$1", id).Scan(&itemID); errors.Is(err, pgx.ErrNoRows) {
			err = items.ErrNotFound
		}
		if err == nil && deleting {
			err = deleteApproval(ctx, tx, actorOf(r), id, itemID)
		} else if err == nil && deciding {
			result, err = decideApproval(ctx, tx, actorOf(r), id, itemID, body.Decision, body.Comment, batch)
		} else if err == nil {
			result, err = patchApproval(ctx, tx, actorOf(r), id, itemID, version, body.Description, body.DueDate, body.ApproverIDs, body.Policy, batch)
		}
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40001" {
			err = items.ErrConflict
		}
		writeItemError(w, err)
		return
	}
	s.publishNotifications(batch)
	s.publishItem(r, "item.updated", itemID.String(), itemID.String(), 0)
	if deleting {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	status := http.StatusOK
	if creating {
		status = http.StatusCreated
	}
	writeItem(w, status, result, nil)
}

func readApproval(ctx context.Context, tx pgx.Tx, id uuid.UUID) (items.Detail, error) {
	var out items.Detail
	err := tx.QueryRow(ctx, approvalSelect+"WHERE a.id=$1", id).Scan(&out.Version, &out.Body)
	return out, err
}

// recordApprovalEvent appends one history row, pinned to the item's current version.
func recordApprovalEvent(ctx context.Context, tx pgx.Tx, approval, itemID uuid.UUID, actorID, kind string, subjects []uuid.UUID, decision, comment string) error {
	if len(subjects) == 0 {
		subjects = []uuid.UUID{uuid.Nil}
	}
	_, err := tx.Exec(ctx, `INSERT INTO approval_events(approval_id,item_id,kind,actor_id,subject_id,decision,comment,item_version)
SELECT $1,$2,$3,$4::uuid,NULLIF(s,'00000000-0000-0000-0000-000000000000'::uuid),NULLIF($6,''),$7,(SELECT version FROM items WHERE id=$2)
FROM unnest($5::uuid[]) s`, approval, itemID, kind, actorID, subjects, decision, comment)
	return err
}

func addApprovers(ctx context.Context, tx pgx.Tx, actor policy.Actor, approval, itemID uuid.UUID, ids []uuid.UUID, snippet string, batch *notifications.Batch) error {
	if len(ids) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, "INSERT INTO approval_approvers(approval_id,user_id) SELECT $1,unnest($2::uuid[]) ON CONFLICT DO NOTHING", approval, ids); err != nil {
		return err
	}
	if err := recordApprovalEvent(ctx, tx, approval, itemID, actor.ID, "approver_requested", ids, "", ""); err != nil {
		return err
	}
	principals := make([]notifications.Principal, len(ids))
	for i, id := range ids {
		principals[i] = notifications.Principal{Kind: "user", ID: id.String()}
	}
	_, err := batch.Record(ctx, tx, actor, "approval_requested", notifications.Target{Item: itemID.String()}, snippet, principals, nil)
	return err
}

func validateApprovalPolicy(value string) error {
	if value != "any" && value != "all" {
		return items.InvalidField{Field: "policy", Message: "policy must be any or all"}
	}
	return nil
}

func createApproval(ctx context.Context, tx pgx.Tx, actor policy.Actor, itemID uuid.UUID, description *string, due json.RawMessage, approvers *[]uuid.UUID, decisionPolicy *string, pin filePin, batch *notifications.Batch) (items.Detail, error) {
	if _, err := approvalCaps(ctx, tx, actor, itemID, policy.Edit); err != nil {
		return items.Detail{}, err
	}
	if err := pin.validate(ctx, tx, itemID); err != nil {
		return items.Detail{}, err
	}
	if decisionPolicy == nil {
		defaultPolicy := "all"
		decisionPolicy = &defaultPolicy
	}
	if err := validateApprovalPolicy(*decisionPolicy); err != nil {
		return items.Detail{}, err
	}
	if approvers == nil {
		return items.Detail{}, items.InvalidField{Field: "approver_ids", Message: "choose between 1 and 20 approvers"}
	}
	text := ""
	if description != nil {
		var err error
		if text, err = approvalText(*description, "description", 2000); err != nil {
			return items.Detail{}, err
		}
	}
	var date *string
	if len(due) > 0 {
		var err error
		if date, err = approvalDate(due); err != nil {
			return items.Detail{}, err
		}
	}
	ids, err := checkApprovers(ctx, tx, *approvers, itemID, pin.AttachmentID != nil)
	if err != nil {
		return items.Detail{}, err
	}
	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM approvals WHERE item_id=$1", itemID).Scan(&count); err != nil {
		return items.Detail{}, err
	}
	if count >= maxApprovalsPerItem {
		return items.Detail{}, items.Invalid("an item can have at most 50 approvals")
	}
	id, err := uuid.NewV7()
	if err != nil {
		return items.Detail{}, err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO approvals(id,item_id,description,due_date,created_by,policy,attachment_id,attachment_version) VALUES($1,$2,$3,$4::date,$5::uuid,$6,$7,$8)", id, itemID, text, date, actor.ID, *decisionPolicy, pin.AttachmentID, pin.Version); err != nil {
		return items.Detail{}, err
	}
	if err := addApprovers(ctx, tx, actor, id, itemID, ids, text, batch); err != nil {
		return items.Detail{}, err
	}
	return readApproval(ctx, tx, id)
}

func patchApproval(ctx context.Context, tx pgx.Tx, actor policy.Actor, id, itemID uuid.UUID, version int, description *string, due json.RawMessage, approvers *[]uuid.UUID, decisionPolicy *string, batch *notifications.Batch) (items.Detail, error) {
	if _, err := approvalCaps(ctx, tx, actor, itemID, policy.Edit); err != nil {
		return items.Detail{}, err
	}
	if description == nil && len(due) == 0 && approvers == nil && decisionPolicy == nil {
		return items.Detail{}, items.Invalid("at least one field is required")
	}
	var current int
	var text string
	if err := tx.QueryRow(ctx, "SELECT version,description FROM approvals WHERE id=$1 FOR UPDATE", id).Scan(&current, &text); err != nil {
		return items.Detail{}, items.ErrNotFound
	}
	if current != version {
		return items.Detail{}, items.ErrConflict
	}
	if decisionPolicy != nil {
		if err := validateApprovalPolicy(*decisionPolicy); err != nil {
			return items.Detail{}, err
		}
		if _, err := tx.Exec(ctx, "UPDATE approvals SET policy=$2 WHERE id=$1", id, *decisionPolicy); err != nil {
			return items.Detail{}, err
		}
	}
	if description != nil {
		var err error
		if text, err = approvalText(*description, "description", 2000); err != nil {
			return items.Detail{}, err
		}
		if _, err := tx.Exec(ctx, "UPDATE approvals SET description=$2 WHERE id=$1", id, text); err != nil {
			return items.Detail{}, err
		}
	}
	if len(due) > 0 {
		date, err := approvalDate(due)
		if err != nil {
			return items.Detail{}, err
		}
		if _, err := tx.Exec(ctx, "UPDATE approvals SET due_date=$2::date, deadline_stage=CASE WHEN due_date IS DISTINCT FROM $2::date THEN 0 ELSE deadline_stage END WHERE id=$1", id, date); err != nil {
			return items.Detail{}, err
		}
	}
	if approvers != nil {
		var file bool
		if err := tx.QueryRow(ctx, "SELECT attachment_id IS NOT NULL FROM approvals WHERE id=$1", id).Scan(&file); err != nil {
			return items.Detail{}, err
		}
		ids, err := checkApprovers(ctx, tx, *approvers, itemID, file)
		if err != nil {
			return items.Detail{}, err
		}
		rows, err := tx.Query(ctx, "SELECT user_id FROM approval_approvers WHERE approval_id=$1", id)
		if err != nil {
			return items.Detail{}, err
		}
		existing, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return items.Detail{}, err
		}
		if _, err := tx.Exec(ctx, "DELETE FROM approval_approvers WHERE approval_id=$1 AND user_id<>ALL($2::uuid[])", id, ids); err != nil {
			return items.Detail{}, err
		}
		removed := slices.DeleteFunc(slices.Clone(existing), func(u uuid.UUID) bool { return slices.Contains(ids, u) })
		if len(removed) > 0 {
			if err := recordApprovalEvent(ctx, tx, id, itemID, actor.ID, "approver_removed", removed, "", ""); err != nil {
				return items.Detail{}, err
			}
		}
		added := slices.DeleteFunc(slices.Clone(ids), func(u uuid.UUID) bool { return slices.Contains(existing, u) })
		if err := addApprovers(ctx, tx, actor, id, itemID, added, text, batch); err != nil {
			return items.Detail{}, err
		}
	}
	if _, err := tx.Exec(ctx, "UPDATE approvals SET version=version+1 WHERE id=$1", id); err != nil {
		return items.Detail{}, err
	}
	return readApproval(ctx, tx, id)
}

func deleteApproval(ctx context.Context, tx pgx.Tx, actor policy.Actor, id, itemID uuid.UUID) error {
	if _, err := approvalCaps(ctx, tx, actor, itemID, policy.Edit); err != nil {
		return err
	}
	if err := recordApprovalEvent(ctx, tx, id, itemID, actor.ID, "withdrawn", nil, "", ""); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "DELETE FROM approvals WHERE id=$1", id)
	return err
}

func decideApproval(ctx context.Context, tx pgx.Tx, actor policy.Actor, id, itemID uuid.UUID, decision, comment string, batch *notifications.Batch) (items.Detail, error) {
	if _, err := approvalCaps(ctx, tx, actor, itemID, policy.View); err != nil && !(errors.Is(err, items.ErrNotFound) && approvalReviewer(ctx, tx, actor, id)) {
		return items.Detail{}, err
	}
	if decision != "approved" && decision != "rejected" {
		return items.Detail{}, items.InvalidField{Field: "decision", Message: "decision must be approved or rejected"}
	}
	comment, err := approvalText(comment, "comment", 2000)
	if err != nil {
		return items.Detail{}, err
	}
	var creator string
	if err := tx.QueryRow(ctx, "SELECT created_by::text FROM approvals WHERE id=$1 FOR UPDATE", id).Scan(&creator); err != nil {
		return items.Detail{}, items.ErrNotFound
	}
	tag, err := tx.Exec(ctx, "UPDATE approval_approvers SET decision=$3,comment=$4,decided_at=now() WHERE approval_id=$1 AND user_id=$2::uuid", id, actor.ID, decision, comment)
	if err != nil {
		return items.Detail{}, err
	}
	if tag.RowsAffected() == 0 {
		return items.Detail{}, items.ErrForbidden
	}
	if err := recordApprovalEvent(ctx, tx, id, itemID, actor.ID, "decided", []uuid.UUID{uuid.MustParse(actor.ID)}, decision, comment); err != nil {
		return items.Detail{}, err
	}
	if _, err := tx.Exec(ctx, "UPDATE approvals SET version=version+1 WHERE id=$1", id); err != nil {
		return items.Detail{}, err
	}
	snippet := decision
	if comment != "" {
		snippet += ": " + comment
	}
	if _, err := batch.Record(ctx, tx, actor, "approval_decided", notifications.Target{Item: itemID.String()}, snippet, []notifications.Principal{{Kind: "user", ID: creator}}, nil); err != nil {
		return items.Detail{}, err
	}
	if err := events.Emit(ctx, tx, events.Event{Type: events.ApprovalDecided, ItemID: itemID.String(), ActorID: actor.ID, Payload: map[string]any{"approval_id": id.String(), "decision": decision}}); err != nil {
		return items.Detail{}, err
	}
	if err := applyApprovalOutcome(ctx, tx, actor, id, itemID); err != nil {
		return items.Detail{}, err
	}
	return readApproval(ctx, tx, id)
}
