package httpserver

import (
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/items"
)

func (s *Server) customFieldRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/custom-fields", s.requireUser(s.listCustomFields))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/custom-fields", s.requireUser(s.createCustomField))
	mux.HandleFunc("PATCH /api/v1/spaces/{spaceId}/custom-fields/{fieldId}", s.requireUser(s.updateCustomField))
	mux.HandleFunc("DELETE /api/v1/spaces/{spaceId}/custom-fields/{fieldId}", s.requireUser(s.archiveCustomField))
}

func (s *Server) listCustomFields(w http.ResponseWriter, r *http.Request) {
	fields, err := items.ListCustomFields(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": fields})
}

func (s *Server) createCustomField(w http.ResponseWriter, r *http.Request) {
	var input items.CustomFieldCreate
	if !decode(w, r, &input) {
		return
	}
	field, err := items.CreateCustomField(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"), input)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, field)
}

func (s *Server) updateCustomField(w http.ResponseWriter, r *http.Request) {
	var input items.CustomFieldUpdate
	if !decode(w, r, &input) {
		return
	}
	field, err := items.UpdateCustomField(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"), r.PathValue("fieldId"), input)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, field)
}

func (s *Server) archiveCustomField(w http.ResponseWriter, r *http.Request) {
	if err := items.ArchiveCustomField(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"), r.PathValue("fieldId")); err != nil {
		writeItemError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
