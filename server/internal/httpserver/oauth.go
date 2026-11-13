package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Bearer principal: the shared seam between OAuth tokens (S11.3.3) and API keys (S11.3.1).
// A resolver returns (nil, nil) when the token is not its kind, so the next one is tried.
// API keys register theirs with `bearerResolvers = append(bearerResolvers, apiKeyResolver)` in init().
type bearerPrincipal struct {
	UserID, UserType string
	Scopes           []string
}
type bearerResolver func(ctx context.Context, s *Server, token string) (*bearerPrincipal, error)

var bearerResolvers = []bearerResolver{oauthResolver}

// Scope table is shared: "read" allows safe methods, "write" also allows mutations.
// Rights are never copied into the token: every call is still checked against the live user.
var bearerScopes = []string{"read", "write"}

func scopeAllows(scopes []string, method string) bool {
	if method == http.MethodGet || method == http.MethodHead {
		return slices.Contains(scopes, "read") || slices.Contains(scopes, "write")
	}
	return slices.Contains(scopes, "write")
}

// bearerUser handles requests without a session cookie. It returns false after writing the error.
func (s *Server) bearerUser(w http.ResponseWriter, r *http.Request) (*bearerPrincipal, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token == "" {
		writeError(w, 401, "authentication required")
		return nil, false
	}
	// Tokens must not manage sessions, passwords or other apps.
	if p := r.URL.Path; strings.HasPrefix(p, "/api/v1/auth/") || strings.HasPrefix(p, "/api/v1/oauth/") || strings.HasPrefix(p, "/api/v1/api-keys") {
		writeError(w, 403, "not available to tokens")
		return nil, false
	}
	for _, resolve := range bearerResolvers {
		p, err := resolve(r.Context(), s, token)
		if err != nil {
			writeError(w, 500, "could not check token")
			return nil, false
		}
		if p == nil {
			continue
		}
		if !scopeAllows(p.Scopes, r.Method) {
			writeError(w, 403, "token scope does not allow this request")
			return nil, false
		}
		return p, true
	}
	writeError(w, 401, "invalid token")
	return nil, false
}

func oauthResolver(ctx context.Context, s *Server, token string) (*bearerPrincipal, error) {
	if !strings.HasPrefix(token, "pms_oat_") {
		return nil, nil
	}
	p := &bearerPrincipal{}
	err := s.pool.QueryRow(ctx, `SELECT u.id::text,u.user_type::text,g.scopes FROM oauth_grants g
 JOIN users u ON u.id=g.user_id AND u.status='active' JOIN oauth_clients c ON c.id=g.client_uuid AND c.revoked_at IS NULL
 WHERE g.access_hash=$1 AND g.revoked_at IS NULL AND g.access_expires_at>now()`, security.TokenHash(token)).Scan(&p.UserID, &p.UserType, &p.Scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func (s *Server) OAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/oauth/clients", s.requireUser(s.listOAuthClients))
	mux.HandleFunc("POST /api/v1/oauth/clients", s.requireUser(s.createOAuthClient))
	mux.HandleFunc("DELETE /api/v1/oauth/clients/{id}", s.requireUser(s.deleteOAuthClient))
	mux.HandleFunc("GET /api/v1/oauth/authorize", s.requireUser(s.oauthAuthorizeInfo))
	mux.HandleFunc("POST /api/v1/oauth/authorize", s.requireUser(s.oauthAuthorize))
	mux.HandleFunc("GET /api/v1/oauth/grants", s.requireUser(s.listOAuthGrants))
	mux.HandleFunc("DELETE /api/v1/oauth/grants/{id}", s.requireUser(s.revokeOAuthGrant))
	// No requireUser and no Origin check (see csrf): the client authenticates with its secret.
	mux.HandleFunc("POST /api/v1/oauth/token", s.oauthToken)
}

func randomToken(prefix string) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return prefix + base64.RawURLEncoding.EncodeToString(b)
}

func isAdmin(r *http.Request) bool { return r.Context().Value(userTypeKey) == "admin" }

