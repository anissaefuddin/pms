package httpserver

import (
	"errors"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/twofactor"
	"github.com/anissaefuddin/pms/server/internal/webhooks"
	"github.com/google/uuid"
)

// Webhook subscriptions belong to their creator (API-02); nobody else can see or change them, so another user's id is "not found".
func (s *Server) webhookRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/webhooks", s.requireUser(s.listWebhooks))
	mux.HandleFunc("POST /api/v1/webhooks", s.requireUser(s.createWebhook))
	mux.HandleFunc("PATCH /api/v1/webhooks/{webhookId}", s.requireUser(s.updateWebhook))
	mux.HandleFunc("DELETE /api/v1/webhooks/{webhookId}", s.requireUser(s.deleteWebhook))
	mux.HandleFunc("GET /api/v1/webhooks/{webhookId}/deliveries", s.requireUser(s.listWebhookDeliveries))
}

// webhookID turns a malformed id into one nothing matches, so it answers 404 like any unknown id.
func webhookID(r *http.Request) string {
	if _, err := uuid.Parse(r.PathValue("webhookId")); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	return r.PathValue("webhookId")
}

func writeWebhookError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, webhooks.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, webhooks.ErrNoKey):
		writeProblem(w, http.StatusUnprocessableEntity, "key_not_configured", "webhook secrets need PMS_CONNECTOR_KEY on the server")
	case errors.Is(err, webhooks.ErrInvalid):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "webhook operation failed")
	}
}

func (s *Server) listWebhooks(w http.ResponseWriter, r *http.Request) {
	items, err := s.hooks().List(r.Context(), actorOf(r).ID)
	if err != nil {
		writeWebhookError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "event_types": webhooks.EventTypes})
}

func (s *Server) createWebhook(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name       string   `json:"name"`
		URL        string   `json:"url"`
		EventTypes []string `json:"event_types"`
	}
	if !decode(w, r, &in) {
		return
	}
	sub, err := s.hooks().Create(r.Context(), actorOf(r).ID, in.Name, in.URL, in.EventTypes)
	if err != nil {
		writeWebhookError(w, err)
		return
	}
	s.audit(r, actorOf(r).ID, "", auditEvent{Action: auditWebhookCreated, TargetType: "webhook", TargetID: sub.ID, Details: map[string]any{"url": sub.URL}})
	writeJSON(w, http.StatusCreated, sub)
}

func (s *Server) updateWebhook(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name       *string  `json:"name"`
		URL        *string  `json:"url"`
		EventTypes []string `json:"event_types"`
		Enabled    *bool    `json:"enabled"`
	}
	if !decode(w, r, &in) {
		return
	}
	sub, err := s.hooks().Update(r.Context(), actorOf(r).ID, webhookID(r), in.Name, in.URL, in.EventTypes, in.Enabled)
	if err != nil {
		writeWebhookError(w, err)
		return
	}
	s.audit(r, actorOf(r).ID, "", auditEvent{Action: auditWebhookUpdated, TargetType: "webhook", TargetID: sub.ID})
	writeJSON(w, http.StatusOK, sub)
}

func (s *Server) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	if err := s.hooks().Delete(r.Context(), actorOf(r).ID, webhookID(r)); err != nil {
		writeWebhookError(w, err)
		return
	}
	s.audit(r, actorOf(r).ID, "", auditEvent{Action: auditWebhookDeleted, TargetType: "webhook", TargetID: webhookID(r)})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	items, err := s.hooks().Deliveries(r.Context(), actorOf(r).ID, webhookID(r))
	if err != nil {
		writeWebhookError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// hooks builds the webhook service with the integration key, so signing secrets are encrypted at rest when it is configured.
func (s *Server) hooks() *webhooks.Service {
	return webhooks.New(s.pool).WithKeys(twofactor.Keys{Current: s.cfg.ConnectorKey, Previous: s.cfg.ConnectorKeyPrevious})
}
