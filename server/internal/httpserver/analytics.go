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

// runOperationalAnalytics answers a Workspace-level aggregate for an authorized admin (A1.3). Every successful and refused
// call is audited with the query shape (never the rows), so privileged use stays visible.
func (s *Server) runOperationalAnalytics(w http.ResponseWriter, r *http.Request) {
	var q analytics.Query
	if !decode(w, r, &q) {
		return
	}
	actor := actorOf(r)
	result, err := analytics.RunOperational(r.Context(), s.pool, actor, q)
	detail := map[string]any{"reporting_on": q.ReportingOn, "metric": q.Metric, "group_by": q.GroupBy, "space_ids": len(q.SpaceIDs)}
	if err != nil {
		s.audit(r, actor.ID, "", auditEvent{Action: auditOperationalDenied, TargetType: "report", TargetID: q.ReportingOn, Details: detail})
		writeAnalyticsError(w, err)
		return
	}
	detail["groups"] = len(result.Rows)
	detail["suppressed"] = result.Suppressed
	s.audit(r, actor.ID, "", auditEvent{Action: auditOperationalRead, TargetType: "report", TargetID: q.ReportingOn, Details: detail})
	writeJSON(w, http.StatusOK, result)
}
