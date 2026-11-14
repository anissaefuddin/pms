package httpserver

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/connectors"
	"github.com/anissaefuddin/pms/server/internal/connectors/github"
	"github.com/anissaefuddin/pms/server/internal/twofactor"
	"github.com/google/uuid"
)

var errHandled = errors.New("response already written")

func (s *Server) connectorService() *connectors.Service {
	return &connectors.Service{Pool: s.pool, Keys: twofactor.Keys{Current: s.cfg.ConnectorKey, Previous: s.cfg.ConnectorKeyPrevious}}
}

func writeConnectorError(w http.ResponseWriter, err error) {
	var invalid connectors.Invalid
	switch {
	case errors.As(err, &invalid):
		writeError(w, http.StatusUnprocessableEntity, invalid.Error())
	case errors.Is(err, connectors.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, connectors.ErrDisabled), errors.Is(err, connectors.ErrAlreadyClosed):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, connectors.ErrNoKey):
		writeError(w, http.StatusServiceUnavailable, "connector secrets need PMS_CONNECTOR_KEY on the server")
	default:
		slog.Error("connector operation failed", "error", err)
		writeError(w, http.StatusBadGateway, "connector operation failed")
	}
}

// connectorRoutes: administrators only. The id in the path is validated as a uuid before any query.
func (s *Server) connectorRoutes(mux *http.ServeMux) {
	admin := func(h http.HandlerFunc) http.HandlerFunc { return s.requireUser(requireAdmin(h)) }
	id := func(r *http.Request, name string) (string, bool) {
		v, err := uuid.Parse(r.PathValue(name))
		return v.String(), err == nil
	}
	one := func(do func(w http.ResponseWriter, r *http.Request, id string) (any, error)) http.HandlerFunc {
		return admin(func(w http.ResponseWriter, r *http.Request) {
			cid, ok := id(r, "connectorId")
			if !ok {
				writeError(w, http.StatusNotFound, "not found")
				return
			}
			out, err := do(w, r, cid)
			if errors.Is(err, errHandled) {
				return
			}
			if err != nil {
				writeConnectorError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	}
	// GitHub calls this without a session; the HMAC signature of the install's webhook_secret is the credential.
	// It only marks the install due, so a replayed or forged-then-rejected delivery can do no more than an early poll.
	mux.HandleFunc("POST /api/v1/connectors/{connectorId}/github-webhook", func(w http.ResponseWriter, r *http.Request) {
		cid, ok := id(r, "connectorId")
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if !ok || err != nil {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		svc := s.connectorService()
		kind, secret, err := svc.Secret(r.Context(), cid, "webhook_secret")
		if err != nil || kind != "github" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if !github.VerifySignature(secret, body, r.Header.Get("X-Hub-Signature-256")) {
			writeError(w, http.StatusUnauthorized, "bad signature")
			return
		}
		if r.Header.Get("X-GitHub-Event") == "issues" {
			if err := svc.RequestSync(r.Context(), cid); err != nil {
				writeConnectorError(w, err)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v1/connectors", admin(func(w http.ResponseWriter, r *http.Request) {
		list, err := s.connectorService().List(r.Context())
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"kinds": connectors.Default.Kinds(), "items": list})
	}))
	mux.HandleFunc("POST /api/v1/connectors", admin(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Kind    string            `json:"kind"`
			Name    string            `json:"name"`
			Config  map[string]any    `json:"config"`
			Secrets map[string]string `json:"secrets"`
		}
		if !decode(w, r, &in) {
			return
		}
		out, err := s.connectorService().Install(r.Context(), r.Context().Value(userKey).(string), in.Kind, in.Name, in.Config, in.Secrets)
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}))
	mux.HandleFunc("PATCH /api/v1/connectors/{connectorId}", one(func(w http.ResponseWriter, r *http.Request, cid string) (any, error) {
		var in struct {
			Name    *string           `json:"name"`
			Config  map[string]any    `json:"config"`
			Secrets map[string]string `json:"secrets"`
		}
		if !decode(w, r, &in) {
			return nil, errHandled
		}
		return s.connectorService().Configure(r.Context(), cid, in.Name, in.Config, in.Secrets)
	}))
	mux.HandleFunc("POST /api/v1/connectors/{connectorId}/disable", one(func(w http.ResponseWriter, r *http.Request, cid string) (any, error) {
		return s.connectorService().SetEnabled(r.Context(), cid, false)
	}))
	mux.HandleFunc("POST /api/v1/connectors/{connectorId}/enable", one(func(w http.ResponseWriter, r *http.Request, cid string) (any, error) {
		return s.connectorService().SetEnabled(r.Context(), cid, true)
	}))
	mux.HandleFunc("POST /api/v1/connectors/{connectorId}/links", one(func(w http.ResponseWriter, r *http.Request, cid string) (any, error) {
		var in struct {
			ItemID     string `json:"item_id"`
			ExternalID string `json:"external_id"`
		}
		if !decode(w, r, &in) {
			return nil, errHandled
		}
		if _, err := uuid.Parse(in.ItemID); err != nil {
			return nil, connectors.Invalid("item_id must be a uuid")
		}
		ext, err := s.connectorService().Link(r.Context(), cid, in.ItemID, in.ExternalID)
		return map[string]string{"external_id": ext}, err
	}))
	mux.HandleFunc("POST /api/v1/connectors/{connectorId}/sync", one(func(w http.ResponseWriter, r *http.Request, cid string) (any, error) {
		return s.connectorService().Sync(r.Context(), cid)
	}))
	mux.HandleFunc("GET /api/v1/connectors/{connectorId}/conflicts", one(func(w http.ResponseWriter, r *http.Request, cid string) (any, error) {
		list, err := s.connectorService().Conflicts(r.Context(), cid, r.URL.Query().Get("status") == "all")
		return map[string]any{"items": list}, err
	}))
	mux.HandleFunc("POST /api/v1/connector-conflicts/{conflictId}/resolve", admin(func(w http.ResponseWriter, r *http.Request) {
		cid, ok := id(r, "conflictId")
		var in struct {
			Resolution string `json:"resolution"`
		}
		if !ok {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if !decode(w, r, &in) {
			return
		}
		if err := s.connectorService().Resolve(r.Context(), cid, in.Resolution, r.Context().Value(userKey).(string)); err != nil {
			writeConnectorError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}
