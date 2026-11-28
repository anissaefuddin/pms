package httpserver

import (
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/items"
)

// portfolioRoutes is kept separate so the main router only needs one wiring call.
func (s *Server) portfolioRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/portfolio/timeline", s.requireUser(s.portfolioTimeline))
	mux.HandleFunc("GET /api/v1/portfolio/projects", s.requireUser(s.portfolioProjects))
}

func (s *Server) portfolioTimeline(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProjectIDs []string `json:"project_ids"`
	}
	if !decode(w, r, &body) {
		return
	}
	timeline, err := items.Portfolio(r.Context(), s.pool, actorOf(r), body.ProjectIDs)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, timeline)
}

// portfolioProjects is the consolidated monitor (PROMPT 034). Rows, summary and options all come from the actor's authorized projects.
func (s *Server) portfolioProjects(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := items.PortfolioFilter{Customer: q.Get("customer"), Owner: q.Get("owner"), Health: q.Get("health"), Status: q.Get("status"),
		Query: q.Get("q"), Sort: q.Get("sort"), Desc: q.Get("order") == "desc"}
	switch f.Health {
	case "", "green", "yellow", "red", "none":
	default:
		writeError(w, http.StatusUnprocessableEntity, "health must be green, yellow, red or none")
		return
	}
	switch f.Status {
	case "", "active", "completed", "deferred", "cancelled":
	default:
		writeError(w, http.StatusUnprocessableEntity, "status must be active, completed, deferred or cancelled")
		return
	}
	switch f.Sort {
	case "", "title", "finish", "progress", "health", "overdue", "updated", "customer", "owner":
	default:
		writeError(w, http.StatusUnprocessableEntity, "sort is not supported")
		return
	}
	if len(f.Query) > 200 {
		writeError(w, http.StatusUnprocessableEntity, "q is too long")
		return
	}
	out, err := items.PortfolioProjects(r.Context(), s.pool, actorOf(r), f)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
