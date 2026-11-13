package httpserver

import (
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/items"
)

// portfolioRoutes is kept separate so the main router only needs one wiring call.
func (s *Server) portfolioRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/portfolio/timeline", s.requireUser(s.portfolioTimeline))
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
