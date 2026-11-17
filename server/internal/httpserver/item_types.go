package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
)

func (s *Server) listItemTypes(w http.ResponseWriter, r *http.Request) {
	list, err := items.ListItemTypes(r.Context(), s.pool, actorOf(r))
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

func (s *Server) listTypeFields(w http.ResponseWriter, r *http.Request) {
	list, err := items.ListTypeFields(r.Context(), s.pool, actorOf(r), r.PathValue("typeId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (s *Server) setTypeField(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Default json.RawMessage `json:"default"`
	}
	if !decode(w, r, &in) {
		return
	}
	t, err := items.SetTypeField(r.Context(), s.pool, actorOf(r), r.PathValue("typeId"), r.PathValue("fieldId"), in.Default)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) deleteTypeField(w http.ResponseWriter, r *http.Request) {
	if err := items.DeleteTypeField(r.Context(), s.pool, actorOf(r), r.PathValue("typeId"), r.PathValue("fieldId")); err != nil {
		writeItemError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) itemTypeUsage(w http.ResponseWriter, r *http.Request) {
	usage, err := items.ItemTypeUsage(r.Context(), s.pool, actorOf(r))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": usage})
}
