package httpserver

import (
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/analytics"
)

// runAnalytics answers one closed-schema query for the caller; the same engine serves widgets and reports.
func (s *Server) runAnalytics(w http.ResponseWriter, r *http.Request) {
	var q analytics.Query
	if !decode(w, r, &q) {
		return
	}
	result, err := analytics.Run(r.Context(), s.pool, actorOf(r), q)
	if err != nil {
		writeAnalyticsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
