package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Server) ProjectFileRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/items/{projectId}/files", s.requireUser(s.listProjectFiles))
}

// listProjectFiles lists live attachments of a project and its descendants, only for items the caller can view.
// shortcut: newest 5000 candidates before the policy filter, add keyset paging when a project exceeds that.
func (s *Server) listProjectFiles(w http.ResponseWriter, r *http.Request) {
	project, err := uuid.Parse(r.PathValue("projectId"))
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
	actor := actorOf(r)
	if _, err := approvalCaps(ctx, tx, actor, project, policy.View); err != nil {
		writeItemError(w, err)
		return
	}
	rows, err := tx.Query(ctx, `WITH RECURSIVE t(id,depth) AS (
 SELECT $1::uuid,0
 UNION SELECT l.item_id,t.depth+1 FROM t JOIN item_locations l ON l.parent_item_id=t.id AND l.deleted_root_id IS NULL
  JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL WHERE t.depth<30)
 , outcomes(attachment_id,attachment_version,created_at,outcome) AS (
  -- WA-D08: one outcome per approval pinned to a file version, using the same rule as the approval list.
  SELECT a.attachment_id,a.attachment_version,a.created_at,(`+approvalOutcome+`)
  FROM approvals a JOIN t ON t.id=a.item_id WHERE a.attachment_id IS NOT NULL)
 SELECT a.item_id::text, jsonb_build_object('id',a.id,'item_id',a.item_id,'item_title',i.title,'filename',a.filename,
 'content_type',a.content_type,'size_bytes',a.size_bytes,'created_at',a.created_at,
 'version',a.current_version,
 'review_status',CASE WHEN r.outcome IS NULL THEN
   CASE WHEN EXISTS(SELECT 1 FROM approval_events e WHERE e.kind='withdrawn' AND e.attachment_id=a.id AND e.attachment_version=a.current_version) THEN 'cancelled' ELSE 'not_requested' END
  WHEN r.outcome='pending' THEN 'in_review' ELSE r.outcome END,
 'uploader',jsonb_build_object('id',u.id,'name',concat_ws(' ',u.first_name,u.last_name)))
 FROM attachments a JOIN (SELECT DISTINCT id FROM t) t ON t.id=a.item_id
 JOIN items i ON i.id=a.item_id AND i.deleted_at IS NULL JOIN users u ON u.id=a.uploader_id
 LEFT JOIN LATERAL (SELECT o.outcome FROM outcomes o WHERE o.attachment_id=a.id AND o.attachment_version=a.current_version
   ORDER BY (o.outcome='pending') DESC, o.created_at DESC LIMIT 1) r ON TRUE
 WHERE a.deleted_at IS NULL ORDER BY a.created_at DESC,a.id LIMIT 5000`, project)
	if err != nil {
		writeItemError(w, err)
		return
	}
	type row struct {
		ItemID string
		Body   json.RawMessage
	}
	all, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
	if err != nil {
		writeItemError(w, err)
		return
	}
	ids := make([]string, len(all))
	for i := range all {
		ids[i] = all[i].ItemID
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		writeItemError(w, err)
		return
	}
	out := []json.RawMessage{}
	for _, f := range all {
		if caps[f.ItemID].Has(policy.View) {
			out = append(out, f.Body)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}
