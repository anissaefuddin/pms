package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/anissaefuddin/pms/server/internal/automation"
)

func (s *Server) automationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/automation-rules", s.requireUser(s.listAutomationRules))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/automation-rules", s.requireUser(s.createAutomationRule))
	mux.HandleFunc("GET /api/v1/automation-rules/{ruleId}", s.requireUser(s.getAutomationRule))
	mux.HandleFunc("PATCH /api/v1/automation-rules/{ruleId}", s.requireUser(s.updateAutomationRule))
	mux.HandleFunc("DELETE /api/v1/automation-rules/{ruleId}", s.requireUser(s.deleteAutomationRule))
	mux.HandleFunc("PUT /api/v1/automation-rules/{ruleId}/draft", s.requireUser(s.saveAutomationDraft))
	mux.HandleFunc("POST /api/v1/automation-rules/{ruleId}/publish", s.requireUser(s.publishAutomationRule))
	mux.HandleFunc("GET /api/v1/automation-rules/{ruleId}/runs", s.requireUser(s.listAutomationRuns))
	mux.HandleFunc("GET /api/v1/automation/dead-events", s.requireUser(s.listDeadEvents))
	mux.HandleFunc("POST /api/v1/automation/events/{eventId}/replay", s.requireUser(s.replayEvent))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/automation-rules/dry-run", s.requireUser(s.dryRunAutomation))
}

func (s *Server) listAutomationRuns(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "limit must be a number")
			return
		}
		limit = n
	}
	runs, next, err := automation.Runs(r.Context(), s.pool, actorOf(r), r.PathValue("ruleId"), r.URL.Query().Get("status"), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": runs, "next_cursor": next})
}

func (s *Server) listDeadEvents(w http.ResponseWriter, r *http.Request) {
	dead, err := automation.DeadEvents(r.Context(), s.pool, actorOf(r))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": dead})
}

func (s *Server) replayEvent(w http.ResponseWriter, r *http.Request) {
	cleared, err := automation.Replay(r.Context(), s.pool, actorOf(r), r.PathValue("eventId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"requeued": true, "reopened_runs": cleared})
}

func (s *Server) dryRunAutomation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Definition json.RawMessage `json:"definition"`
		ItemID     string          `json:"item_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	result, err := automation.New(s.pool).DryRun(r.Context(), actorOf(r), r.PathValue("spaceId"), in.Definition, in.ItemID)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) listAutomationRules(w http.ResponseWriter, r *http.Request) {
	rules, err := automation.List(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rules})
}

func (s *Server) createAutomationRule(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        string          `json:"name"`
		ContainerID *string         `json:"container_id"`
		Definition  json.RawMessage `json:"definition"`
	}
	if !decode(w, r, &in) {
		return
	}
	rule, err := automation.Create(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"), in.Name, in.ContainerID, in.Definition)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) getAutomationRule(w http.ResponseWriter, r *http.Request) {
	rule, err := automation.Get(r.Context(), s.pool, actorOf(r), r.PathValue("ruleId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) updateAutomationRule(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name    *string `json:"name"`
		Enabled *bool   `json:"enabled"`
	}
	if !decode(w, r, &in) {
		return
	}
	rule, err := automation.Update(r.Context(), s.pool, actorOf(r), r.PathValue("ruleId"), in.Name, in.Enabled)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) deleteAutomationRule(w http.ResponseWriter, r *http.Request) {
	if err := automation.Delete(r.Context(), s.pool, actorOf(r), r.PathValue("ruleId")); err != nil {
		writeItemError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) saveAutomationDraft(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Definition json.RawMessage `json:"definition"`
	}
	if !decode(w, r, &in) {
		return
	}
	version, err := automation.SaveDraft(r.Context(), s.pool, actorOf(r), r.PathValue("ruleId"), in.Definition)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, version)
}

func (s *Server) publishAutomationRule(w http.ResponseWriter, r *http.Request) {
	rule, err := automation.Publish(r.Context(), s.pool, actorOf(r), r.PathValue("ruleId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}
