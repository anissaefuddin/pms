package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/views"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// WA-D05: limits on the per-user store. Rows beyond the limit are refused rather than silently dropped.
const (
	viewExpansionMaxRows  = 500
	viewExpansionMaxViews = 200
)

// saveViewExpansion stores the caller's own expanded rows and Gantt collapse flag for one view. The view must exist and be
// visible to the caller (404 otherwise), so the store never keeps ids that point at nothing.
func (s *Server) saveViewExpansion(w http.ResponseWriter, r *http.Request) {
	viewID := r.PathValue("viewId")
	if _, err := uuid.Parse(viewID); err != nil {
		writeError(w, 422, "invalid view id")
		return
	}
	var input struct {
		Rows           []string `json:"rows"`
		GanttCollapsed bool     `json:"gantt_collapsed"`
	}
	if !decode(w, r, &input) {
		return
	}
	if len(input.Rows) > viewExpansionMaxRows {
		writeError(w, 422, "too many expanded rows")
		return
	}
	seen := make(map[string]bool, len(input.Rows))
	rows := make([]string, 0, len(input.Rows))
	for _, id := range input.Rows {
		if _, err := uuid.Parse(id); err != nil {
			writeError(w, 422, "invalid row id")
			return
		}
		if !seen[id] {
			seen[id] = true
			rows = append(rows, id)
		}
	}
	value, err := json.Marshal(map[string]any{"rows": rows, "gantt_collapsed": input.GanttCollapsed})
	if err != nil {
		writeError(w, 500, "could not save view expansion")
		return
	}
	s.viewTx(w, r, http.StatusNoContent, func(tx pgx.Tx) (any, error) {
		if err := views.Visible(r.Context(), tx, actorOf(r), uuid.MustParse(viewID)); err != nil {
			return nil, err
		}
		// A view already in the store is always updated; a new view needs room under the limit.
		tag, err := tx.Exec(r.Context(),
			`UPDATE users SET view_expansion = jsonb_set(view_expansion, ARRAY[$2::text], $3::jsonb, true)
			 WHERE id=$1::uuid AND status='active'
			   AND (view_expansion ? $2::text OR (SELECT count(*) FROM jsonb_object_keys(view_expansion)) < $4)`,
			actorOf(r).ID, viewID, value, viewExpansionMaxViews)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() != 1 {
			return nil, views.Invalid("too many views remembered; expand rows in an existing view or ask an admin to reset")
		}
		return nil, nil
	})
}
