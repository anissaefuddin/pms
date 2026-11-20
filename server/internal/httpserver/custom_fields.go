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
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/custom-fields/{fieldId}/impact", s.requireUser(s.customFieldImpact))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/custom-fields/{fieldId}/restore", s.requireUser(s.restoreCustomField))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/custom-fields/{fieldId}/grants", s.requireUser(s.listFieldGrants))
	mux.HandleFunc("PUT /api/v1/spaces/{spaceId}/custom-fields/{fieldId}/grants", s.requireUser(s.setFieldGrants))
	// Workspace custom fields: the same handlers with no Space, which the items package reads as the Workspace (admins only).
	mux.HandleFunc("GET /api/v1/custom-fields", s.requireUser(s.listCustomFields))
	mux.HandleFunc("POST /api/v1/custom-fields", s.requireUser(s.createCustomField))
	mux.HandleFunc("PATCH /api/v1/custom-fields/{fieldId}", s.requireUser(s.updateCustomField))
	mux.HandleFunc("DELETE /api/v1/custom-fields/{fieldId}", s.requireUser(s.archiveCustomField))
	mux.HandleFunc("GET /api/v1/custom-fields/{fieldId}/impact", s.requireUser(s.customFieldImpact))
	mux.HandleFunc("POST /api/v1/custom-fields/{fieldId}/restore", s.requireUser(s.restoreCustomField))
	mux.HandleFunc("GET /api/v1/custom-fields/{fieldId}/grants", s.requireUser(s.listFieldGrants))
	mux.HandleFunc("PUT /api/v1/custom-fields/{fieldId}/grants", s.requireUser(s.setFieldGrants))
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

func (s *Server) customFieldImpact(w http.ResponseWriter, r *http.Request) {
	impact, err := items.CustomFieldImpactOf(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"), r.PathValue("fieldId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, impact)
}

func (s *Server) restoreCustomField(w http.ResponseWriter, r *http.Request) {
	field, err := items.RestoreCustomField(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"), r.PathValue("fieldId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, field)
}

func (s *Server) listFieldGrants(w http.ResponseWriter, r *http.Request) {
	grants, err := items.ListFieldGrants(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"), r.PathValue("fieldId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": grants})
}

// setFieldGrants replaces the whole grant list. An empty list lifts the restriction.
func (s *Server) setFieldGrants(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Grants []items.FieldGrant `json:"grants"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Grants == nil {
		in.Grants = []items.FieldGrant{}
	}
	actor := actorOf(r)
	if err := items.SetFieldGrants(r.Context(), s.pool, actor, r.PathValue("spaceId"), r.PathValue("fieldId"), in.Grants); err != nil {
		writeItemError(w, err)
		return
	}
	s.audit(r, actor.ID, "", auditEvent{Action: auditCustomFieldGrants, TargetType: "custom_field", TargetID: r.PathValue("fieldId"),
		Details: map[string]any{"grants": len(in.Grants)}})
	writeJSON(w, http.StatusOK, map[string]any{"items": in.Grants})
}
