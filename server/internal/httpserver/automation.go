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
	mux.HandleFunc("DELETE /api/v1/automation-rules/{ruleId}", s.requireUser(s.deleteAutomationRule)) // archives; see Archive
	mux.HandleFunc("POST /api/v1/automation-rules/{ruleId}/archive", s.requireUser(s.archiveAutomationRule))
	mux.HandleFunc("POST /api/v1/automation-rules/{ruleId}/restore", s.requireUser(s.restoreAutomationRule))
	mux.HandleFunc("POST /api/v1/automation-rules/{ruleId}/owner", s.requireUser(s.transferAutomationRule))
	mux.HandleFunc("POST /api/v1/automation-rules/{ruleId}/duplicate", s.requireUser(s.duplicateAutomationRule))
	mux.HandleFunc("GET /api/v1/automation-rules/{ruleId}/export", s.requireUser(s.exportAutomationRule))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/automation-rules/import", s.requireUser(s.importAutomationRule))
	mux.HandleFunc("GET /api/v1/automation/templates", s.requireUser(s.automationTemplates))
	mux.HandleFunc("GET /api/v1/automation/workspace-rules", s.requireUser(s.listWorkspaceRules))
	mux.HandleFunc("POST /api/v1/automation/workspace-rules", s.requireUser(s.createWorkspaceRule))
	mux.HandleFunc("POST /api/v1/automation-rules/{ruleId}/targets", s.requireUser(s.setAutomationTargets))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/workspace-automation", s.requireUser(s.spaceWorkspaceRules))
	mux.HandleFunc("GET /api/v1/automation/usage", s.requireUser(s.automationUsage))
	mux.HandleFunc("GET /api/v1/automation/limits", s.requireUser(s.getAutomationLimits))
	mux.HandleFunc("PUT /api/v1/automation/limits", s.requireUser(s.setAutomationLimits))
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
		RuleID     string          `json:"rule_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	result, err := automation.New(s.pool).DryRun(r.Context(), actorOf(r), r.PathValue("spaceId"), in.Definition, in.ItemID, in.RuleID)
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

// Deleting a rule archives it: its versions and run history stay, and it stops running (A1 delete policy).
func (s *Server) deleteAutomationRule(w http.ResponseWriter, r *http.Request) {
	if _, err := automation.Archive(r.Context(), s.pool, actorOf(r), r.PathValue("ruleId")); err != nil {
		writeItemError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) archiveAutomationRule(w http.ResponseWriter, r *http.Request) {
	rule, err := automation.Archive(r.Context(), s.pool, actorOf(r), r.PathValue("ruleId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) restoreAutomationRule(w http.ResponseWriter, r *http.Request) {
	rule, err := automation.Restore(r.Context(), s.pool, actorOf(r), r.PathValue("ruleId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) transferAutomationRule(w http.ResponseWriter, r *http.Request) {
	var in struct {
		OwnerID string `json:"owner_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	rule, err := automation.TransferOwner(r.Context(), s.pool, actorOf(r), r.PathValue("ruleId"), in.OwnerID)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
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

func (s *Server) duplicateAutomationRule(w http.ResponseWriter, r *http.Request) {
	rule, err := automation.Duplicate(r.Context(), s.pool, actorOf(r), r.PathValue("ruleId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) exportAutomationRule(w http.ResponseWriter, r *http.Request) {
	out, err := automation.ExportRule(r.Context(), s.pool, actorOf(r), r.PathValue("ruleId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) importAutomationRule(w http.ResponseWriter, r *http.Request) {
	var in automation.Export
	if !decode(w, r, &in) {
		return
	}
	rule, err := automation.ImportRule(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"), in)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) automationTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": automation.Templates()})
}

func (s *Server) listWorkspaceRules(w http.ResponseWriter, r *http.Request) {
	rules, err := automation.ListWorkspace(r.Context(), s.pool, actorOf(r))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rules})
}

func (s *Server) createWorkspaceRule(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name       string          `json:"name"`
		Definition json.RawMessage `json:"definition"`
	}
	if !decode(w, r, &in) {
		return
	}
	rule, err := automation.CreateWorkspace(r.Context(), s.pool, actorOf(r), in.Name, in.Definition)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) setAutomationTargets(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SpaceIDs []string `json:"space_ids"`
	}
	if !decode(w, r, &in) {
		return
	}
	rule, err := automation.SetTargets(r.Context(), s.pool, actorOf(r), r.PathValue("ruleId"), in.SpaceIDs)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) getAutomationLimits(w http.ResponseWriter, r *http.Request) {
	limits, err := automation.GetLimits(r.Context(), s.pool, actorOf(r))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, limits)
}

func (s *Server) setAutomationLimits(w http.ResponseWriter, r *http.Request) {
	var in automation.Limits
	if !decode(w, r, &in) {
		return
	}
	limits, err := automation.SetLimits(r.Context(), s.pool, actorOf(r), in)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, limits)
}

func (s *Server) spaceWorkspaceRules(w http.ResponseWriter, r *http.Request) {
	rules, err := automation.WorkspaceRulesForSpace(r.Context(), s.pool, actorOf(r), r.PathValue("spaceId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rules})
}

// automationUsage shows the rolling-hour usage against each cap: for a rule (?rule_id), for a Space (?space_id), or for the workspace.
func (s *Server) automationUsage(w http.ResponseWriter, r *http.Request) {
	usage, err := automation.UsageFor(r.Context(), s.pool, actorOf(r), r.URL.Query().Get("rule_id"), r.URL.Query().Get("space_id"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"window": usage.Window, "rule": usage.Rule, "workspace": usage.Workspace, "space": usage.Space, "throttled_by": usage.ThrottledBy()})
}
