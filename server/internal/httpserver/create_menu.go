package httpserver

import (
	"net/http"
	"slices"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

func (s *Server) createMenuRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/create-menu", s.requireUser(s.getCreateMenu))
	mux.HandleFunc("PUT /api/v1/spaces/{spaceId}/create-menu", s.requireUser(s.putCreateMenu))
}

type createMenuEntry struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Icon    string `json:"icon"`
	Base    string `json:"base"`
	Enabled bool   `json:"enabled"`
}

// createMenu lists the active item types in default order with the Space's toggles applied (SPC-14).
func createMenu(r *http.Request, db pgx.Tx, spaceID string) ([]createMenuEntry, int, error) {
	var hidden []string
	var version int
	if err := db.QueryRow(r.Context(), "SELECT create_menu_hidden, version FROM spaces WHERE id=$1 AND deleted_at IS NULL", spaceID).Scan(&hidden, &version); err != nil {
		return nil, 0, err
	}
	types, err := items.ListItemTypes(r.Context(), db)
	if err != nil {
		return nil, 0, err
	}
	out := []createMenuEntry{}
	for _, t := range types {
		if t.Active {
			out = append(out, createMenuEntry{t.Key, t.Name, t.Icon, t.Base, !slices.Contains(hidden, t.Key)})
		}
	}
	return out, version, nil
}

func (s *Server) getCreateMenu(w http.ResponseWriter, r *http.Request) {
	tx, spaceID, ok := s.bookmarkTx(w, r, policy.View)
	if !ok {
		return
	}
	defer tx.Rollback(r.Context())
	list, version, err := createMenu(r, tx, spaceID.String())
	if err != nil {
		writeItemError(w, err)
		return
	}
	setETag(w, version)
	writeJSON(w, http.StatusOK, map[string]any{"items": list, "version": version})
}

func (s *Server) putCreateMenu(w http.ResponseWriter, r *http.Request) {
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	var in struct {
		Enabled []string `json:"enabled"`
	}
	if !decode(w, r, &in) {
		return
	}
	tx, spaceID, ok := s.bookmarkTx(w, r, policy.Edit)
	if !ok {
		return
	}
	defer tx.Rollback(r.Context())
	list, _, err := createMenu(r, tx, spaceID.String())
	if err != nil {
		writeItemError(w, err)
		return
	}
	known := map[string]bool{}
	for _, e := range list {
		known[e.Key] = true
	}
	on := 0
	for _, k := range in.Enabled {
		if !known[k] {
			writeItemError(w, fieldErr("enabled", "unknown item type "+k))
			return
		}
		on++
	}
	if on == 0 {
		writeItemError(w, fieldErr("enabled", "keep at least one item type in the menu"))
		return
	}
	hidden := []string{}
	for _, e := range list {
		if !slices.Contains(in.Enabled, e.Key) {
			hidden = append(hidden, e.Key)
		}
	}
	var next int
	err = tx.QueryRow(r.Context(), "UPDATE spaces SET create_menu_hidden=$3, version=version+1 WHERE id=$1 AND version=$2 AND deleted_at IS NULL RETURNING version", spaceID, version, hidden).Scan(&next)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusConflict, "the Space was changed by someone else; reload and try again")
		return
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	for i := range list {
		list[i].Enabled = !slices.Contains(hidden, list[i].Key)
	}
	setETag(w, next)
	s.audit(r, actorOf(r).ID, "", auditEvent{Action: auditCreateMenu, TargetType: "space", TargetID: spaceID.String(), Details: map[string]any{"hidden": hidden}})
	writeJSON(w, http.StatusOK, map[string]any{"items": list, "version": next})
}
