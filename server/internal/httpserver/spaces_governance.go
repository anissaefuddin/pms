package httpserver

import (
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/spaces"
)

// Space governance routes: archive/restore (D-03), explicit admins and ownership transfer (D-SA1), Public discovery and
// join (D-01), and the icon catalogue (D-04). Every decision is made in package spaces, which uses package policy.
func (s *Server) spaceGovernanceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/archive", s.requireUser(s.archiveSpace))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/restore", s.requireUser(s.restoreSpace))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/admins", s.requireUser(s.listSpaceAdmins))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/admins", s.requireUser(s.grantSpaceAdmin))
	mux.HandleFunc("DELETE /api/v1/spaces/{spaceId}/admins/{userId}", s.requireUser(s.revokeSpaceAdmin))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/transfer-ownership", s.requireUser(s.transferSpaceOwnership))
	mux.HandleFunc("GET /api/v1/spaces/discover", s.requireUser(s.discoverSpaces))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/join", s.requireUser(s.joinSpace))
	mux.HandleFunc("GET /api/v1/space-icons", s.requireUser(s.listSpaceIcons))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/default-workflows", s.requireUser(s.getSpaceDefaultWorkflows))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/delete-impact", s.requireUser(s.getSpaceDeleteImpact))
	mux.HandleFunc("PUT /api/v1/spaces/{spaceId}/default-workflows", s.requireUser(s.setSpaceDefaultWorkflows))
}

func (s *Server) archiveSpace(w http.ResponseWriter, r *http.Request) { s.setSpaceArchived(w, r, true) }

func (s *Server) restoreSpace(w http.ResponseWriter, r *http.Request) {
	s.setSpaceArchived(w, r, false)
}

func (s *Server) setSpaceArchived(w http.ResponseWriter, r *http.Request, archive bool) {
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	space, warnings, err := s.spaces.SetArchived(r.Context(), actorOf(r), r.PathValue("spaceId"), version, archive)
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	action := auditSpaceArchived
	if !archive {
		action = auditSpaceRestored
	}
	s.audit(r, actorOf(r).ID, "", auditEvent{Action: action, TargetType: "space", TargetID: space.ID})
	setETag(w, space.Version)
	if warnings == nil {
		warnings = []string{}
	}
	writeJSON(w, 200, map[string]any{"space": space, "warnings": warnings})
}

func (s *Server) listSpaceAdmins(w http.ResponseWriter, r *http.Request) {
	list, err := s.spaces.ListAdmins(r.Context(), actorOf(r), r.PathValue("spaceId"))
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": list})
}

func (s *Server) grantSpaceAdmin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		UserID string `json:"user_id"`
	}
	if !decode(w, r, &input) {
		return
	}
	actor := actorOf(r)
	spaceID := r.PathValue("spaceId")
	if err := s.spaces.GrantAdmin(r.Context(), actor, spaceID, input.UserID); err != nil {
		writeSpaceError(w, err)
		return
	}
	s.audit(r, actor.ID, "", auditEvent{Action: auditSpaceAdminGranted, TargetType: "space", TargetID: spaceID, Details: map[string]any{"user_id": input.UserID}})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) revokeSpaceAdmin(w http.ResponseWriter, r *http.Request) {
	actor := actorOf(r)
	spaceID := r.PathValue("spaceId")
	userID := r.PathValue("userId")
	if err := s.spaces.RevokeAdmin(r.Context(), actor, spaceID, userID); err != nil {
		writeSpaceError(w, err)
		return
	}
	s.audit(r, actor.ID, "", auditEvent{Action: auditSpaceAdminRemoved, TargetType: "space", TargetID: spaceID, Details: map[string]any{"user_id": userID}})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) transferSpaceOwnership(w http.ResponseWriter, r *http.Request) {
	var input struct {
		UserID string `json:"user_id"`
	}
	if !decode(w, r, &input) {
		return
	}
	actor := actorOf(r)
	spaceID := r.PathValue("spaceId")
	if err := s.spaces.TransferOwnership(r.Context(), actor, spaceID, input.UserID); err != nil {
		writeSpaceError(w, err)
		return
	}
	s.audit(r, actor.ID, "", auditEvent{Action: auditSpaceOwnership, TargetType: "space", TargetID: spaceID, Details: map[string]any{"to": input.UserID}})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) discoverSpaces(w http.ResponseWriter, r *http.Request) {
	list, err := s.spaces.Discover(r.Context(), actorOf(r), r.URL.Query().Get("q"))
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": list})
}

func (s *Server) joinSpace(w http.ResponseWriter, r *http.Request) {
	actor := actorOf(r)
	spaceID := r.PathValue("spaceId")
	space, err := s.spaces.Join(r.Context(), actor, spaceID)
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	s.audit(r, actor.ID, "", auditEvent{Action: auditSpaceJoined, TargetType: "space", TargetID: spaceID})
	setETag(w, space.Version)
	writeJSON(w, 200, space)
}

func (s *Server) listSpaceIcons(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"items": spaces.IconKeys})
}

// getSpaceDefaultWorkflows returns the Space's default workflow for Tasks and Projects (B).
func (s *Server) getSpaceDefaultWorkflows(w http.ResponseWriter, r *http.Request) {
	out, err := s.spaces.GetDefaultWorkflows(r.Context(), actorOf(r), r.PathValue("spaceId"))
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// setSpaceDefaultWorkflows replaces the Space's defaults. Existing items keep their workflow and status.
func (s *Server) setSpaceDefaultWorkflows(w http.ResponseWriter, r *http.Request) {
	var input spaces.DefaultWorkflows
	if !decode(w, r, &input) {
		return
	}
	out, err := s.spaces.SetDefaultWorkflows(r.Context(), actorOf(r), r.PathValue("spaceId"), input)
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// getSpaceDeleteImpact previews what deleting the Space would affect (D). It changes nothing.
func (s *Server) getSpaceDeleteImpact(w http.ResponseWriter, r *http.Request) {
	out, err := s.spaces.GetDeleteImpact(r.Context(), actorOf(r), r.PathValue("spaceId"))
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
