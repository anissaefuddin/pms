package httpserver

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/spaces"
)

func (s *Server) spaceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/spaces", s.requireUser(s.listSpaces))
	mux.HandleFunc("POST /api/v1/spaces", s.requireUser(s.createSpace))
	mux.HandleFunc("GET /api/v1/space-templates", s.requireUser(s.listSpaceTemplates))
	mux.HandleFunc("POST /api/v1/space-templates", s.requireUser(s.saveSpaceTemplate))
	mux.HandleFunc("DELETE /api/v1/space-templates/{templateId}", s.requireUser(s.deleteSpaceTemplate))
	mux.HandleFunc("GET /api/v1/item-types", s.requireUser(s.listItemTypes))
	mux.HandleFunc("POST /api/v1/item-types", s.requireUser(s.createItemType))
	mux.HandleFunc("PATCH /api/v1/item-types/{typeId}", s.requireUser(s.updateItemType))
	mux.HandleFunc("GET /api/v1/item-types/usage", s.requireUser(s.itemTypeUsage))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/item-types/usage", s.requireUser(s.spaceItemTypeUsage))
	mux.HandleFunc("GET /api/v1/item-types/{typeId}/fields", s.requireUser(s.listTypeFields))
	mux.HandleFunc("PUT /api/v1/item-types/{typeId}/fields/{fieldId}", s.requireUser(s.setTypeField))
	mux.HandleFunc("DELETE /api/v1/item-types/{typeId}/fields/{fieldId}", s.requireUser(s.deleteTypeField))
	mux.HandleFunc("GET /api/v1/items/{itemId}/activity", s.requireUser(s.listItemActivity))
	mux.HandleFunc("GET /api/v1/items/{itemId}/relations", s.requireUser(s.listItemRelations))
	mux.HandleFunc("PUT /api/v1/items/{itemId}/followers", s.requireUser(s.replaceItemFollowers))
	mux.HandleFunc("POST /api/v1/space-templates/{templateId}/spaces", s.requireUser(s.createSpaceFromTemplate))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}", s.requireUser(s.getSpace))
	mux.HandleFunc("PATCH /api/v1/spaces/{spaceId}", s.requireUser(s.updateSpace))
	mux.HandleFunc("DELETE /api/v1/spaces/{spaceId}", s.requireUser(s.deleteSpace))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/shares", s.requireUser(s.listSpaceShares))
	mux.HandleFunc("PUT /api/v1/spaces/{spaceId}/shares", s.requireUser(s.replaceSpaceShares))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/members", s.requireUser(s.listSpaceMembers))
	mux.HandleFunc("GET /api/v1/spaces/registry", s.requireUser(s.listSpaceRegistry))
	mux.HandleFunc("GET /api/v1/overview", s.requireUser(s.getOverview))
	mux.HandleFunc("GET /api/v1/overview/timeline", s.requireUser(s.getOverviewTimeline))
	mux.HandleFunc("GET /api/v1/overview/tasks", s.requireUser(s.getOverviewTasks))
	mux.HandleFunc("GET /api/v1/overview/agenda", s.requireUser(s.getOverviewAgenda))
	s.spaceGovernanceRoutes(mux)
	mux.HandleFunc("GET /api/v1/users", s.requireUser(s.listUsers))
	mux.HandleFunc("GET /api/v1/groups", s.requireUser(s.listGroups))
}

func actorOf(r *http.Request) policy.Actor {
	return policy.Actor{ID: r.Context().Value(userKey).(string), Type: policy.UserType(r.Context().Value(userTypeKey).(string))}
}

