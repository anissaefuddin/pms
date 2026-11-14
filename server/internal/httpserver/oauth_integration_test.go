package httpserver_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func (f *fixture) form(path string, v url.Values) reply {
	f.t.Helper()
	resp, err := http.Post(f.srv.URL+"/api/v1"+path, "application/x-www-form-urlencoded", strings.NewReader(v.Encode()))
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	out := reply{status: resp.StatusCode}
	_ = json.NewDecoder(resp.Body).Decode(&out.body)
	return out
}

func TestOAuthCodePKCEAndRevocation(t *testing.T) {
	f := newFixture(t)
	admin, user := f.person("admin"), f.person("regular")
	const redirect = "https://app.example.com/cb"
	made := f.call(admin, "POST", "/oauth/clients", map[string]any{"name": "Sync", "redirect_uris": []string{redirect}})
	f.want(made, 201, "create client")
	cid, secret := made.body["client_id"].(string), made.body["client_secret"].(string)
	f.want(f.call(user, "POST", "/oauth/clients", map[string]any{"name": "X", "redirect_uris": []string{redirect}}), 403, "members cannot register")
	f.want(f.call(admin, "POST", "/oauth/clients", map[string]any{"name": "X", "redirect_uris": []string{"ftp://x"}}), 422, "bad redirect")
	if list := f.call(admin, "GET", "/oauth/clients", nil); list.items()[0]["client_secret"] != nil {
		t.Fatal("secret must be shown once")
	}
	verifier := strings.Repeat("v", 50)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	authz := func(over map[string]any) reply {
		b := map[string]any{"client_id": cid, "redirect_uri": redirect, "scope": "read write", "state": "state-1234", "code_challenge": challenge, "code_challenge_method": "S256", "approve": true}
		for k, v := range over {
			b[k] = v
		}
		return f.call(user, "POST", "/oauth/authorize", b)
	}
	code := func() string {
		r := authz(nil)
		f.want(r, 200, "approve")
		u, _ := url.Parse(r.body["redirect_to"].(string))
		if u.Query().Get("state") != "state-1234" {
			t.Fatal("state not echoed")
		}
		return u.Query().Get("code")
	}
	f.want(authz(map[string]any{"redirect_uri": redirect + "/x"}), 400, "redirect must match exactly")
	f.want(authz(map[string]any{"code_challenge_method": "plain"}), 400, "S256 only")
	f.want(authz(map[string]any{"scope": "admin"}), 400, "unknown scope")
	deny := authz(map[string]any{"approve": false})
	if !strings.Contains(deny.body["redirect_to"].(string), "access_denied") {
		t.Fatal("denial must say access_denied")
	}
	exchange := func(c, v, ru string) reply {
		return f.form("/oauth/token", url.Values{"grant_type": {"authorization_code"}, "code": {c}, "code_verifier": {v}, "redirect_uri": {ru}, "client_id": {cid}, "client_secret": {secret}})
	}
	f.want(f.form("/oauth/token", url.Values{"grant_type": {"authorization_code"}, "code": {code()}, "code_verifier": {verifier}, "redirect_uri": {redirect}, "client_id": {cid}, "client_secret": {"nope"}}), 401, "bad secret")
	f.want(exchange(code(), "wrong-verifier", redirect), 400, "wrong PKCE verifier")
	f.want(exchange(code(), verifier, redirect+"/other"), 400, "token redirect mismatch")
	c := code()
	tok := exchange(c, verifier, redirect)
	f.want(tok, 200, "exchange")
	f.want(exchange(c, verifier, redirect), 400, "code is single use")
	access, refresh := tok.body["access_token"].(string), tok.body["refresh_token"].(string)
	f.want(f.call(nil, "GET", "/oauth/grants", nil, "Authorization", "Bearer "+access), 403, "tokens cannot manage apps")
	// The replay above revoked the grant, so use a fresh one for the rest.
	f.want(f.call(nil, "GET", "/notifications", nil, "Authorization", "Bearer "+access), 401, "replay revokes what the code made")
	tok = exchange(code(), verifier, redirect)
	access, refresh = tok.body["access_token"].(string), tok.body["refresh_token"].(string)
	bearer := func(tk, method, path string) reply {
		return f.call(nil, method, path, nil, "Authorization", "Bearer "+tk)
	}
	f.want(bearer(access, "GET", "/notifications"), 200, "bearer reads as the user")
	f.want(bearer(access, "POST", "/notifications/read-all"), 204, "write scope mutates")

	// Refresh rotates; the old pair stops working.
	next := f.form("/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {cid}, "client_secret": {secret}})
	f.want(next, 200, "refresh")
	f.want(bearer(access, "GET", "/notifications"), 401, "old access token dies on rotation")
	f.want(f.form("/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {cid}, "client_secret": {secret}}), 400, "old refresh token dies")
	access, refresh = next.body["access_token"].(string), next.body["refresh_token"].(string)

	// Read-only grants cannot write.
	ro := f.call(user, "POST", "/oauth/authorize", map[string]any{"client_id": cid, "redirect_uri": redirect, "scope": "read", "state": "state-1234", "code_challenge": challenge, "code_challenge_method": "S256", "approve": true})
	u, _ := url.Parse(ro.body["redirect_to"].(string))
	rt := exchange(u.Query().Get("code"), verifier, redirect).body["access_token"].(string)
	f.want(bearer(rt, "GET", "/notifications"), 200, "read scope reads")
	f.want(bearer(rt, "POST", "/notifications/read-all"), 403, "read scope cannot write")
	f.want(bearer(rt, "GET", "/users/types"), 403, "member token has no admin rights")

	// Revocation is immediate for both token kinds.
	grants := f.call(user, "GET", "/oauth/grants", nil)
	f.want(grants, 200, "authorized apps")
	if len(grants.items()) != 2 || len(f.call(admin, "GET", "/oauth/grants", nil).items()) != 0 {
		t.Fatalf("grants = %v", grants.body)
	}
	f.want(f.call(admin, "DELETE", "/oauth/grants/"+grants.items()[0]["id"].(string), nil), 404, "other user's grant")
	for _, g := range grants.items() {
		f.want(f.call(user, "DELETE", "/oauth/grants/"+g["id"].(string), nil), 204, "revoke")
	}
	f.want(bearer(access, "GET", "/notifications"), 401, "revoked access token")
	f.want(f.form("/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {cid}, "client_secret": {secret}}), 400, "revoked refresh token")

	// Removing the app kills its grants too.
	tok = exchange(code(), verifier, redirect)
	a2 := tok.body["access_token"].(string)
	f.want(f.call(admin, "DELETE", "/oauth/clients/"+made.body["id"].(string), nil), 204, "remove app")
	f.want(bearer(a2, "GET", "/notifications"), 401, "removed app token")
}
