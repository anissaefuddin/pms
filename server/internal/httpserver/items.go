package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/spaces"
)

func (s *Server) itemShares(w http.ResponseWriter, r *http.Request) {
	var version *int
	var input struct {
		Shares []spaces.ShareInput `json:"shares"`
	}
	if r.Method == http.MethodPut {
		v, ok := ifMatchVersion(w, r)
		if !ok || !decode(w, r, &input) {
			return
		}
		if input.Shares == nil {
			writeError(w, 422, "shares is required")
			return
		}
		version = &v
	}
	batch := s.notificationBatch()
	var before []spaces.Share
	if version != nil {
		before, _, _ = s.spaces.ItemShares(r.Context(), actorOf(r), r.PathValue("itemId"), nil, nil, nil) // only for the audit diff
	}
	rows, current, err := s.spaces.ItemShares(r.Context(), actorOf(r), r.PathValue("itemId"), version, input.Shares, batch)
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	if version != nil {
		s.auditShares(r, auditShareItem, "item", r.PathValue("itemId"), before, rows)
	}
	s.publishNotifications(batch)
	setETag(w, current)
	writeJSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (s *Server) getItem(w http.ResponseWriter, r *http.Request) {
	item, err := items.Get(r.Context(), s.pool, actorOf(r), r.PathValue("itemId"))
	writeItem(w, http.StatusOK, item, err)
}

func (s *Server) listWorkflows(w http.ResponseWriter, r *http.Request) {
	var result json.RawMessage
	err := s.pool.QueryRow(r.Context(), `
SELECT jsonb_build_object('items',COALESCE(jsonb_agg(`+workflowJSON+` ORDER BY w.is_default DESC,w.name,w.id),'[]'::jsonb))
FROM workflows w`).Scan(&result)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) listRootItems(w http.ResponseWriter, r *http.Request) {
	s.listItems(w, r, r.PathValue("spaceId"), false)
}

func (s *Server) listChildItems(w http.ResponseWriter, r *http.Request) {
	s.listItems(w, r, r.PathValue("itemId"), true)
}

func (s *Server) listItems(w http.ResponseWriter, r *http.Request, container string, children bool) {
	query := r.URL.Query()
	limit := 50
	if query.Has("limit") {
		var err error
		limit, err = strconv.Atoi(query.Get("limit"))
		if err != nil {
			writeItemError(w, items.Invalid("limit must be an integer"))
			return
		}
	}
	split := func(key string) []string {
		if value := query.Get(key); value != "" {
			return strings.Split(value, ",")
		}
		return nil
	}
	page, err := items.List(r.Context(), s.pool, actorOf(r), container, children, items.ListFilter{
		Limit: limit, Types: split("type"), StatusGroups: split("status_group"),
		Assignee: query.Get("assignee"), Sort: query.Get("sort"), Cursor: query.Get("cursor"),
		Fields: query["field"],
	})
	if err != nil {
		if !errors.Is(r.Context().Err(), context.Canceled) {
			writeItemError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) createItem(w http.ResponseWriter, r *http.Request) {
	var fields map[string]json.RawMessage
	if !decode(w, r, &fields) {
		return
	}
	batch := s.notificationBatch()
	item, err := items.Create(r.Context(), s.pool, actorOf(r), fields, batch)
	if err == nil {
		s.publishNotifications(batch)
		var created struct{ ID string }
		_ = json.Unmarshal(item.Body, &created)
		s.publishItem(r, "item.created", created.ID, created.ID, item.Version)
	}
	writeItem(w, http.StatusCreated, item, err)
}

func (s *Server) deleteItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("itemId")
	if err := items.Delete(r.Context(), s.pool, actorOf(r), id); err != nil {
		writeItemError(w, err)
		return
	}
	s.publishItem(r, "item.deleted", id, id, 0)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) reorderItem(w http.ResponseWriter, r *http.Request) {
	var input items.ReorderInput
	if !decode(w, r, &input) {
		return
	}
	if err := items.Reorder(r.Context(), s.pool, actorOf(r), r.PathValue("itemId"), input); err != nil {
		writeItemError(w, err)
		return
	}
	id := r.PathValue("itemId")
	s.publishItem(r, "item.updated", id, id, 0)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) replaceItemLocations(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Locations []items.Location `json:"locations"`
	}
	if !decode(w, r, &input) {
		return
	}
	id := r.PathValue("itemId")
	item, err := items.ReplaceLocations(r.Context(), s.pool, actorOf(r), id, input.Locations)
	if err == nil {
		s.publishItem(r, "item.updated", id, id, item.Version)
	}
	writeItem(w, http.StatusOK, item, err)
}

func (s *Server) patchItem(w http.ResponseWriter, r *http.Request) {
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	var fields map[string]json.RawMessage
	if !decode(w, r, &fields) {
		return
	}
	batch := s.notificationBatch()
	item, err := items.Patch(r.Context(), s.pool, actorOf(r), r.PathValue("itemId"), version, fields, batch)
	if err == nil {
		s.publishNotifications(batch)
		id := r.PathValue("itemId")
		if len(item.ScheduleChanges) == 0 {
			s.publishItem(r, "item.updated", id, id, item.Version)
		} else {
			s.publishSchedule(item.ScheduleChanges, id, item.Version)
		}
	}
	writeItem(w, http.StatusOK, item, err)
}

func writeItem(w http.ResponseWriter, status int, item items.Detail, err error) {
	if err != nil {
		writeItemError(w, err)
		return
	}
	setETag(w, item.Version)
	writeJSON(w, status, item.Body)
}

func writeItemError(w http.ResponseWriter, err error) {
	var invalid items.Invalid
	var field items.InvalidField
	var guard items.GuardError
	switch {
	case errors.Is(err, items.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, items.ErrForbidden):
		writeError(w, http.StatusForbidden, "item operation forbidden")
	case errors.Is(err, items.ErrConflict):
		writeError(w, http.StatusConflict, "the Item changed; reload and try again")
	case errors.Is(err, errApprovalFinal):
		writeProblem(w, http.StatusConflict, "approval_final", "this approval is already decided and cannot be changed")
	case errors.As(err, &guard):
		problems := make([]map[string]string, len(guard.Problems))
		for i, p := range guard.Problems {
			problems[i] = map[string]string{"field": p.Field, "message": p.Message}
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "about:blank", "title": http.StatusText(http.StatusUnprocessableEntity),
			"status": http.StatusUnprocessableEntity, "code": "transition_blocked",
			"detail": guard.Error(), "request_id": w.Header().Get("X-Request-ID"), "errors": problems,
		})
	case errors.As(err, &field):
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "about:blank", "title": http.StatusText(http.StatusUnprocessableEntity),
			"status": http.StatusUnprocessableEntity, "code": "validation_failed",
			"detail": field.Message, "request_id": w.Header().Get("X-Request-ID"),
			"errors": []map[string]string{{"field": field.Field, "message": field.Message}},
		})
	case errors.As(err, &invalid):
		writeError(w, http.StatusUnprocessableEntity, invalid.Error())
	default:
		slog.Error("item operation", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func (s *Server) listSharedItems(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := items.ListFilter{Limit: 50, Cursor: query.Get("cursor")}
	if query.Has("limit") {
		var err error
		filter.Limit, err = strconv.Atoi(query.Get("limit"))
		if err != nil {
			writeItemError(w, items.Invalid("limit must be an integer"))
			return
		}
	}
	page, err := items.ListShared(r.Context(), s.pool, actorOf(r), filter)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
