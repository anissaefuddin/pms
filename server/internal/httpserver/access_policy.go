package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// Account access policy (ENT-05, D-95): an IP allowlist that applies to signing in and to every signed-in request.
// Public forms, public dashboard links, SCIM, webhooks and the health check are not part of it: they carry their own credentials.
// shortcut: the policy is cached for 10 seconds per process (a saved change applies at once on this process and within 10 s on another);
// move to a notification channel if the server ever runs as several replicas.

const (
	auditIPPolicy = "policy.ip_allowlist_changed"
	policyTTL     = 10 * time.Second
)

type accessPolicy struct {
	OTP      bool
	Enabled  bool
	Prefixes []netip.Prefix
	Version  int
	List     []string
}

type policyCache struct {
	mu     sync.Mutex
	value  accessPolicy
	loaded time.Time
}

func (s *Server) loadPolicy(ctx context.Context) (accessPolicy, error) {
	s.policyCache.mu.Lock()
	defer s.policyCache.mu.Unlock()
	if !s.policyCache.loaded.IsZero() && time.Since(s.policyCache.loaded) < policyTTL {
		return s.policyCache.value, nil
	}
	var p accessPolicy
	if err := s.pool.QueryRow(ctx, "SELECT ip_allowlist_enabled,ip_allowlist,version,otp_login_enabled FROM access_policy WHERE id=1").Scan(&p.Enabled, &p.List, &p.Version, &p.OTP); err != nil {
		return accessPolicy{}, err
	}
	for _, entry := range p.List {
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			p.Prefixes = append(p.Prefixes, prefix)
		}
	}
	s.policyCache.value, s.policyCache.loaded = p, time.Now()
	return p, nil
}

func (s *Server) forgetPolicy() {
	s.policyCache.mu.Lock()
	s.policyCache.loaded = time.Time{}
	s.policyCache.mu.Unlock()
}

func allowedBy(p accessPolicy, ip string) bool {
	if !p.Enabled {
		return true
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range p.Prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// ipAllowed answers whether this request may reach the application. A database failure lets the request through: an outage of the
// policy table must not lock everyone out (the session check that follows still needs the database).
func (s *Server) ipAllowed(r *http.Request) bool {
	if s.cfg.DisableIPAllowlist {
		return true
	}
	p, err := s.loadPolicy(r.Context())
	if err != nil {
		if r.Context().Err() == nil {
			slog.ErrorContext(r.Context(), "access policy", "error", err)
		}
		return true
	}
	return allowedBy(p, s.clientIP(r))
}

func writeIPDenied(w http.ResponseWriter) {
	writeProblem(w, http.StatusForbidden, "ip_not_allowed", "your network address is not allowed to use this application")
}

func (s *Server) accessPolicyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/access-policy", s.requireUser(requireAdmin(s.getAccessPolicy)))
	mux.HandleFunc("PUT /api/v1/access-policy", s.requireUser(requireAdmin(s.putAccessPolicy)))
}

func (s *Server) policyView(ctx context.Context, r *http.Request) (map[string]any, int, error) {
	var enabled, otp bool
	var list []string
	var version int
	if err := s.pool.QueryRow(ctx, "SELECT ip_allowlist_enabled,ip_allowlist,version,otp_login_enabled FROM access_policy WHERE id=1").Scan(&enabled, &list, &version, &otp); err != nil {
		return nil, 0, err
	}
	if list == nil {
		list = []string{}
	}
	return map[string]any{"ip_allowlist_enabled": enabled, "ip_allowlist": list, "otp_login_enabled": otp, "version": version, "your_ip": s.clientIP(r), "override_active": s.cfg.DisableIPAllowlist}, version, nil
}

func (s *Server) getAccessPolicy(w http.ResponseWriter, r *http.Request) {
	view, version, err := s.policyView(r.Context(), r)
	if err != nil {
		adminWriteError(w, err)
		return
	}
	setETag(w, version)
	writeJSON(w, 200, view)
}

// normalizeCIDR accepts an address or a network and returns the network in canonical form.
func normalizeCIDR(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if prefix, err := netip.ParsePrefix(raw); err == nil {
		return prefix.Masked().String(), true
	}
	if addr, err := netip.ParseAddr(raw); err == nil {
		addr = addr.Unmap()
		return netip.PrefixFrom(addr, addr.BitLen()).String(), true
	}
	return "", false
}

func (s *Server) putAccessPolicy(w http.ResponseWriter, r *http.Request) {
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	var in struct {
		Enabled bool     `json:"ip_allowlist_enabled"`
		List    []string `json:"ip_allowlist"`
		OTP     bool     `json:"otp_login_enabled"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.OTP && len(s.cfg.OTPKey) != 32 {
		writeProblem(w, 422, "key_not_configured", "one-time-code sign-in needs PMS_OTP_KEY on the server")
		return
	}
	if len(in.List) > 50 {
		writeError(w, 422, "an allowlist can hold at most 50 entries")
		return
	}
	seen, clean := map[string]bool{}, []string{}
	var prefixes []netip.Prefix
	for _, raw := range in.List {
		entry, ok := normalizeCIDR(raw)
		if !ok {
			writeError(w, 422, "every entry must be an IP address or a network such as 203.0.113.0/24")
			return
		}
		if !seen[entry] {
			seen[entry] = true
			clean = append(clean, entry)
			prefixes = append(prefixes, netip.MustParsePrefix(entry))
		}
	}
	if in.Enabled {
		if len(clean) == 0 {
			writeError(w, 422, "add at least one address before turning the allowlist on")
			return
		}
		// Refuse a policy that would lock the person saving it out: they could not undo it.
		if !allowedBy(accessPolicy{Enabled: true, Prefixes: prefixes}, s.clientIP(r)) {
			writeProblem(w, 422, "self_lockout", "your own address "+s.clientIP(r)+" is not in the list; add it first so you are not locked out")
			return
		}
	}
	err := s.adminTx(r, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, "UPDATE access_policy SET ip_allowlist_enabled=$2,ip_allowlist=$3,otp_login_enabled=$5,version=version+1,updated_at=now(),updated_by=$4::uuid WHERE id=1 AND version=$1", version, in.Enabled, clean, actorOf(r).ID, in.OTP)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return adminError{409, "version_conflict", "the policy was changed by someone else; reload and try again"}
		}
		return s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditIPPolicy, TargetType: "policy", TargetID: "ip_allowlist", Details: map[string]any{"enabled": in.Enabled, "entries": clean, "otp_login": in.OTP}})
	})
	if err != nil {
		adminWriteError(w, err)
		return
	}
	s.forgetPolicy()
	view, newVersion, err := s.policyView(r.Context(), r)
	if err != nil {
		adminWriteError(w, err)
		return
	}
	setETag(w, newVersion)
	writeJSON(w, 200, view)
}
