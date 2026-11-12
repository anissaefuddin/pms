package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/items"
)

func (s *Server) getItem(w http.ResponseWriter, r *http.Request) {
	item, err := items.Get(r.Context(), s.pool, actorOf(r), r.PathValue("itemId"))
	writeItem(w, http.StatusOK, item, err)
}

func (s *Server) listWorkflows(w http.ResponseWriter, r *http.Request) {
	var result json.RawMessage
	err := s.pool.QueryRow(r.Context(), `
SELECT jsonb_build_object('items',COALESCE(jsonb_agg(jsonb_build_object(
  'id',w.id,'name',w.name,'is_default',w.is_default,
  'statuses',COALESCE((SELECT jsonb_agg(jsonb_build_object(
    'id',s.id,'name',s.name,'color',s.color,'group',s.grp) ORDER BY s.position)
    FROM statuses s WHERE s.workflow_id=w.id),'[]'::jsonb)
) ORDER BY w.is_default DESC,w.name,w.id),'[]'::jsonb))
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
	})
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) createItem(w http.ResponseWriter, r *http.Request) {
	var fields map[string]json.RawMessage
	if !decode(w, r, &fields) {
		return
	}
	item, err := items.Create(r.Context(), s.pool, actorOf(r), fields)
	writeItem(w, http.StatusCreated, item, err)
}

func (s *Server) deleteItem(w http.ResponseWriter, r *http.Request) {
	if err := items.Delete(r.Context(), s.pool, actorOf(r), r.PathValue("itemId")); err != nil {
		writeItemError(w, err)
		return
	}
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
	w.WriteHeader(http.StatusNoContent)
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
	item, err := items.Patch(r.Context(), s.pool, actorOf(r), r.PathValue("itemId"), version, fields)
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
	switch {
	case errors.Is(err, items.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, items.ErrForbidden):
		writeError(w, http.StatusForbidden, "item operation forbidden")
	case errors.Is(err, items.ErrConflict):
		writeError(w, http.StatusConflict, "the Item changed; reload and try again")
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
