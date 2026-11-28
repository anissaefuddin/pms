package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/pydio"
	"github.com/jackc/pgx/v5"
)

// Pydio folder link settings. The PMS_PYDIO_* environment applies until an administrator saves settings; the saved row then replaces
// it as a whole, and removing the row brings the environment back. The listing switch and its token stay with the operator.
// shortcut: the settings are cached for 10 seconds per process, like the access policy (a saved change applies at once on this
// process and within 10 s on another).

const (
	auditPydioConfigured = "pydio.configured"
	auditPydioRemoved    = "pydio.removed"
	pydioTTL             = 10 * time.Second
	maxPydioRoots        = 50
	maxPydioCustomers    = 500
)

type pydioSettings struct {
	Enabled       bool                `json:"enabled"`
	BaseURL       string              `json:"base_url"`
	Roots         []string            `json:"roots"`
	CustomerRoots map[string][]string `json:"customer_roots"`
}

type pydioCache struct {
	mu     sync.Mutex
	value  pydioRuntime
	loaded time.Time
}

type pydioView struct {
	Source  string `json:"source"` // database | environment | none
	Version int    `json:"version"`
	pydioSettings
	ListingEnabled        bool `json:"listing_enabled"`
	ListingTokenAvailable bool `json:"listing_token_available"`
}

// pydioRuntime is what the request handlers use. client is nil while Pydio links are off.
type pydioRuntime struct {
	client        *pydio.Client
	customerRoots map[string][]string
}

// pydioToken returns the operator's token only for the address it was configured with: saved settings that point elsewhere must
// never make PMS send the token to another host.
func (s *Server) pydioToken(baseURL string) string {
	if baseURL != s.cfg.PydioLinkURL {
		return ""
	}
	return s.cfg.PydioLinkToken
}

// pydioCurrent returns the settings in force and where they come from.
func (s *Server) pydioCurrent(ctx context.Context) (pydioView, error) {
	view := pydioView{Source: "database", ListingEnabled: s.cfg.PydioListing}
	var customers []byte
	err := s.pool.QueryRow(ctx, "SELECT enabled,base_url,roots,customer_roots,version FROM pydio_settings WHERE id=1").
		Scan(&view.Enabled, &view.BaseURL, &view.Roots, &customers, &view.Version)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		view.Source, view.Enabled = "environment", s.cfg.PydioLinkURL != ""
		if !view.Enabled && len(s.cfg.PydioCustomerRoots) == 0 {
			view.Source = "none"
		}
		view.BaseURL, view.Roots, view.CustomerRoots = s.cfg.PydioLinkURL, s.cfg.PydioLinkRoots, s.cfg.PydioCustomerRoots
	case err != nil:
		return pydioView{}, err
	default:
		if err := json.Unmarshal(customers, &view.CustomerRoots); err != nil {
			return pydioView{}, err
		}
	}
	if view.Roots == nil {
		view.Roots = []string{}
	}
	if view.CustomerRoots == nil {
		view.CustomerRoots = map[string][]string{}
	}
	view.ListingTokenAvailable = s.pydioToken(view.BaseURL) != ""
	return view, nil
}

// pydioNow returns the runtime in force. If the settings cannot be read, links are off: falling back to the environment could
// switch on what an administrator turned off.
func (s *Server) pydioNow(ctx context.Context) pydioRuntime {
	s.pydioCache.mu.Lock()
	defer s.pydioCache.mu.Unlock()
	if !s.pydioCache.loaded.IsZero() && time.Since(s.pydioCache.loaded) < pydioTTL {
		return s.pydioCache.value
	}
	// Bounded: a value check runs inside a write transaction and must not wait for a free connection without end.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	view, err := s.pydioCurrent(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "pydio settings", "error", err)
		return pydioRuntime{}
	}
	var rt pydioRuntime
	if view.Enabled {
		rt = pydioRuntime{client: pydio.New(view.BaseURL, s.pydioToken(view.BaseURL), view.Roots), customerRoots: view.CustomerRoots}
	}
	s.pydioCache.value, s.pydioCache.loaded = rt, time.Now()
	return rt
}

func (s *Server) forgetPydio() {
	s.pydioCache.mu.Lock()
	s.pydioCache.loaded = time.Time{}
	s.pydioCache.mu.Unlock()
}

// cleanPydioRoots trims and de-duplicates a root list and refuses anything that is not a workspace path.
func cleanPydioRoots(in []string) ([]string, bool) {
	var out []string
	for _, root := range in {
		root = strings.Trim(strings.TrimSpace(root), "/")
		if len(root) > 500 || !config.CheckPydioRoot(root) {
			return nil, false
		}
		if !slices.Contains(out, root) {
			out = append(out, root)
		}
	}
	return out, len(out) >= 1 && len(out) <= maxPydioRoots
}