func (s *Server) listOAuthClients(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeError(w, 403, "administrators only")
		return
	}
	rows, err := s.pool.Query(r.Context(), `SELECT id::text,client_id,name,redirect_uris,created_at FROM oauth_clients WHERE revoked_at IS NULL ORDER BY created_at DESC`)
	if err != nil {
		writeError(w, 500, "could not list apps")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, cid, name string
		var uris []string
		var at time.Time
		if err := rows.Scan(&id, &cid, &name, &uris, &at); err != nil {
			writeError(w, 500, "could not list apps")
			return
		}
		out = append(out, map[string]any{"id": id, "client_id": cid, "name": name, "redirect_uris": uris, "created_at": at})
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

// validRedirect: absolute https URL (http only for loopback), no fragment, compared exactly later.
func validRedirect(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Fragment != "" || u.Host == "" || len(raw) > 500 {
		return false
	}
	return u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))
}

func (s *Server) createOAuthClient(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeError(w, 403, "administrators only")
		return
	}
	var body struct {
		Name         string   `json:"name"`
		RedirectURIs []string `json:"redirect_uris"`
	}
	if !decode(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 120 || strings.ContainsRune(name, 0) || len(body.RedirectURIs) < 1 || len(body.RedirectURIs) > 10 {
		writeError(w, 422, "name (1-120 characters) and 1-10 redirect URIs are required")
		return
	}
	for _, u := range body.RedirectURIs {
		if !validRedirect(u) {
			writeError(w, 422, "redirect URIs must be absolute https URLs without a fragment")
			return
		}
	}
	id, clientID, secret := uuid.New(), randomToken("pms_c_"), randomToken("pms_cs_")
	if _, err := s.pool.Exec(r.Context(), `INSERT INTO oauth_clients(id,client_id,name,secret_hash,redirect_uris,created_by) VALUES($1,$2,$3,$4,$5,$6)`,
		id, clientID, name, security.TokenHash(secret), body.RedirectURIs, actorOf(r).ID); err != nil {
		writeError(w, 500, "could not create app")
		return
	}
	// The secret is shown once; only its hash is stored.
	writeJSON(w, 201, map[string]any{"id": id, "client_id": clientID, "client_secret": secret, "name": name, "redirect_uris": body.RedirectURIs})
}

