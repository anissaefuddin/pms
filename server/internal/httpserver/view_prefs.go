package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/views"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// WA-D07: limits on the per-user store. Beyond a limit the request is refused, never silently cut.
const (
	viewPrefsMaxViews  = 100 // views listed as hidden or ordered in one workspace
	viewPrefsMaxScopes = 200 // workspaces remembered per user
)

// saveViewPreferences stores the caller's own hidden views and tab order for one workspace. The shared view
// configuration (position, default, visibility) is never touched. The workspace must be visible to the caller (404 otherwise).
func (s *Server) saveViewPreferences(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeId")
	if _, err := uuid.Parse(scopeID); err != nil {
		writeError(w, 422, "invalid workspace id")
		return
	}
	var input struct {
		Hidden []string `json:"hidden"`
		Order  []string `json:"order"`
	}
	if !decode(w, r, &input) {
		return
	}
	if len(input.Hidden) > viewPrefsMaxViews || len(input.Order) > viewPrefsMaxViews {
		writeError(w, 422, "too many views in preferences")
		return
	}
	hidden, ok := uniqueUUIDs(input.Hidden)
	if !ok {
		writeError(w, 422, "invalid view id in hidden")
		return
	}
	order, ok := uniqueUUIDs(input.Order)
	if !ok {
		writeError(w, 422, "invalid view id in order")
		return
	}
	value, err := json.Marshal(map[string]any{"hidden": hidden, "order": order})
	if err != nil {
		writeError(w, 500, "could not save view preferences")
		return
	}
	s.viewTx(w, r, http.StatusNoContent, func(tx pgx.Tx) (any, error) {
		if err := views.WorkspaceVisible(r.Context(), tx, actorOf(r), uuid.MustParse(scopeID)); err != nil {
			return nil, err
		}
		// An existing workspace is always updated; a new one needs room under the limit.
		tag, err := tx.Exec(r.Context(),
			`UPDATE users SET view_prefs = jsonb_set(view_prefs, ARRAY[$2::text], $3::jsonb, true)
			 WHERE id=$1::uuid AND status='active'
			   AND (view_prefs ? $2::text OR (SELECT count(*) FROM jsonb_object_keys(view_prefs)) < $4)`,
			actorOf(r).ID, scopeID, value, viewPrefsMaxScopes)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() != 1 {
			return nil, views.Invalid("too many workspaces remembered")
		}
		return nil, nil
	})
}

// uniqueUUIDs keeps the order given, drops repeats, and reports false if any entry is not a UUID.
func uniqueUUIDs(ids []string) ([]string, bool) {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return nil, false
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, true
}
