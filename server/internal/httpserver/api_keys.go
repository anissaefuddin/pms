package httpserver

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/jackc/pgx/v5"
)

const keyPrefix = "pmsk_"

// viaKeyKey marks a request authenticated by an API key; such a request acts as the key's owner under policy,
// so a key can never do more than its owner, and "read" keys are further limited to safe methods.
const viaKeyKey contextKey = "via_api_key"

func (s *Server) apiKeyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/api-keys", s.requireUser(s.sessionOnly(s.listAPIKeys)))
	mux.HandleFunc("POST /api/v1/api-keys", s.requireUser(s.sessionOnly(s.createAPIKey)))
	mux.HandleFunc("POST /api/v1/api-keys/{keyId}/rotate", s.requireUser(s.sessionOnly(s.rotateAPIKey)))
	mux.HandleFunc("DELETE /api/v1/api-keys/{keyId}", s.requireUser(s.sessionOnly(s.revokeAPIKey)))
}

// sessionOnly keeps key management (and account changes) out of reach of a key itself.
func (s *Server) sessionOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Value(viaKeyKey) != nil {
			writeError(w, http.StatusForbidden, "an API key cannot manage API keys")
			return
		}
		if t := actorOf(r).Type; t != policy.Admin && t != policy.Regular {
			writeError(w, http.StatusForbidden, "your account type cannot use API keys")
			return
		}
		next(w, r)
	}
}

// keyAllowed: keys never touch sign-in, password, 2FA or profile changes; "read" keys only GET/HEAD.
func keyAllowed(scope string, r *http.Request) bool {
	p := r.URL.Path
	if strings.HasPrefix(p, "/api/v1/auth/") || strings.HasPrefix(p, "/api/v1/me") && r.Method != http.MethodGet {
		return false
	}
	return scope == "write" || r.Method == http.MethodGet || r.Method == http.MethodHead
}

// authenticateKey resolves a Bearer key. ok=false means the response was already written.
func (s *Server) authenticateKey(w http.ResponseWriter, r *http.Request, token string) (context.Context, bool) {
	var id, userID, userType, scope string
	err := s.pool.QueryRow(r.Context(), `SELECT k.id::text,u.id::text,u.user_type::text,k.scope FROM api_keys k JOIN users u ON u.id=k.user_id
 WHERE k.token_hash=$1 AND k.revoked_at IS NULL AND (k.expires_at IS NULL OR k.expires_at>now()) AND u.status='active'`, security.TokenHash(token)).Scan(&id, &userID, &userType, &scope)
	if err != nil {
		if r.Context().Err() == nil {
			status, msg := http.StatusUnauthorized, "invalid API key"
			if err != pgx.ErrNoRows {
				status, msg = 500, "could not check API key"
			}
			writeError(w, status, msg)
		}
		return nil, false
	}
	if !keyAllowed(scope, r) {
		writeError(w, http.StatusForbidden, "this API key's scope does not allow this request")
		return nil, false
	}
	_, _ = s.pool.Exec(r.Context(), "UPDATE api_keys SET last_used_at=now() WHERE id=$1 AND (last_used_at IS NULL OR last_used_at<now()-interval '1 minute')", id)
	ctx := context.WithValue(context.WithValue(r.Context(), userKey, userID), userTypeKey, userType)
	return context.WithValue(ctx, viaKeyKey, true), true
}