func (s *Server) deleteOAuthClient(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if !isAdmin(r) || err != nil {
		writeError(w, 403, "administrators only")
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "could not remove app")
		return
	}
	defer tx.Rollback(r.Context())
	tag, err := tx.Exec(r.Context(), `UPDATE oauth_clients SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL`, id)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, 404, "app not found")
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE oauth_grants SET revoked_at=now() WHERE client_uuid=$1 AND revoked_at IS NULL`, id); err != nil || tx.Commit(r.Context()) != nil {
		writeError(w, 500, "could not remove app")
		return
	}
	w.WriteHeader(204)
}

type authRequest struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	Approve             bool   `json:"approve"`
}

// checkAuthRequest validates what must be right before anyone is redirected anywhere.
func (s *Server) checkAuthRequest(r *http.Request, q authRequest) (clientUUID uuid.UUID, name string, scopes []string, msg string) {
	var uris []string
	err := s.pool.QueryRow(r.Context(), `SELECT id,name,redirect_uris FROM oauth_clients WHERE client_id=$1 AND revoked_at IS NULL`, q.ClientID).Scan(&clientUUID, &name, &uris)
	if err != nil {
		return uuid.Nil, "", nil, "unknown app"
	}
	if !slices.Contains(uris, q.RedirectURI) { // exact match, no prefix or wildcard
		return uuid.Nil, "", nil, "redirect URI does not match"
	}
	if q.CodeChallengeMethod != "S256" || len(q.CodeChallenge) != 43 || strings.Trim(q.CodeChallenge, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_") != "" {
		return uuid.Nil, "", nil, "PKCE with S256 is required"
	}
	scopes = strings.Fields(q.Scope)
	if len(scopes) == 0 || len(q.State) < 8 || len(q.State) > 500 {
		return uuid.Nil, "", nil, "scope and state are required"
	}
	for _, sc := range scopes {
		if !slices.Contains(bearerScopes, sc) {
			return uuid.Nil, "", nil, "unknown scope"
		}
	}
	return clientUUID, name, scopes, ""
}

func (s *Server) oauthAuthorizeInfo(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	q := authRequest{v.Get("client_id"), v.Get("redirect_uri"), v.Get("scope"), v.Get("state"), v.Get("code_challenge"), v.Get("code_challenge_method"), false}
	_, name, scopes, msg := s.checkAuthRequest(r, q)
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	writeJSON(w, 200, map[string]any{"app_name": name, "scopes": scopes})
}

func (s *Server) oauthAuthorize(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie("pms_session"); err != nil { // consent is a human act, never a token's
		writeError(w, 403, "consent needs a signed-in session")
		return
	}
	var q authRequest
	if !decode(w, r, &q) {
		return
	}
	clientUUID, _, scopes, msg := s.checkAuthRequest(r, q)
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	u, _ := url.Parse(q.RedirectURI)
	args := u.Query()
	args.Set("state", q.State)
	if !q.Approve {
		args.Set("error", "access_denied")
	} else {
		code := randomToken("pms_code_")
		if _, err := s.pool.Exec(r.Context(), `INSERT INTO oauth_codes(code_hash,client_uuid,user_id,redirect_uri,scopes,code_challenge,expires_at) VALUES($1,$2,$3,$4,$5,$6,now()+interval '60 seconds')`,
			security.TokenHash(code), clientUUID, actorOf(r).ID, q.RedirectURI, scopes, q.CodeChallenge); err != nil {
			writeError(w, 500, "could not approve")
			return
		}
		args.Set("code", code)
	}
	u.RawQuery = args.Encode()
	writeJSON(w, 200, map[string]any{"redirect_to": u.String()})
}

func tokenError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]any{"error": code})
}

func (s *Server) oauthToken(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	if err := r.ParseForm(); err != nil {
		tokenError(w, 400, "invalid_request")
		return
	}
	id, secret, basic := r.BasicAuth()
	if !basic {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	var clientUUID uuid.UUID
	var want []byte
	err := s.pool.QueryRow(r.Context(), `SELECT id,secret_hash FROM oauth_clients WHERE client_id=$1 AND revoked_at IS NULL`, id).Scan(&clientUUID, &want)
	if err != nil || subtle.ConstantTimeCompare(want, security.TokenHash(secret)) != 1 {
		tokenError(w, 401, "invalid_client")
		return
	}
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		s.exchangeCode(w, r, clientUUID)
	case "refresh_token":
		s.refreshGrant(w, r, clientUUID)
	default:
		tokenError(w, 400, "unsupported_grant_type")
	}
}

func issue(w http.ResponseWriter, scopes []string, access, refresh string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"access_token": access, "refresh_token": refresh, "token_type": "Bearer", "expires_in": 3600, "scope": strings.Join(scopes, " ")})
}

const (
	accessTTL  = "1 hour"
	refreshTTL = "30 days"
)

func (s *Server) exchangeCode(w http.ResponseWriter, r *http.Request, client uuid.UUID) {
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		tokenError(w, 500, "server_error")
		return
	}
	defer tx.Rollback(ctx)
	var user uuid.UUID
	var redirect, challenge string
	var scopes []string
	var expires time.Time
	var usedAt *time.Time
	var codeClient uuid.UUID
	var oldGrant *uuid.UUID
	hash := security.TokenHash(r.PostForm.Get("code"))
	err = tx.QueryRow(ctx, `SELECT client_uuid,user_id,redirect_uri,scopes,code_challenge,expires_at,used_at,grant_id FROM oauth_codes WHERE code_hash=$1 FOR UPDATE`, hash).
		Scan(&codeClient, &user, &redirect, &scopes, &challenge, &expires, &usedAt, &oldGrant)
	if err != nil || codeClient != client {
		tokenError(w, 400, "invalid_grant")
		return
	}
	if usedAt != nil { // replay: the code is burned and what it produced is revoked
		if oldGrant != nil {
			_, _ = tx.Exec(ctx, `UPDATE oauth_grants SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL`, *oldGrant)
			_ = tx.Commit(ctx)
		}
		tokenError(w, 400, "invalid_grant")
		return
	}
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if time.Now().After(expires) || r.PostForm.Get("redirect_uri") != redirect ||
		subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(sum[:])), []byte(challenge)) != 1 {
		// A wrong verifier or redirect also burns the code: a guesser gets one try.
		_, _ = tx.Exec(ctx, `UPDATE oauth_codes SET used_at=now() WHERE code_hash=$1`, hash)
		_ = tx.Commit(ctx)
		tokenError(w, 400, "invalid_grant")
		return
	}
	access, refresh, grant := randomToken("pms_oat_"), randomToken("pms_ort_"), uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO oauth_grants(id,client_uuid,user_id,scopes,access_hash,refresh_hash,access_expires_at,refresh_expires_at) VALUES($1,$2,$3,$4,$5,$6,now()+interval '`+accessTTL+`',now()+interval '`+refreshTTL+`')`,
		grant, client, user, scopes, security.TokenHash(access), security.TokenHash(refresh)); err != nil {
		tokenError(w, 500, "server_error")
		return
	}
	if _, err = tx.Exec(ctx, `UPDATE oauth_codes SET used_at=now(), grant_id=$2 WHERE code_hash=$1`, hash, grant); err != nil || tx.Commit(ctx) != nil {
		tokenError(w, 500, "server_error")
		return
	}
	issue(w, scopes, access, refresh)
}

