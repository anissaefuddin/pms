package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/anissaefuddin/pms/server/internal/sso"
	"github.com/anissaefuddin/pms/server/internal/twofactor"
	"github.com/jackc/pgx/v5"
)

const (
	auditSSOConfigured = "sso.configured"
	auditSSORemoved    = "sso.removed"
	auditSSOLogin      = "sso.login"
	auditSSOFailed     = "sso.login_failed"
	auditSSOLinked     = "sso.linked"
	auditSSOUnlinked   = "sso.unlinked"
	ssoCookie          = "pms_sso"
)

func (s *Server) ssoStore() sso.Store {
	return sso.Store{Pool: s.pool, Keys: twofactor.Keys{Current: s.cfg.SSOKey, Previous: s.cfg.SSOKeyPrevious}}
}

func (s *Server) ssoRedirectURL(kind string) string {
	return s.cfg.PublicURL + "/api/v1/auth/sso/" + kind + "/callback"
}

func (s *Server) ssoRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/sso/providers", s.ssoProviders)
	mux.HandleFunc("GET /api/v1/auth/sso/{kind}/start", s.ssoStart)
	mux.HandleFunc("GET /api/v1/auth/sso/{kind}/callback", s.ssoCallback)
	mux.HandleFunc("GET /api/v1/sso/{kind}", s.requireUser(requireAdmin(s.ssoGetConfig)))
	mux.HandleFunc("PUT /api/v1/sso/{kind}", s.requireUser(requireAdmin(s.ssoPutConfig)))
	mux.HandleFunc("DELETE /api/v1/sso/{kind}", s.requireUser(requireAdmin(s.ssoDeleteConfig)))
	mux.HandleFunc("GET /api/v1/me/identities", s.requireUser(s.myIdentities))
	mux.HandleFunc("DELETE /api/v1/me/identities/{kind}", s.requireUser(s.unlinkIdentity))
}

// localPath returns next only when it is a path on this site; anything else (another host, //host, backslashes) becomes "/".
func localPath(next string) string {
	if next == "" || len(next) > 500 || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.ContainsAny(next, "\\\r\n\t") {
		return "/"
	}
	if u, err := url.Parse(next); err != nil || u.Host != "" || u.Scheme != "" {
		return "/"
	}
	return next
}

func knownSSOKind(kind string) bool { return kind == "oidc" }

