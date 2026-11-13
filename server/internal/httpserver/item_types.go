package httpserver

import (
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
)

func (s *Server) listItemTypes(w http.ResponseWriter, r *http.Request) {
	list, err := items.ListItemTypes(r.Context(), s.pool)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (s *Server) createItemType(w http.ResponseWriter, r *http.Request) {
	var in items.ItemTypeInput
	if !decode(w, r, &in) {
		return
	}
	t, err := items.CreateItemType(r.Context(), s.pool, actorOf(r), in)
	if err != nil {
		writeItemError(w, err)
		return
	}
	setETag(w, t.Version)
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) updateItemType(w http.ResponseWriter, r *http.Request) {
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	var in items.ItemTypeInput
	if !decode(w, r, &in) {
		return
	}
	t, err := items.UpdateItemType(r.Context(), s.pool, actorOf(r), r.PathValue("typeId"), version, in)
	if err != nil {
		writeItemError(w, err)
		return
	}
	setETag(w, t.Version)
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) listItemRelations(w http.ResponseWriter, r *http.Request) {
	id, ok := s.itemWithCap(w, r, policy.View)
	if !ok {
		return
	}
	groups, err := items.ItemRelations(r.Context(), s.pool, actorOf(r), id)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": groups})
}