func (s *Server) refreshGrant(w http.ResponseWriter, r *http.Request, client uuid.UUID) {
	access, refresh := randomToken("pms_oat_"), randomToken("pms_ort_")
	var scopes []string
	// Rotation in one statement: the old refresh token stops matching the moment this succeeds.
	err := s.pool.QueryRow(r.Context(), `UPDATE oauth_grants g SET access_hash=$3,refresh_hash=$4,access_expires_at=now()+interval '`+accessTTL+`',refresh_expires_at=now()+interval '`+refreshTTL+`'
 FROM users u WHERE u.id=g.user_id AND u.status='active' AND g.client_uuid=$1 AND g.refresh_hash=$2 AND g.revoked_at IS NULL AND g.refresh_expires_at>now() RETURNING g.scopes`,
		client, security.TokenHash(r.PostForm.Get("refresh_token")), security.TokenHash(access), security.TokenHash(refresh)).Scan(&scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		tokenError(w, 400, "invalid_grant")
		return
	}
	if err != nil {
		tokenError(w, 500, "server_error")
		return
	}
	issue(w, scopes, access, refresh)
}

func (s *Server) listOAuthGrants(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT g.id::text,c.name,g.scopes,g.created_at FROM oauth_grants g JOIN oauth_clients c ON c.id=g.client_uuid
 WHERE g.user_id=$1 AND g.revoked_at IS NULL AND g.refresh_expires_at>now() ORDER BY g.created_at DESC`, actorOf(r).ID)
	if err != nil {
		writeError(w, 500, "could not list apps")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name string
		var scopes []string
		var at time.Time
		if err := rows.Scan(&id, &name, &scopes, &at); err != nil {
			writeError(w, 500, "could not list apps")
			return
		}
		out = append(out, map[string]any{"id": id, "app_name": name, "scopes": scopes, "created_at": at})
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (s *Server) revokeOAuthGrant(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "app not found")
		return
	}
	tag, err := s.pool.Exec(r.Context(), `UPDATE oauth_grants SET revoked_at=now() WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL`, id, actorOf(r).ID)
	if err != nil {
		writeError(w, 500, "could not revoke")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 404, "app not found")
		return
	}
	w.WriteHeader(204)
}
