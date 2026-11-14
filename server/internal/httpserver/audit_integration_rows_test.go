package httpserver_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/webhooks"
)

// noSecrets fails when an audit row carries any of the given secret values (token, client secret, signing key).
func noSecrets(t *testing.T, e map[string]any, secrets ...string) {
	t.Helper()
	raw, _ := json.Marshal(e)
	for _, s := range secrets {
		if s != "" && strings.Contains(string(raw), s) {
			t.Fatalf("audit row leaks a secret: %s", raw)
		}
	}
}

// S11.3: every API-key, webhook and OAuth action writes exactly one audit row with actor, target and safe details; refused ones write none.
func TestAuditRowsForIntegrationActions(t *testing.T) {
	old := webhooks.AllowInternal
	t.Cleanup(func() { webhooks.AllowInternal = old })
	webhooks.AllowInternal = true
	f := newFixture(t)
	admin, user, other := f.person("admin"), f.person("regular"), f.person("regular")
	count := func(action string) int {
		return len(f.call(admin, "GET", "/audit-events?limit=50&action="+action, nil).items())
	}
	row := func(action string, p *person, targetType, targetID string) map[string]any {
		t.Helper()
		if n := count(action); n != 1 {
			t.Fatalf("%s: %d rows, want 1", action, n)
		}
		e := lastAudit(t, f, admin, action)
		if e["actor_id"] != p.id || e["target_type"] != targetType || e["target_id"] != targetID {
			t.Fatalf("%s row = %v", action, e)
		}
		return e
	}
	total := func() int { return len(f.call(admin, "GET", "/audit-events?limit=200", nil).items()) }

	// API keys.
	made := f.call(user, "POST", "/api-keys", map[string]any{"name": "CI", "scope": "read"})
	f.want(made, 201, "create key")
	secret, keyID := made.body["secret"].(string), made.body["key"].(map[string]any)["id"].(string)
	e := row("api_key.created", user, "api_key", keyID)
	if d := e["details"].(map[string]any); d["scope"] != "read" || d["name"] != "CI" {
		t.Fatalf("api_key.created details = %v", d)
	}
	noSecrets(t, e, secret)
	rot := f.call(user, "POST", "/api-keys/"+keyID+"/rotate", nil)
	f.want(rot, 200, "rotate key")
	e = row("api_key.rotated", user, "api_key", keyID)
	noSecrets(t, e, rot.body["secret"].(string), secret)
	before := total()
	f.want(f.call(other, "POST", "/api-keys/"+keyID+"/rotate", nil), 404, "rotate foreign key")
	f.want(f.call(other, "DELETE", "/api-keys/"+keyID, nil), 404, "revoke foreign key")
	f.want(f.call(user, "POST", "/api-keys", map[string]any{"name": "", "scope": "read"}), 422, "bad key")
	f.want(f.call(user, "POST", "/api-keys", map[string]any{"name": "x", "scope": "admin"}), 422, "bad scope")
	if total() != before {
		t.Fatal("a refused API-key action wrote an audit row")
	}
	f.want(f.call(user, "DELETE", "/api-keys/"+keyID, nil), 204, "revoke key")
	row("api_key.revoked", user, "api_key", keyID)
	f.want(f.call(user, "DELETE", "/api-keys/"+keyID, nil), 404, "revoke twice")
	if count("api_key.revoked") != 1 {
		t.Fatal("second revoke wrote a row")
	}

	// Webhooks.
	hook := f.call(user, "POST", "/webhooks", map[string]any{"name": "h", "url": "https://example.com/hook", "event_types": []string{"item.created"}})
	f.want(hook, 201, "create webhook")
	hookID, hookSecret := hook.body["id"].(string), hook.body["secret"].(string)
	e = row("webhook.created", user, "webhook", hookID)
	if e["details"].(map[string]any)["url"] != "https://example.com/hook" {
		t.Fatalf("webhook.created details = %v", e["details"])
	}
	noSecrets(t, e, hookSecret)
	f.want(f.call(user, "PATCH", "/webhooks/"+hookID, map[string]any{"enabled": false}), 200, "update webhook")
	noSecrets(t, row("webhook.updated", user, "webhook", hookID), hookSecret)
	before = total()
	f.want(f.call(other, "PATCH", "/webhooks/"+hookID, map[string]any{"enabled": true}), 404, "update foreign webhook")
	f.want(f.call(other, "DELETE", "/webhooks/"+hookID, nil), 404, "delete foreign webhook")
	f.want(f.call(user, "PATCH", "/webhooks/"+hookID, map[string]any{"url": "ftp://x"}), 422, "bad url")
	f.want(f.call(user, "POST", "/webhooks", map[string]any{"name": "h", "url": "https://example.com/x", "event_types": []string{"nope"}}), 422, "bad event")
	if total() != before {
		t.Fatal("a refused webhook action wrote an audit row")
	}
	f.want(f.call(user, "DELETE", "/webhooks/"+hookID, nil), 204, "delete webhook")
	row("webhook.deleted", user, "webhook", hookID)

	// OAuth apps and grants.
	redirect := "https://app.example.com/cb"
	before = total()
	f.want(f.call(user, "POST", "/oauth/clients", map[string]any{"name": "X", "redirect_uris": []string{redirect}}), 403, "member registers app")
	f.want(f.call(admin, "POST", "/oauth/clients", map[string]any{"name": "X", "redirect_uris": []string{"ftp://x"}}), 422, "bad redirect")
	f.want(f.call(other, "DELETE", "/oauth/clients/00000000-0000-0000-0000-000000000001", nil), 403, "member removes app")
	f.want(f.call(admin, "DELETE", "/oauth/clients/00000000-0000-0000-0000-000000000001", nil), 404, "unknown app")
	if total() != before {
		t.Fatal("a refused OAuth action wrote an audit row")
	}
	app := f.call(admin, "POST", "/oauth/clients", map[string]any{"name": "Sync", "redirect_uris": []string{redirect}})
	f.want(app, 201, "create app")
	appID, cid, cs := app.body["id"].(string), app.body["client_id"].(string), app.body["client_secret"].(string)
	e = row("oauth.app_created", admin, "oauth_app", appID)
	if e["details"].(map[string]any)["name"] != "Sync" {
		t.Fatalf("oauth.app_created details = %v", e["details"])
	}
	noSecrets(t, e, cs)

	verifier := strings.Repeat("v", 50)
	sum := sha256.Sum256([]byte(verifier))
	authz := f.call(user, "POST", "/oauth/authorize", map[string]any{"client_id": cid, "redirect_uri": redirect, "scope": "read", "state": "state-1234",
		"code_challenge": base64.RawURLEncoding.EncodeToString(sum[:]), "code_challenge_method": "S256", "approve": true})
	f.want(authz, 200, "approve")
	u, _ := url.Parse(authz.body["redirect_to"].(string))
	tok := f.form("/oauth/token", url.Values{"grant_type": {"authorization_code"}, "code": {u.Query().Get("code")}, "code_verifier": {verifier}, "redirect_uri": {redirect}, "client_id": {cid}, "client_secret": {cs}})
	f.want(tok, 200, "token")
	grants := f.call(user, "GET", "/oauth/grants", nil)
	grantID := grants.items()[0]["id"].(string)
	before = total()
	f.want(f.call(other, "DELETE", "/oauth/grants/"+grantID, nil), 404, "revoke foreign grant")
	if total() != before {
		t.Fatal("a refused grant revoke wrote an audit row")
	}
	f.want(f.call(user, "DELETE", "/oauth/grants/"+grantID, nil), 204, "revoke grant")
	noSecrets(t, row("oauth.grant_revoked", user, "oauth_grant", grantID), cs, tok.body["access_token"].(string), tok.body["refresh_token"].(string), u.Query().Get("code"))
	f.want(f.call(admin, "DELETE", "/oauth/clients/"+appID, nil), 204, "remove app")
	noSecrets(t, row("oauth.app_removed", admin, "oauth_app", appID), cs)

	// The audit page can filter by the three new categories.
	for cat, n := range map[string]int{"api_key": 3, "webhook": 3, "oauth": 3} {
		if got := len(f.call(admin, "GET", "/audit-events?limit=50&category="+cat, nil).items()); got != n {
			t.Fatalf("category %s: %d rows, want %d", cat, got, n)
		}
	}
}