// ssoProviders lists the enabled providers for the sign-in page. It is public and says nothing but the kind and display name.
func (s *Server) ssoProviders(w http.ResponseWriter, r *http.Request) {
	if wait := s.publicLimiter.take("sso:" + s.clientIP(r)); wait > 0 {
		writeRateLimit(w, wait)
		return
	}
	list, err := s.ssoStore().Enabled(r.Context())
	if err != nil {
		slog.Error("sso providers", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	type entry struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	}
	out := []entry{}
	for _, p := range list {
		out = append(out, entry{p.Kind, p.Name})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// sessionUserID returns the user of the request's session cookie, without writing a response.
func (s *Server) sessionUserID(r *http.Request) string {
	cookie, err := r.Cookie("pms_session")
	if err != nil {
		return ""
	}
	var id string
	if s.pool.QueryRow(r.Context(), `SELECT u.id::text FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND u.status='active'
AND s.expires_at>now() AND s.absolute_expires_at>now()`, security.TokenHash(cookie.Value)).Scan(&id) != nil {
		return ""
	}
	return id
}

func (s *Server) ssoOIDC(ctx context.Context, p sso.Provider) (sso.OIDC, error) {
	var cfg sso.OIDCConfig
	if err := json.Unmarshal(p.Config, &cfg); err != nil {
		return sso.OIDC{}, err
	}
	secret, err := s.ssoStore().Secret(ctx, p.ID)
	if err != nil {
		return sso.OIDC{}, err
	}
	return sso.OIDC{Config: cfg, Secret: secret, RedirectURL: s.ssoRedirectURL("oidc")}, nil
}

func (s *Server) ssoSetBrowserCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: ssoCookie, Value: value, Path: "/api/v1/auth/sso", HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
}

// ssoStart sends the browser to the identity provider. purpose "link" (query link=1) needs a signed-in user and ties the identity to that user.
func (s *Server) ssoStart(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if !knownSSOKind(kind) {
		writeError(w, http.StatusNotFound, "single sign-on is not available")
		return
	}
	ip := s.clientIP(r)
	if wait := s.publicLimiter.take("sso:" + ip); wait > 0 {
		writeRateLimit(w, wait)
		return
	}
	ctx := r.Context()
	store := s.ssoStore()
	provider, err := store.ByKind(ctx, kind)
	if err != nil || !provider.Enabled {
		writeError(w, http.StatusNotFound, "single sign-on is not available")
		return
	}
	purpose, user := "login", ""
	if r.URL.Query().Get("link") == "1" {
		if user = s.sessionUserID(r); user == "" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		purpose = "link"
	}
	next := localPath(r.URL.Query().Get("next"))
	client, err := s.ssoOIDC(ctx, provider)
	if err != nil {
		slog.Error("sso start", "error", err)
		s.ssoFail(w, r, purpose, "failed", "", kind, nil)
		return
	}
	state, browser, st, err := store.NewState(ctx, provider.ID, purpose, user, next)
	if err != nil {
		slog.Error("sso state", "error", err)
		s.ssoFail(w, r, purpose, "failed", "", kind, nil)
		return
	}
	target, err := client.Begin(ctx, state, st.Nonce, st.Verifier)
	if err != nil {
		slog.Error("sso discovery", "kind", kind, "error", err)
		s.ssoFail(w, r, purpose, "unavailable", "", kind, nil)
		return
	}
	s.ssoSetBrowserCookie(w, browser, int(sso.StateTTL.Seconds()))
	http.Redirect(w, r, target, http.StatusFound)
}

// ssoFail records the failure, forgets the browser cookie and sends the person back with a fixed reason code. The identity provider's
// own error text is never shown or stored: it is attacker-influenced input.
func (s *Server) ssoFail(w http.ResponseWriter, r *http.Request, purpose, reason, userID, kind string, extra map[string]any) {
	details := map[string]any{"reason": reason, "purpose": purpose}
	for k, v := range extra {
		details[k] = v
	}
	s.audit(r, userID, "", auditEvent{Action: auditSSOFailed, TargetType: "sso", TargetID: kind, Details: details})
	s.ssoSetBrowserCookie(w, "", -1)
	home := "/login"
	if purpose == "link" {
		home = "/settings/security"
	}
	http.Redirect(w, r, home+"?sso_error="+url.QueryEscape(reason), http.StatusFound)
}

func (s *Server) newSession(ctx context.Context, userID string) (token string, expires time.Time, err error) {
	plain, hash, err := security.Token()
	if err != nil {
		return "", time.Time{}, err
	}
	now := time.Now()
	_, err = s.pool.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,expires_at,absolute_expires_at) VALUES($1,$2,$3,$4)", hash, userID, now.Add(config.IdleSessionTimeout), now.Add(config.AbsoluteSessionTimeout))
	return plain, now.Add(config.AbsoluteSessionTimeout), err
}

// ssoCallback finishes a sign-in or a link. Order matters: state and browser binding first (before anything is exchanged),
// then the provider's answer, then the link rules, and only then a session.
func (s *Server) ssoCallback(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if !knownSSOKind(kind) {
		writeError(w, http.StatusNotFound, "single sign-on is not available")
		return
	}
	if wait := s.publicLimiter.take("sso:" + s.clientIP(r)); wait > 0 {
		writeRateLimit(w, wait)
		return
	}
	ctx, q, store := r.Context(), r.URL.Query(), s.ssoStore()
	browser, _ := r.Cookie(ssoCookie)
	cookie := ""
	if browser != nil {
		cookie = browser.Value
	}
	st, err := store.TakeState(ctx, q.Get("state"), cookie)
	if err != nil {
		s.ssoFail(w, r, "login", "state", "", kind, nil)
		return
	}
	provider, err := store.ByID(ctx, st.ProviderID)
	if err != nil || !provider.Enabled || provider.Kind != kind {
		s.ssoFail(w, r, st.Purpose, "unavailable", st.UserID, kind, nil)
		return
	}
	if q.Get("error") != "" {
		s.ssoFail(w, r, st.Purpose, "denied", st.UserID, kind, nil)
		return
	}
	if q.Get("code") == "" || len(q.Get("code")) > 2048 {
		s.ssoFail(w, r, st.Purpose, "failed", st.UserID, kind, nil)
		return
	}
	client, err := s.ssoOIDC(ctx, provider)
	if err != nil {
		slog.Error("sso callback", "error", err)
		s.ssoFail(w, r, st.Purpose, "failed", st.UserID, kind, nil)
		return
	}
	id, err := client.Complete(ctx, q.Get("code"), st.Verifier, st.Nonce)
	switch {
	case errors.Is(err, sso.ErrDomain):
		s.ssoFail(w, r, st.Purpose, "domain", st.UserID, kind, nil)
		return
	case err != nil:
		slog.Warn("sso verification failed", "kind", kind, "error", err)
		s.ssoFail(w, r, st.Purpose, "failed", st.UserID, kind, nil)
		return
	}
	if st.Purpose == "link" {
		s.ssoFinishLink(w, r, store, provider, kind, st, id)
		return
	}
	userID, err := store.UserOf(ctx, provider.ID, id.Subject)
	if errors.Is(err, sso.ErrNotFound) {
		userID, err = s.ssoFirstLogin(ctx, store, provider, client.Config, id)
	}
	if err != nil {
		reason := "failed"
		switch {
		case errors.Is(err, errSSOUnlinked):
			reason = "unlinked"
		case errors.Is(err, errSSOUnverified):
			reason = "unverified"
		case errors.Is(err, errSSOInactive):
			reason = "inactive"
		default:
			slog.Error("sso login", "error", err)
		}
		s.ssoFail(w, r, "login", reason, "", kind, map[string]any{"subject": clipSubject(id.Subject)})
		return
	}
	var status string
	if err := s.pool.QueryRow(ctx, "SELECT status::text FROM users WHERE id=$1::uuid", userID).Scan(&status); err != nil || status != "active" {
		s.ssoFail(w, r, "login", "inactive", userID, kind, nil)
		return
	}
	if presented, err := r.Cookie("pms_session"); err == nil { // a fresh sign-in replaces the session the browser held (SEC-04)
		_, _ = s.pool.Exec(ctx, "DELETE FROM sessions WHERE token_hash=$1", security.TokenHash(presented.Value))
	}
	token, expires, err := s.newSession(ctx, userID)
	if err != nil {
		slog.Error("sso session", "error", err)
		s.ssoFail(w, r, "login", "failed", userID, kind, nil)
		return
	}
	_, _ = s.pool.Exec(ctx, "UPDATE users SET last_login_at=now() WHERE id=$1::uuid", userID)
	store.Touch(ctx, provider.ID, id.Subject)
	s.audit(r, userID, "", auditEvent{Action: auditSSOLogin, TargetType: "sso", TargetID: kind})
	s.audit(r, userID, "", auditEvent{Action: auditLogin, TargetType: "user", TargetID: userID, Details: map[string]any{"method": "sso:" + kind}})
	s.setSessionCookie(w, token, expires)
	s.ssoSetBrowserCookie(w, "", -1)
	http.Redirect(w, r, localPath(st.Next), http.StatusFound)
}

func clipSubject(s string) string { return clip(s, 80) }

var (
	errSSOUnlinked   = errors.New("no user is linked to this identity")
	errSSOUnverified = errors.New("the email address is not verified")
	errSSOInactive   = errors.New("the user is not active")
)

// ssoFirstLogin links a first-time identity to the active user with the same verified email, when the provider allows it.
// It never creates a user (ADR-IDP): people arrive by invitation or provisioning.
func (s *Server) ssoFirstLogin(ctx context.Context, store sso.Store, p sso.Provider, cfg sso.OIDCConfig, id sso.Identity) (string, error) {
	if !cfg.AutoLink {
		return "", errSSOUnlinked
	}
	if id.Email == "" || !id.EmailVerified {
		return "", errSSOUnverified
	}
	var userID, status string
	if err := s.pool.QueryRow(ctx, "SELECT id::text, status::text FROM users WHERE lower(email)=$1", id.Email).Scan(&userID, &status); errors.Is(err, pgx.ErrNoRows) {
		return "", errSSOUnlinked
	} else if err != nil {
		return "", err
	}
	if status != "active" {
		return "", errSSOInactive
	}
	if err := store.Link(ctx, p.ID, id.Subject, userID, id.Email); err != nil {
		if errors.Is(err, sso.ErrTaken) {
			return "", errSSOUnlinked
		}
		return "", err
	}
	_ = s.auditIn(ctx, s.pool, nil, userID, "", auditEvent{Action: auditSSOLinked, TargetType: "sso", TargetID: p.Kind, Details: map[string]any{"how": "email"}})
	return userID, nil
}

func (s *Server) ssoFinishLink(w http.ResponseWriter, r *http.Request, store sso.Store, p sso.Provider, kind string, st sso.State, id sso.Identity) {
	ctx := r.Context()
	if st.UserID == "" || s.sessionUserID(r) != st.UserID { // the person who finishes must still be the one who started
		s.ssoFail(w, r, "link", "session", st.UserID, kind, nil)
		return
	}
	if err := store.Link(ctx, p.ID, id.Subject, st.UserID, id.Email); err != nil {
		reason := "failed"
		if errors.Is(err, sso.ErrTaken) {
			reason = "already_linked"
		} else {
			slog.Error("sso link", "error", err)
		}
		s.ssoFail(w, r, "link", reason, st.UserID, kind, nil)
		return
	}
	s.audit(r, st.UserID, "", auditEvent{Action: auditSSOLinked, TargetType: "sso", TargetID: kind, Details: map[string]any{"how": "signed_in"}})
	s.ssoSetBrowserCookie(w, "", -1)
	dest := localPath(st.Next)
	if st.Next == "" || dest == "/" {
		dest = "/settings/security"
	}
	sep := "?"
	if strings.Contains(dest, "?") {
		sep = "&"
	}
	http.Redirect(w, r, dest+sep+"sso=linked", http.StatusFound)
}

// ---- administration ----

type ssoConfigBody struct {
	Name           string   `json:"name"`
	Enabled        bool     `json:"enabled"`
	Issuer         string   `json:"issuer"`
	ClientID       string   `json:"client_id"`
	ClientSecret   *string  `json:"client_secret"` // nil keeps the stored secret
	AutoLink       bool     `json:"auto_link"`
	AllowedDomains []string `json:"allowed_domains"`
}

type ssoConfigView struct {
	Configured     bool       `json:"configured"`
	Kind           string     `json:"kind"`
	Name           string     `json:"name"`
	Enabled        bool       `json:"enabled"`
	Issuer         string     `json:"issuer"`
	ClientID       string     `json:"client_id"`
	SecretSet      bool       `json:"secret_set"`
	AutoLink       bool       `json:"auto_link"`
	AllowedDomains []string   `json:"allowed_domains"`
	RedirectURL    string     `json:"redirect_url"`
	KeyConfigured  bool       `json:"key_configured"`
	UpdatedAt      *time.Time `json:"updated_at"`
}

func (s *Server) ssoView(kind string, p *sso.Provider) ssoConfigView {
	v := ssoConfigView{Kind: kind, Name: "Single sign-on", AutoLink: true, AllowedDomains: []string{}, RedirectURL: s.ssoRedirectURL(kind), KeyConfigured: len(s.cfg.SSOKey) == 32}
	if p == nil {
		return v
	}
	var cfg sso.OIDCConfig
	_ = json.Unmarshal(p.Config, &cfg)
	if cfg.AllowedDomains == nil {
		cfg.AllowedDomains = []string{}
	}
	v.Configured, v.Name, v.Enabled, v.SecretSet, v.UpdatedAt = true, p.Name, p.Enabled, p.SecretSet, &p.UpdatedAt
	v.Issuer, v.ClientID, v.AutoLink, v.AllowedDomains = cfg.Issuer, cfg.ClientID, cfg.AutoLink, cfg.AllowedDomains
	return v
}

func (s *Server) ssoGetConfig(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if !knownSSOKind(kind) {
		writeError(w, http.StatusNotFound, "unknown provider kind")
		return
	}
	p, err := s.ssoStore().ByKind(r.Context(), kind)
	if errors.Is(err, sso.ErrNotFound) {
		writeJSON(w, http.StatusOK, s.ssoView(kind, nil))
		return
	}
	if err != nil {
		slog.Error("sso config", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, s.ssoView(kind, &p))
}

// ssoPutConfig saves the provider. Enabling it first proves that discovery works and that a secret is stored, so a typo cannot
// put a dead button on the sign-in page.
func (s *Server) ssoPutConfig(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if !knownSSOKind(kind) {
		writeError(w, http.StatusNotFound, "unknown provider kind")
		return
	}
	var in ssoConfigBody
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if n := len([]rune(in.Name)); n < 1 || n > 60 {
		writeError(w, http.StatusUnprocessableEntity, "name must be 1 to 60 characters")
		return
	}
	if in.ClientSecret != nil && len(*in.ClientSecret) > 1000 {
		writeError(w, http.StatusUnprocessableEntity, "client_secret is too long")
		return
	}
	cfg := sso.OIDCConfig{Issuer: in.Issuer, ClientID: in.ClientID, AutoLink: in.AutoLink, AllowedDomains: in.AllowedDomains}
	if err := cfg.Validate(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, strings.TrimPrefix(err.Error(), sso.ErrConfig.Error()+": "))
		return
	}
	ctx, store := r.Context(), s.ssoStore()
	secretStored := false
	if existing, err := store.ByKind(ctx, kind); err == nil {
		secretStored = existing.SecretSet
	}
	if in.ClientSecret != nil {
		secretStored = *in.ClientSecret != ""
	}
	if in.Enabled {
		if !secretStored {
			writeError(w, http.StatusUnprocessableEntity, "a client secret is required before the provider can be enabled")
			return
		}
		secret := ""
		if in.ClientSecret != nil {
			secret = *in.ClientSecret
		} else if existing, err := store.ByKind(ctx, kind); err == nil {
			secret, _ = store.Secret(ctx, existing.ID)
		}
		if err := (sso.OIDC{Config: cfg, Secret: secret, RedirectURL: s.ssoRedirectURL(kind)}).Check(ctx); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "the issuer did not answer OpenID Connect discovery")
			return
		}
	}
	p, err := store.Save(ctx, kind, in.Name, cfg, in.ClientSecret, in.Enabled)
	if errors.Is(err, sso.ErrNoKey) {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		slog.Error("sso save", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	sso.Forget(cfg.Issuer)
	s.audit(r, actorOf(r).ID, "", auditEvent{Action: auditSSOConfigured, TargetType: "sso", TargetID: kind,
		Details: map[string]any{"enabled": p.Enabled, "issuer": cfg.Issuer, "client_id": cfg.ClientID, "auto_link": cfg.AutoLink, "allowed_domains": cfg.AllowedDomains, "secret_changed": in.ClientSecret != nil}})
	writeJSON(w, http.StatusOK, s.ssoView(kind, &p))
}

func (s *Server) ssoDeleteConfig(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if !knownSSOKind(kind) {
		writeError(w, http.StatusNotFound, "unknown provider kind")
		return
	}
	ok, err := s.ssoStore().Delete(r.Context(), kind)
	if err != nil {
		slog.Error("sso delete", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "the provider is not configured")
		return
	}
	s.audit(r, actorOf(r).ID, "", auditEvent{Action: auditSSORemoved, TargetType: "sso", TargetID: kind})
	w.WriteHeader(http.StatusNoContent)
}

// ---- the person's own links ----

func (s *Server) myIdentities(w http.ResponseWriter, r *http.Request) {
	list, err := s.ssoStore().Linked(r.Context(), actorOf(r).ID)
	if err != nil {
		slog.Error("identities", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

// unlinkIdentity removes a link. Someone with no password would lock themselves out, so that is refused.
func (s *Server) unlinkIdentity(w http.ResponseWriter, r *http.Request) {
	kind, user := r.PathValue("kind"), actorOf(r).ID
	var hasPassword bool
	if err := s.pool.QueryRow(r.Context(), "SELECT COALESCE(password_hash,'')<>'' FROM users WHERE id=$1::uuid", user).Scan(&hasPassword); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !hasPassword {
		writeError(w, http.StatusConflict, "set a password before unlinking, otherwise you could not sign in")
		return
	}
	ok, err := s.ssoStore().Unlink(r.Context(), user, kind)
	if err != nil {
		slog.Error("unlink", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "no such link")
		return
	}
	s.audit(r, user, "", auditEvent{Action: auditSSOUnlinked, TargetType: "sso", TargetID: kind})
	w.WriteHeader(http.StatusNoContent)
}
