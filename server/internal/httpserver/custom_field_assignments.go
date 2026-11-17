package httpserver

import (
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/items"
)

// Field assignments (CF-D02): a Space's Full role decides where fields apply in that Space.
func (s *Server) customFieldAssignmentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/custom-fields/{fieldId}/assignments", s.requireUser(s.listFieldAssignments))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/custom-fields/{fieldId}/assignments", s.requireUser(s.assignField))
	mux.HandleFunc("DELETE /api/v1/spaces/{spaceId}/custom-fields/{fieldId}/assignments/{scopeKind}/{scopeId}", s.requireUser(s.unassignField))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/field-choices", s.requireUser(s.listSpaceFieldChoices))
}

func (s *Server) listFieldAssignments(w http.ResponseWriter, r *http.Request) {
	list, err := items.ListFieldAssignments(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"), r.PathValue("fieldId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (s *Server) assignField(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ScopeKind string `json:"scope_kind"`
		ScopeID   string `json:"scope_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	assignment, err := items.AssignField(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"), r.PathValue("fieldId"), in.ScopeKind, in.ScopeID)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, assignment)
}

func (s *Server) unassignField(w http.ResponseWriter, r *http.Request) {
	err := items.UnassignField(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"), r.PathValue("fieldId"), r.PathValue("scopeKind"), r.PathValue("scopeId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listSpaceFieldChoices(w http.ResponseWriter, r *http.Request) {
	list, err := items.ListSpaceFieldChoices(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}
