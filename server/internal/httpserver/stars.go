package httpserver

import (
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const maxStars = 500

func (s *Server) starRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/stars", s.requireUser(s.listStars))
	mux.HandleFunc("GET /api/v1/me/created", s.requireUser(s.listCreated))
	mux.HandleFunc("PUT /api/v1/items/{itemId}/star", s.requireUser(s.setStar))
	mux.HandleFunc("DELETE /api/v1/items/{itemId}/star", s.requireUser(s.setStar))
}

// listStars returns the caller's starred Items that the caller may still view, newest star first.
func (s *Server) listStars(w http.ResponseWriter, r *http.Request) {
	actor, ctx := actorOf(r), r.Context()
	rows, err := s.pool.Query(ctx, `SELECT i.id::text, i.title, i.type::text FROM item_stars st
JOIN items i ON i.id=st.item_id AND i.deleted_at IS NULL WHERE st.user_id=$1::uuid ORDER BY st.created_at DESC, i.id LIMIT $2`, actor.ID, maxStars)
	if err != nil {
		writeAnalyticsError(w, err)
		return
	}
	type star struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Type  string `json:"type"`
	}
	var candidates []star
	var ids []string
	for rows.Next() {
		var st star
		if err := rows.Scan(&st.ID, &st.Title, &st.Type); err != nil {
			rows.Close()
			writeAnalyticsError(w, err)
			return
		}
		candidates, ids = append(candidates, st), append(ids, st.ID)
	}
	rows.Close()
	caps, err := policy.ItemsCaps(ctx, s.pool, actor, ids, nil)
	if err != nil {
		writeAnalyticsError(w, err)
		return
	}
	out := []star{}
	for _, st := range candidates {
		if caps[st.ID].Has(policy.View) {
			out = append(out, st)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

const createdPage = 200

// listCreated returns the Items the caller created and may still view, at any depth, newest first. Access is checked after the
// page is read, so a page can be short while next_cursor is set.
func (s *Server) listCreated(w http.ResponseWriter, r *http.Request) {
	actor, ctx := actorOf(r), r.Context()
	var before *uuid.UUID
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid cursor")
			return
		}
		before = &id
	}
	rows, err := s.pool.Query(ctx, `SELECT i.id::text, i.title, i.type::text FROM items i
WHERE i.created_by=$1::uuid AND i.deleted_at IS NULL
  AND ($2::uuid IS NULL OR (i.created_at,i.id) < (SELECT c.created_at,c.id FROM items c WHERE c.id=$2 AND c.created_by=$1::uuid))
ORDER BY i.created_at DESC, i.id DESC LIMIT $3`, actor.ID, before, createdPage+1)
	if err != nil {
		writeAnalyticsError(w, err)
		return
	}
	type created struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Type  string `json:"type"`
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[created])
	if err != nil {
		writeAnalyticsError(w, err)
		return
	}
	var next *string
	if len(list) > createdPage {
		list = list[:createdPage]
		next = &list[createdPage-1].ID
	}
	ids := make([]string, len(list))
	for i, item := range list {
		ids[i] = item.ID
	}
	caps, err := policy.ItemsCaps(ctx, s.pool, actor, ids, nil)
	if err != nil {
		writeAnalyticsError(w, err)
		return
	}
	out := []created{}
	for _, item := range list {
		if caps[item.ID].Has(policy.View) {
			out = append(out, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "next_cursor": next})
}

// setStar stars (PUT) or unstars (DELETE) an Item for the caller. Both are idempotent; starring needs View, so an Item
// the caller cannot see answers 404 exactly like any other Item route. Unstarring needs nothing: it only touches the caller's own row.
func (s *Server) setStar(w http.ResponseWriter, r *http.Request) {
	actor, ctx := actorOf(r), r.Context()
	id, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method == http.MethodDelete {
		_, err = s.pool.Exec(ctx, "DELETE FROM item_stars WHERE user_id=$1::uuid AND item_id=$2", actor.ID, id)
	} else {
		caps, capsErr := policy.ItemCaps(ctx, s.pool, actor, id.String())
		if capsErr != nil {
			writeAnalyticsError(w, capsErr)
			return
		}
		if !caps.Has(policy.View) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		_, err = s.pool.Exec(ctx, "INSERT INTO item_stars(user_id,item_id) VALUES ($1::uuid,$2) ON CONFLICT DO NOTHING", actor.ID, id)
	}
	if err != nil {
		writeAnalyticsError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
