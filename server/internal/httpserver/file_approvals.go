package httpserver

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// filePin is the file version an approval is bound to (FIL-04).
type filePin struct {
	AttachmentID *uuid.UUID `json:"attachment_id"`
	Version      *int       `json:"attachment_version"`
}

func (s *Server) fileApprovalRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/file-reviews", s.requireUser(s.listFileReviews))
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// validatePin requires a live attachment of this item with that version; it returns nil for a plain item approval.
func (p filePin) validate(ctx context.Context, tx pgx.Tx, itemID uuid.UUID) error {
	if p.AttachmentID == nil && p.Version == nil {
		return nil
	}
	if p.AttachmentID == nil || p.Version == nil {
		return items.InvalidField{Field: "attachment_version", Message: "attachment_id and attachment_version go together"}
	}
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM attachments a JOIN attachment_versions v ON v.attachment_id=a.id
 WHERE a.id=$1 AND a.item_id=$2 AND a.deleted_at IS NULL AND v.version=$3)`, *p.AttachmentID, itemID, *p.Version).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return items.InvalidField{Field: "attachment_id", Message: "file version not found on this item"}
	}
	return nil
}

// fileReviewer: an external user named as approver on an approval pinned to exactly this file version may read that version.
func fileReviewer(ctx context.Context, db rowQuerier, actor policy.Actor, attachmentID uuid.UUID, version int) bool {
	var ok bool
	err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM approvals a JOIN approval_approvers x ON x.approval_id=a.id
 WHERE a.attachment_id=$1 AND a.attachment_version=$2 AND x.user_id=$3::uuid)`, attachmentID, version, actor.ID).Scan(&ok)
	return err == nil && ok && actor.Type == policy.External
}

// approvalReviewer is the same grant, addressed by approval id (used to decide without item access).
func approvalReviewer(ctx context.Context, db rowQuerier, actor policy.Actor, approvalID uuid.UUID) bool {
	var ok bool
	err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM approvals a JOIN approval_approvers x ON x.approval_id=a.id
 WHERE a.id=$1 AND a.attachment_id IS NOT NULL AND x.user_id=$2::uuid)`, approvalID, actor.ID).Scan(&ok)
	return err == nil && ok && actor.Type == policy.External
}

// listFileReviews lists the file approvals the caller is asked to decide, without naming the item (reviewers may not see it).
func (s *Server) listFileReviews(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT jsonb_build_object('approval_id',a.id,'attachment_id',a.attachment_id,'version',a.attachment_version,
 'filename',v.filename,'size_bytes',v.size_bytes,'description',a.description,'due_date',a.due_date,'decision',x.decision,'comment',x.comment,
 'requested_by',concat_ws(' ',cu.first_name,cu.last_name))
FROM approval_approvers x JOIN approvals a ON a.id=x.approval_id JOIN users cu ON cu.id=a.created_by
JOIN attachment_versions v ON v.attachment_id=a.attachment_id AND v.version=a.attachment_version
JOIN attachments t ON t.id=a.attachment_id AND t.deleted_at IS NULL
WHERE x.user_id=$1::uuid ORDER BY (x.decision IS NOT NULL), a.created_at DESC LIMIT 200`, actorOf(r).ID)
	if err == nil {
		var out []json.RawMessage
		if out, err = pgx.CollectRows(rows, pgx.RowTo[json.RawMessage]); err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"items": out})
			return
		}
	}
	writeItemError(w, err)
}
