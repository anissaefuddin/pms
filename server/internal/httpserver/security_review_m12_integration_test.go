package httpserver_test

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// S12.2.3 / SECURITY_REVIEW_M12: regression locks for behaviour the review proved (SR-01..SR-03 fixes included).

// Every operation declared `security: []` refuses a caller who brings no credential: SCIM answers 401 for all fifteen operations (the
// sweep for protected operations does not reach them), the webhook, token, public-link and sign-in operations never answer 5xx
// and only the documented read-only ones answer 2xx.
func TestSecurityReviewPublicOperationsRefuseAnonymousCallers(t *testing.T) {
	f := newFixture(t)
	_, public := contractOps(t)
	if len(public) < 30 {
		t.Fatalf("only %d public operations found", len(public))
	}
	open := map[string]bool{"GET /auth/sso/providers": true, "GET /healthz": true, "GET /readyz": true, "GET /auth/sso/saml/metadata": true}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for i, op := range public {
		path := pathParam.ReplaceAllString(op.path, "00000000-0000-0000-0000-000000000000")
		body := "{}"
		if op.method == "GET" {
			body = ""
		}
		req, _ := http.NewRequest(op.method, f.srv.URL+"/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if strings.HasPrefix(op.path, "/scim/") && i%2 == 0 {
			req.Header.Set("Authorization", "Bearer not-a-real-scim-token") // an invented token is refused like a missing one
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", op.method, op.path, err)
		}
		resp.Body.Close()
		label := op.method + " " + op.path
		switch {
		case strings.HasPrefix(op.path, "/scim/v2/"):
			if resp.StatusCode != 401 {
				t.Errorf("%s without a valid token: %d", label, resp.StatusCode)
			}
		case resp.StatusCode >= 500:
			t.Errorf("%s: %d", label, resp.StatusCode)
		case resp.StatusCode < 300 && !open[label]:
			t.Errorf("%s answered %d to an anonymous caller", label, resp.StatusCode)
		}
	}
}

// SR-01: an API key and an OAuth access token act as a person, so the network allowlist applies to them like to a session.
func TestSecurityReviewAPIKeyAndOAuthTokenHonourTheIPAllowlist(t *testing.T) {
	f := newFixture(t)
	admin, alice := f.person("admin"), f.person("regular")
	here := []string{"X-Forwarded-For", "203.0.113.7"}
	outside := []string{"X-Forwarded-For", "192.0.2.50"}

	key := f.call(alice, "POST", "/api-keys", map[string]any{"name": "CI", "scope": "read"})
	f.want(key, 201, "create key")
	secret := key.body["secret"].(string)
	const redirect = "https://app.example.com/cb"
	client := f.call(admin, "POST", "/oauth/clients", map[string]any{"name": "Sync", "redirect_uris": []string{redirect}})
	f.want(client, 201, "register client")
	cid, csecret := client.body["client_id"].(string), client.body["client_secret"].(string)
	sum := sha256.Sum256([]byte(strings.Repeat("v", 50)))
	approved := f.call(alice, "POST", "/oauth/authorize", map[string]any{"client_id": cid, "redirect_uri": redirect, "scope": "read", "state": "state-1234",
		"code_challenge": base64.RawURLEncoding.EncodeToString(sum[:]), "code_challenge_method": "S256", "approve": true})
	f.want(approved, 200, "approve")
	u, _ := url.Parse(approved.body["redirect_to"].(string))
	tok := f.form("/oauth/token", url.Values{"grant_type": {"authorization_code"}, "code": {u.Query().Get("code")}, "redirect_uri": {redirect},
		"client_id": {cid}, "client_secret": {csecret}, "code_verifier": {strings.Repeat("v", 50)}})
	f.want(tok, 200, "exchange the code")
	access := tok.body["access_token"].(string)

	bearer := func(secret string, extra []string) []string {
		return append([]string{"Authorization", "Bearer " + secret, "Origin", ""}, extra...)
	}
	f.want(f.call(nil, "GET", "/me", nil, bearer(secret, outside)...), 200, "key, no allowlist yet")
	f.want(f.call(nil, "GET", "/spaces", nil, bearer(access, outside)...), 200, "token, no allowlist yet")

	cur := f.call(admin, "GET", "/access-policy", nil, here...)
	f.want(f.call(admin, "PUT", "/access-policy", map[string]any{"ip_allowlist_enabled": true, "ip_allowlist": []string{"203.0.113.7"}}, append([]string{"If-Match", cur.header.Get("ETag")}, here...)...), 200, "turn the allowlist on")

	for name, r := range map[string]reply{
		"key from outside":   f.call(nil, "GET", "/me", nil, bearer(secret, outside)...),
		"token from outside": f.call(nil, "GET", "/spaces", nil, bearer(access, outside)...),
	} {
		if r.status != 403 || r.body["code"] != "ip_not_allowed" {
			t.Errorf("%s: %d %v", name, r.status, r.body)
		}
	}
	f.want(f.call(nil, "GET", "/me", nil, bearer(secret, here)...), 200, "key from an allowed address")
	f.want(f.call(nil, "GET", "/spaces", nil, bearer(access, here)...), 200, "token from an allowed address")
}

// SR-02 and SR-03: every response (API, error, static, public) carries the framing and sniffing protections, and a client-chosen
// X-Request-ID is only echoed when it is a short plain token.
func TestSecurityReviewResponseHeadersAndRequestID(t *testing.T) {
	f := newFixture(t)
	alice := f.person("regular")
	for name, r := range map[string]reply{
		"api":         f.call(alice, "GET", "/me", nil),
		"401":         f.call(nil, "GET", "/me", nil),
		"404":         f.call(nil, "GET", "/public/forms/nope", nil),
		"unknown":     f.call(nil, "GET", "/no-such-route", nil),
		"login error": f.call(nil, "POST", "/auth/login", map[string]any{"email": "a@b.test", "password": "x"}),
	} {
		h := r.header
		if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("X-Frame-Options") != "DENY" ||
			!strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'none'") || h.Get("Referrer-Policy") != "same-origin" {
			t.Errorf("%s: missing protective headers: %v", name, h)
		}
	}
	if id := f.call(nil, "GET", "/me", nil, "X-Request-ID", "trace-1.a_B").header.Get("X-Request-ID"); id != "trace-1.a_B" {
		t.Errorf("a plain id is kept: %q", id)
	}
	for _, bad := range []string{strings.Repeat("a", 65), "a b", "<script>", "a;b"} {
		if id := f.call(nil, "GET", "/me", nil, "X-Request-ID", bad).header.Get("X-Request-ID"); id == bad || len(id) != 16 {
			t.Errorf("%q should be replaced, got %q", bad, id)
		}
	}
}