type apiKeyBody struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Scope      string     `json:"scope"`
	Prefix     string     `json:"prefix"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT id::text,name,scope,prefix,expires_at,last_used_at,created_at FROM api_keys
 WHERE user_id=$1 AND revoked_at IS NULL ORDER BY created_at DESC,id`, actorOf(r).ID)
	if err != nil {
		writeItemError(w, err)
		return
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[apiKeyBody])
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func newKey() (secret, prefix string, hash []byte, err error) {
	plain, _, err := security.Token()
	if err != nil {
		return "", "", nil, err
	}
	secret = keyPrefix + plain
	return secret, secret[:len(keyPrefix)+6], security.TokenHash(secret), nil
}

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string     `json:"name"`
		Scope     string     `json:"scope"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if !decode(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	switch {
	case name == "" || len(name) > 120:
		writeItemError(w, items.InvalidField{Field: "name", Message: "name must be 1 to 120 characters"})
		return
	case body.Scope != "read" && body.Scope != "write":
		writeItemError(w, items.InvalidField{Field: "scope", Message: "scope must be read or write"})
		return
	case body.ExpiresAt != nil && !body.ExpiresAt.After(time.Now()):
		writeItemError(w, items.InvalidField{Field: "expires_at", Message: "expiry must be in the future"})
		return
	}
	var n int
	if err := s.pool.QueryRow(r.Context(), "SELECT count(*) FROM api_keys WHERE user_id=$1 AND revoked_at IS NULL", actorOf(r).ID).Scan(&n); err != nil {
		writeItemError(w, err)
		return
	}
	if n >= 20 {
		writeItemError(w, items.Invalid("at most 20 active API keys per person; revoke one first"))
		return
	}
	secret, prefix, hash, err := newKey()
	if err != nil {
		writeItemError(w, err)
		return
	}
	var out apiKeyBody
	err = s.pool.QueryRow(r.Context(), `INSERT INTO api_keys(user_id,name,scope,prefix,token_hash,expires_at) VALUES($1,$2,$3,$4,$5,$6)
 RETURNING id::text,name,scope,prefix,expires_at,last_used_at,created_at`, actorOf(r).ID, name, body.Scope, prefix, hash, body.ExpiresAt).
		Scan(&out.ID, &out.Name, &out.Scope, &out.Prefix, &out.ExpiresAt, &out.LastUsedAt, &out.CreatedAt)
	if err != nil {
		writeItemError(w, err)
		return
	}
	s.audit(r, actorOf(r).ID, "", auditEvent{Action: auditAPIKeyCreated, TargetType: "api_key", TargetID: out.ID, Details: map[string]any{"scope": out.Scope, "name": out.Name}})
	writeJSON(w, http.StatusCreated, map[string]any{"key": out, "secret": secret})
}

// rotate replaces the secret in place: the old value stops working at once.
func (s *Server) rotateAPIKey(w http.ResponseWriter, r *http.Request) {
	secret, prefix, hash, err := newKey()
	if err != nil {
		writeItemError(w, err)
		return
	}
	var out apiKeyBody
	err = s.pool.QueryRow(r.Context(), `UPDATE api_keys SET token_hash=$3,prefix=$4,last_used_at=NULL WHERE id::text=$1 AND user_id=$2 AND revoked_at IS NULL
 RETURNING id::text,name,scope,prefix,expires_at,last_used_at,created_at`, r.PathValue("keyId"), actorOf(r).ID, hash, prefix).
		Scan(&out.ID, &out.Name, &out.Scope, &out.Prefix, &out.ExpiresAt, &out.LastUsedAt, &out.CreatedAt)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	s.audit(r, actorOf(r).ID, "", auditEvent{Action: auditAPIKeyRotated, TargetType: "api_key", TargetID: out.ID})
	writeJSON(w, http.StatusOK, map[string]any{"key": out, "secret": secret})
}

func (s *Server) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	tag, err := s.pool.Exec(r.Context(), "UPDATE api_keys SET revoked_at=now() WHERE id::text=$1 AND user_id=$2 AND revoked_at IS NULL", r.PathValue("keyId"), actorOf(r).ID)
	if err != nil {
		writeItemError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	s.audit(r, actorOf(r).ID, "", auditEvent{Action: auditAPIKeyRevoked, TargetType: "api_key", TargetID: r.PathValue("keyId")})
	w.WriteHeader(http.StatusNoContent)
}