// normalize validates the input in place. The error text is shown to the administrator.
func (p *pydioSettings) normalize() error {
	p.BaseURL = strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if len(p.BaseURL) > 500 || !config.CheckPydioOrigin(p.BaseURL) {
		return errors.New("base_url must be an https address without credentials or a path, for example https://dms.example.com")
	}
	roots, ok := cleanPydioRoots(p.Roots)
	if !ok {
		return fmt.Errorf("roots needs 1 to %d workspace paths such as projects, without empty or dot segments", maxPydioRoots)
	}
	p.Roots = roots
	if len(p.CustomerRoots) > maxPydioCustomers {
		return fmt.Errorf("customer_roots can hold at most %d Customers", maxPydioCustomers)
	}
	customers := map[string][]string{}
	for code, list := range p.CustomerRoots {
		cleaned, ok := cleanPydioRoots(list)
		if !ok {
			return fmt.Errorf("every Customer needs 1 to %d workspace paths without empty or dot segments", maxPydioRoots)
		}
		for _, root := range cleaned {
			if !slices.ContainsFunc(p.Roots, func(top string) bool { return root == top || strings.HasPrefix(root, top+"/") }) {
				return fmt.Errorf("the folder %q of Customer %q is outside roots", root, code)
			}
		}
		customers[code] = cleaned
	}
	p.CustomerRoots = customers
	return nil
}

func (s *Server) pydioSettingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/pydio/settings", s.requireUser(requireAdmin(s.getPydioSettings)))
	mux.HandleFunc("PUT /api/v1/pydio/settings", s.requireUser(requireAdmin(s.putPydioSettings)))
	mux.HandleFunc("DELETE /api/v1/pydio/settings", s.requireUser(requireAdmin(s.deletePydioSettings)))
}

func (s *Server) getPydioSettings(w http.ResponseWriter, r *http.Request) {
	view, err := s.pydioCurrent(r.Context())
	if err != nil {
		adminWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// putPydioSettings creates (version 0) or replaces (current version) the saved settings.
func (s *Server) putPydioSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version *int `json:"version"`
		pydioSettings
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Version == nil || *in.Version < 0 || *in.Version >= 1<<31-1 {
		writeError(w, http.StatusUnprocessableEntity, "version is required: 0 to create, otherwise the version you edited")
		return
	}
	if err := in.pydioSettings.normalize(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	codes := make([]string, 0, len(in.CustomerRoots))
	for code := range in.CustomerRoots {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	customers, err := json.Marshal(in.CustomerRoots)
	if err != nil {
		adminWriteError(w, err)
		return
	}
	err = s.adminTx(r, func(ctx context.Context, tx pgx.Tx) error {
		var unknown []string
		if err := tx.QueryRow(ctx, `SELECT COALESCE(array_agg(c ORDER BY c), '{}') FROM unnest($1::text[]) AS c
WHERE NOT EXISTS (SELECT 1 FROM clients WHERE code=c)`, codes).Scan(&unknown); err != nil {
			return err
		}
		if len(unknown) > 0 {
			return adminError{http.StatusUnprocessableEntity, "validation_failed", "unknown Customer code: " + strings.Join(unknown, ", ")}
		}
		query := `INSERT INTO pydio_settings(id,enabled,base_url,roots,customer_roots,updated_by) VALUES (1,$1,$2,$3,$4,$5::uuid) ON CONFLICT (id) DO NOTHING`
		args := []any{in.Enabled, in.BaseURL, in.Roots, customers, actorOf(r).ID}
		if *in.Version > 0 {
			query = `UPDATE pydio_settings SET enabled=$1,base_url=$2,roots=$3,customer_roots=$4,updated_by=$5::uuid,updated_at=now(),version=version+1 WHERE id=1 AND version=$6`
			args = append(args, *in.Version)
		}
		tag, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return adminError{http.StatusConflict, "version_conflict", "the settings were changed by someone else; reload and try again"}
		}
		return s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditPydioConfigured, TargetType: "pydio", TargetID: "settings",
			Details: map[string]any{"enabled": in.Enabled, "base_url": in.BaseURL, "roots": in.Roots, "customer_roots": in.CustomerRoots}})
	})
	if err != nil {
		adminWriteError(w, err)
		return
	}
	s.forgetPydio()
	s.getPydioSettings(w, r)
}

// deletePydioSettings removes the saved settings, so the environment applies again.
func (s *Server) deletePydioSettings(w http.ResponseWriter, r *http.Request) {
	err := s.adminTx(r, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, "DELETE FROM pydio_settings WHERE id=1")
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return adminError{http.StatusNotFound, "not_found", "no Pydio settings are saved"}
		}
		return s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditPydioRemoved, TargetType: "pydio", TargetID: "settings"})
	})
	if err != nil {
		adminWriteError(w, err)
		return
	}
	s.forgetPydio()
	w.WriteHeader(http.StatusNoContent)
}