// writeSpaceError maps service errors to the contract: no access = 404, visible but not allowed = 403,
// stale If-Match = 409, bad input = 422. Anything else is a 500 and is logged, never shown.
func writeSpaceError(w http.ResponseWriter, err error) {
	var invalid spaces.ValidationError
	switch {
	case errors.Is(err, spaces.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, spaces.ErrForbidden):
		writeError(w, http.StatusForbidden, strings.TrimPrefix(err.Error(), "forbidden: "))
	case errors.Is(err, spaces.ErrVersionConflict):
		writeError(w, http.StatusConflict, "the Space was changed by someone else; reload and try again")
	case errors.As(err, &invalid):
		writeError(w, http.StatusUnprocessableEntity, invalid.Message)
	default:
		slog.ErrorContext(logContext(w), "spaces", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func setETag(w http.ResponseWriter, version int) {
	w.Header().Set("ETag", `"`+strconv.Itoa(version)+`"`)
}

func (s *Server) listSpaces(w http.ResponseWriter, r *http.Request) {
	list, err := s.spaces.List(r.Context(), actorOf(r))
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": list})
}

func (s *Server) getSpace(w http.ResponseWriter, r *http.Request) {
	space, err := s.spaces.Get(r.Context(), actorOf(r), r.PathValue("spaceId"))
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	setETag(w, space.Version)
	writeJSON(w, 200, space)
}

func (s *Server) createSpace(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string              `json:"name"`
		Description string              `json:"description"`
		Shares      []spaces.ShareInput `json:"shares"`
	}
	if !decode(w, r, &input) {
		return
	}
	batch := s.notificationBatch()
	space, err := s.spaces.Create(r.Context(), actorOf(r), spaces.CreateInput{Name: input.Name, Description: input.Description, Shares: input.Shares}, batch)
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	s.publishNotifications(batch)
	setETag(w, space.Version)
	writeJSON(w, http.StatusCreated, space)
}

// ifMatchVersion reads the quoted version from If-Match: missing = 428, malformed = 422.
func ifMatchVersion(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		writeError(w, http.StatusPreconditionRequired, "send If-Match with the version you edited")
		return 0, false
	}
	n, err := strconv.Atoi(strings.Trim(strings.TrimPrefix(raw, "W/"), `"`))
	if err != nil || n < 1 {
		writeError(w, http.StatusUnprocessableEntity, "If-Match must be a quoted version number such as \"3\"")
		return 0, false
	}
	if n > math.MaxInt32 { // versions are int4: a larger number can only be stale
		writeError(w, http.StatusConflict, "the Space was changed by someone else; reload and try again")
		return 0, false
	}
	return n, true
}

func (s *Server) updateSpace(w http.ResponseWriter, r *http.Request) {
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var patch spaces.Patch
	for key, raw := range fields {
		switch key {
		case "name":
			var name string
			if err := json.Unmarshal(raw, &name); err != nil {
				writeError(w, http.StatusUnprocessableEntity, "name must be a string")
				return
			}
			patch.Name = &name
		case "description":
			patch.DescriptionSet = true
			if string(raw) != "null" {
				if err := json.Unmarshal(raw, &patch.Description); err != nil {
					writeError(w, http.StatusUnprocessableEntity, "description must be a string or null")
					return
				}
			}
		case "visibility":
			var visibility string
			if err := json.Unmarshal(raw, &visibility); err != nil {
				writeError(w, http.StatusUnprocessableEntity, "visibility must be public or private")
				return
			}
			patch.Visibility = &visibility
		case "icon_key":
			patch.IconKeySet = true
			if string(raw) != "null" {
				if err := json.Unmarshal(raw, &patch.IconKey); err != nil {
					writeError(w, http.StatusUnprocessableEntity, "icon_key must be a string or null")
					return
				}
			}
		case "locked":
			var locked bool
			if err := json.Unmarshal(raw, &locked); err != nil {
				writeError(w, http.StatusUnprocessableEntity, "locked must be true or false")
				return
			}
			patch.Locked = &locked
		default:
			writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("unknown field %q", key))
			return
		}
	}
	space, err := s.spaces.Update(r.Context(), actorOf(r), r.PathValue("spaceId"), version, patch)
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	if patch.Visibility != nil {
		s.audit(r, actorOf(r).ID, "", auditEvent{Action: auditSpaceVisibility, TargetType: "space", TargetID: space.ID,
			Details: map[string]any{"to": space.Visibility}})
	}
	if patch.Locked != nil {
		action := auditSpaceUnlocked
		if *patch.Locked {
			action = auditSpaceLocked
		}
		s.audit(r, actorOf(r).ID, "", auditEvent{Action: action, TargetType: "space", TargetID: space.ID})
	}
	setETag(w, space.Version)
	writeJSON(w, 200, space)
}

func (s *Server) deleteSpace(w http.ResponseWriter, r *http.Request) {
	if err := s.spaces.Delete(r.Context(), actorOf(r), r.PathValue("spaceId")); err != nil {
		writeSpaceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listSpaceShares(w http.ResponseWriter, r *http.Request) {
	list, version, err := s.spaces.ListShares(r.Context(), actorOf(r), r.PathValue("spaceId"))
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	setETag(w, version) // send it back as If-Match when replacing the list
	writeJSON(w, 200, map[string]any{"items": list})
}

// listSpaceRegistry lists the shared Spaces the caller can see, for the Workspace registry (Batch 3).
func (s *Server) listSpaceRegistry(w http.ResponseWriter, r *http.Request) {
	list, err := s.spaces.Registry(r.Context(), actorOf(r))
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": list})
}

// listSpaceMembers answers who has access to a Space and why (direct share or group). Read-only; changes go through shares.
func (s *Server) listSpaceMembers(w http.ResponseWriter, r *http.Request) {
	list, err := s.spaces.Members(r.Context(), actorOf(r), r.PathValue("spaceId"))
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": list})
}

func (s *Server) replaceSpaceShares(w http.ResponseWriter, r *http.Request) {
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	var input struct {
		Shares []spaces.ShareInput `json:"shares"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Shares == nil {
		writeError(w, http.StatusUnprocessableEntity, "shares is required")
		return
	}
	batch := s.notificationBatch()
	before, _, _ := s.spaces.ListShares(r.Context(), actorOf(r), r.PathValue("spaceId")) // only for the audit diff
	list, newVersion, err := s.spaces.ReplaceShares(r.Context(), actorOf(r), r.PathValue("spaceId"), version, input.Shares, batch)
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	s.auditShares(r, auditShareSpace, "space", r.PathValue("spaceId"), before, list)
	s.publishNotifications(batch)
	setETag(w, newVersion)
	writeJSON(w, 200, map[string]any{"items": list})
}

func pageParams(w http.ResponseWriter, r *http.Request) (limit, offset int, ok bool) {
	limit = 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 {
			writeError(w, http.StatusUnprocessableEntity, "limit must be between 1 and 200")
			return 0, 0, false
		}
		limit = n
	}
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		n, convErr := strconv.Atoi(string(decoded))
		if err != nil || convErr != nil || n < 0 {
			writeError(w, http.StatusUnprocessableEntity, "cursor is not valid")
			return 0, 0, false
		}
		offset = n
	}
	return limit, offset, true
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset, ok := pageParams(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	users, next, err := s.directory.Users(r.Context(), actorOf(r), q.Get("q"), q.Get("status"), q.Get("user_type"), limit, offset)
	if err != nil {
		slog.ErrorContext(r.Context(), "list users", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	var cursor *string
	if next >= 0 {
		c := base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(next)))
		cursor = &c
	}
	writeJSON(w, 200, map[string]any{"items": users, "next_cursor": cursor})
}

func (s *Server) listGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.directory.Groups(r.Context(), actorOf(r))
	if err != nil {
		slog.ErrorContext(r.Context(), "list groups", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, 200, map[string]any{"items": groups})
}
