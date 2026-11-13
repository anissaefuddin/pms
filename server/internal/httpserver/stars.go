package httpserver

import (
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
)

const maxStars = 500

func (s *Server) starRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/stars", s.requireUser(s.listStars))
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
