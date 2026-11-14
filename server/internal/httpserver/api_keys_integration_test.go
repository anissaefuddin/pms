package httpserver_test

import (
	"net/http"
	"testing"
	"time"
)

func TestAPIKeysScopeRotateRevokeAndOwnerLimits(t *testing.T) {
	f := newFixture(t)
	owner := f.person("regular")
	other := f.person("regular")
	space := f.space("Keyed")
	f.share(space, "user", owner.id, "full")

	bearer := func(secret string) []string { return []string{"Authorization", "Bearer " + secret, "Origin", ""} }
	f.want(f.call(owner, "POST", "/api-keys", map[string]any{"name": "", "scope": "write"}), 422, "empty name")
	f.want(f.call(owner, "POST", "/api-keys", map[string]any{"name": "x", "scope": "admin"}), 422, "bad scope")
	f.want(f.call(owner, "POST", "/api-keys", map[string]any{"name": "x", "scope": "read", "expires_at": time.Now().Add(-time.Hour)}), 422, "past expiry")

	made := f.call(owner, "POST", "/api-keys", map[string]any{"name": "CI", "scope": "read"})
	f.want(made, 201, "create read key")
	secret := made.body["secret"].(string)
	id := made.body["key"].(map[string]any)["id"].(string)
	if f.queryID("SELECT count(*)::text FROM api_keys WHERE token_hash=sha256(convert_to($1,'UTF8'))", secret) != "1" {
		t.Fatalf("hash of the secret should be stored")
	}
	if list := f.call(owner, "GET", "/api-keys", nil); list.items()[0]["secret"] != nil || len(list.items()) != 1 {
		t.Fatalf("list must not repeat the secret: %v", list.body)
	}
	if got := f.call(other, "GET", "/api-keys", nil); len(got.items()) != 0 {
		t.Fatal("keys are private to their owner")
	}

	// read scope: GET works as the owner (policy still applies), writes are 403.
	f.want(f.call(nil, "GET", "/spaces/"+space, nil, bearer(secret)...), 200, "read with key")
	f.want(f.call(nil, "GET", "/spaces/"+f.space("Hidden"), nil, bearer(secret)...), 404, "key cannot see what its owner cannot")
	f.want(f.call(nil, "PATCH", "/spaces/"+space, map[string]any{"name": "N"}, bearer(secret)...), 403, "read key cannot write")
	f.want(f.call(nil, "POST", "/api-keys", map[string]any{"name": "k", "scope": "read"}, bearer(secret)...), 403, "key cannot manage keys")
	f.want(f.call(nil, "GET", "/me", nil, "Authorization", "Bearer pmsk_bogus"), 401, "unknown key")

	// write scope may write, within the owner's rights.
	w := f.call(owner, "POST", "/api-keys", map[string]any{"name": "W", "scope": "write"})
	wsecret := w.body["secret"].(string)
	if r := f.call(nil, "POST", "/items", map[string]any{"title": "via key", "type": "task", "locations": []map[string]any{{"space_id": space}}}, bearer(wsecret)...); r.status >= 300 {
		t.Fatalf("write key create item: %d %v", r.status, r.body)
	}
	f.want(f.call(nil, "POST", "/auth/password/change", map[string]any{}, bearer(wsecret)...), 403, "key never changes passwords")

	// rotation: old secret dead at once, new one live; only the owner may rotate.
	f.want(f.call(other, "POST", "/api-keys/"+id+"/rotate", nil), 404, "other user rotates")
	rot := f.call(owner, "POST", "/api-keys/"+id+"/rotate", nil)
	f.want(rot, 200, "rotate")
	f.want(f.call(nil, "GET", "/me", nil, bearer(secret)...), 401, "old secret")
	f.want(f.call(nil, "GET", "/me", nil, bearer(rot.body["secret"].(string))...), 200, "new secret")

	// revocation is immediate; owner deactivation also kills keys.
	f.want(f.call(other, "DELETE", "/api-keys/"+id, nil), 404, "other user revokes")
	f.want(f.call(owner, "DELETE", "/api-keys/"+id, nil), 204, "revoke")
	f.want(f.call(nil, "GET", "/me", nil, bearer(rot.body["secret"].(string))...), 401, "revoked")
	f.exec("UPDATE api_keys SET expires_at=now()-interval '1 second' WHERE token_hash=sha256(convert_to($1,'UTF8'))", wsecret)
	f.want(f.call(nil, "GET", "/me", nil, bearer(wsecret)...), 401, "expired")
	_ = http.StatusOK
}
