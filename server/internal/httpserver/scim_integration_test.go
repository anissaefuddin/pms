package httpserver_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const (
	scimUser  = "urn:ietf:params:scim:schemas:core:2.0:User"
	scimGroup = "urn:ietf:params:scim:schemas:core:2.0:Group"
	scimPatch = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
)

// scimCall is the identity provider: a bearer token, a SCIM body, and no cookie and no Origin (a provider's server sends neither).
func (f *fixture) scimCall(token, method, path string, body any) reply {
	f.t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, f.srv.URL+"/api/v1/scim/v2"+path, rdr)
	req.Header.Set("Content-Type", "application/scim+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	out := reply{status: resp.StatusCode, header: resp.Header}
	_ = json.NewDecoder(resp.Body).Decode(&out.body)
	return out
}

func (f *fixture) scimToken(admin *person, name, userType string) string {
	f.t.Helper()
	in := map[string]any{"name": name}
	if userType != "" {
		in["default_user_type"] = userType
	}
	r := f.call(admin, "POST", "/scim-tokens", in)
	f.want(r, 201, "create scim token")
	return r.body["token"].(string)
}

func scimNewUser(email, external string) map[string]any {
	return map[string]any{"schemas": []string{scimUser}, "userName": email, "externalId": external, "active": true,
		"name": map[string]any{"givenName": "Sari", "familyName": "Putri"}, "title": "Analyst"}
}

func scimPatchOps(ops ...map[string]any) map[string]any {
	return map[string]any{"schemas": []string{scimPatch}, "Operations": ops}
}

func resources(r reply) []map[string]any {
	list, _ := r.body["Resources"].([]any)
	out := []map[string]any{}
	for _, v := range list {
		out = append(out, v.(map[string]any))
	}
	return out
}

func TestSCIMTokensAreAdminOnlyHashedAndAudited(t *testing.T) {
	f := newFixture(t)
	admin, regular := f.person("admin"), f.person("regular")

	f.want(f.call(nil, "GET", "/scim-tokens", nil), 401, "anonymous list")
	for _, method := range []string{"GET", "POST"} {
		if r := f.call(regular, method, "/scim-tokens", map[string]any{"name": "x"}); r.status != 403 {
			t.Errorf("a regular user %s tokens = %d", method, r.status)
		}
	}
	for _, bad := range []map[string]any{{"name": ""}, {"name": strings.Repeat("a", 81)}, {"name": "ok", "default_user_type": "admin"}, {"name": "ok", "default_user_type": "root"}} {
		f.want(f.call(admin, "POST", "/scim-tokens", bad), 422, fmt.Sprint(bad))
	}

	created := f.call(admin, "POST", "/scim-tokens", map[string]any{"name": "Okta", "default_user_type": "viewer"})
	f.want(created, 201, "create")
	token := created.body["token"].(string)
	if !strings.HasPrefix(token, "pms_scim_") || created.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("token %q, cache %q", token, created.header.Get("Cache-Control"))
	}
	if n := f.queryID("SELECT count(*)::text FROM scim_tokens WHERE position($1 in encode(token_hash,'hex'))>0 OR prefix=$1", token); n != "0" {
		t.Fatal("the token itself must not be stored")
	}
	list := f.call(admin, "GET", "/scim-tokens", nil)
	f.want(list, 200, "list")
	if strings.Contains(list.raw(), token) || len(list.items()) != 1 || list.items()[0]["token"] != nil || list.items()[0]["prefix"] != token[:13] {
		t.Fatalf("the list must never repeat the token: %s", list.raw())
	}
	if !strings.HasSuffix(list.body["base_url"].(string), "/api/v1/scim/v2") {
		t.Fatalf("base_url = %v", list.body["base_url"])
	}

	f.want(f.scimCall(token, "GET", "/Users", nil), 200, "the token works")
	if f.queryID("SELECT (last_used_at IS NOT NULL)::text FROM scim_tokens") != "true" {
		t.Error("last_used_at must be recorded")
	}
	f.want(f.call(regular, "DELETE", "/scim-tokens/"+created.body["id"].(string), nil), 403, "regular revoke")
	f.want(f.call(admin, "DELETE", "/scim-tokens/"+created.body["id"].(string), nil), 204, "revoke")
	f.want(f.call(admin, "DELETE", "/scim-tokens/"+created.body["id"].(string), nil), 404, "revoke twice")
	f.want(f.call(admin, "DELETE", "/scim-tokens/not-an-id", nil), 404, "bad id")
	f.want(f.scimCall(token, "GET", "/Users", nil), 401, "a revoked token")

	for _, action := range []string{"scim.token_created", "scim.token_revoked"} {
		e := lastAudit(t, f, admin, action)
		if e["actor_id"] != admin.id {
			t.Errorf("%s actor = %v", action, e["actor_id"])
		}
	}
	if strings.Contains(f.call(admin, "GET", "/audit-events?limit=50", nil).raw(), token) {
		t.Fatal("the audit trail must not hold the token")
	}
}

func TestSCIMRefusesAnythingWithoutAValidTokenAndIsRateLimited(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	token := f.scimToken(admin, "Okta", "")
	for _, c := range []struct{ name, token string }{{"no token", ""}, {"wrong token", "pms_scim_" + strings.Repeat("A", 43)}, {"not a scim token", "x"}} {
		r := f.scimCall(c.token, "GET", "/Users", nil)
		if r.status != 401 || r.header.Get("WWW-Authenticate") == "" || r.header.Get("Content-Type") != "application/scim+json" || r.body["status"] != "401" {
			t.Errorf("%s: %d %v", c.name, r.status, r.body)
		}
	}
	// A signed-in administrator's session is not a SCIM credential, and a SCIM token is not a session.
	req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1/scim/v2/Users", nil)
	req.AddCookie(admin.cookie)
	if resp, _ := http.DefaultClient.Do(req); resp.StatusCode != 401 {
		t.Errorf("a session cookie on SCIM = %d", resp.StatusCode)
	}
	if r := f.scimCall(token, "GET", "/Users", nil); r.status != 200 {
		t.Fatalf("a valid token = %d", r.status)
	}
	req, _ = http.NewRequest("GET", f.srv.URL+"/api/v1/users/"+admin.id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if resp, _ := http.DefaultClient.Do(req); resp.StatusCode != 401 {
		t.Errorf("a SCIM token on a normal endpoint = %d", resp.StatusCode)
	}

	for _, discovery := range []string{"/ServiceProviderConfig", "/ResourceTypes", "/Schemas"} {
		f.want(f.scimCall(token, "GET", discovery, nil), 200, discovery)
	}
	if r := f.scimCall(token, "GET", "/ServiceProviderConfig", nil); r.body["patch"].(map[string]any)["supported"] != true {
		t.Errorf("config: %v", r.body)
	}

	limited := 0
	for i := 0; i < 620; i++ {
		if f.scimCall(token, "GET", "/Users/"+"00000000-0000-4000-8000-000000000001", nil).status == 429 {
			limited++
		}
	}
	if limited == 0 {
		t.Fatal("a token that floods the server must be rate limited")
	}
	other := f.scimToken(admin, "Entra", "")
	f.want(f.scimCall(other, "GET", "/Users", nil), 200, "the limit is per token")
}

// E2E-PAR-12: the provider creates the person, the person signs in with single sign-on and works, then the provider deprovisions and
// every way in closes at once; reprovisioning opens it again.
func TestSCIMProvisionSignInDeprovisionAndRestore(t *testing.T) {
	s := newSSOSetup(t)
	f, admin := s.f, s.admin
	token := f.scimToken(admin, "Okta", "")
	seats := func() int {
		r := f.call(admin, "GET", "/seats", nil)
		f.want(r, 200, "seats")
		return int(r.body["used"].(float64))
	}
	before := seats()

	created := f.scimCall(token, "POST", "/Users", scimNewUser("Sari.Putri@Example.com", "okta-1"))
	f.want(created, 201, "provision")
	id := created.body["id"].(string)
	if created.header.Get("Content-Type") != "application/scim+json" || created.header.Get("Location") == "" || created.body["userName"] != "sari.putri@example.com" ||
		created.body["active"] != true || created.body["externalId"] != "okta-1" || created.body["displayName"] != "Sari Putri" {
		t.Fatalf("the created user: %v", created.body)
	}
	if got := f.queryID("SELECT user_type::text||'/'||status::text||'/'||(password_hash IS NULL)::text FROM users WHERE id=$1", id); got != "regular/active/true" {
		t.Fatalf("stored as %s", got)
	}
	if seats() != before+1 {
		t.Fatal("a provisioned user takes a seat")
	}
	if r := f.call(nil, "POST", "/auth/login", map[string]any{"email": "sari.putri@example.com", "password": "anything at all"}); r.status != 401 {
		t.Fatalf("a user with no password cannot sign in with one: %d", r.status)
	}
	if e := lastAudit(t, f, admin, "scim.user_created"); e["actor_name"] != "scim:Okta" || e["actor_id"] != nil || e["target_id"] != id {
		t.Fatalf("audit = %v", e)
	}

	// Single sign-on: the identity provider vouches for the verified email, which links the account.
	b := newBrowser(f)
	if location, cookies := s.signIn(b, "", "sub-sari", "sari.putri@example.com", true); location != "/" || cookies != 1 {
		t.Fatalf("sign-in ended at %q with %d cookies", location, cookies)
	}
	sari := &person{id: id, cookie: &http.Cookie{Name: "pms_session", Value: b.sessionCookie()}}
	if me := f.call(sari, "GET", "/me", nil); me.status != 200 || me.body["id"] != id {
		t.Fatalf("the session works: %v", me)
	}

	// Deprovision (Okta sends DELETE or active=false; Entra sends the string "False").
	for i, off := range []func() reply{
		func() reply { return f.scimCall(token, "DELETE", "/Users/"+id, nil) },
		func() reply {
			return f.scimCall(token, "PATCH", "/Users/"+id, scimPatchOps(map[string]any{"op": "Replace", "path": "active", "value": "False"}))
		},
	} {
		if i == 1 {
			f.want(f.scimCall(token, "PATCH", "/Users/"+id, scimPatchOps(map[string]any{"op": "Replace", "path": "active", "value": "True"})), 200, "reprovision first")
			b = newBrowser(f)
			s.signIn(b, "", "sub-sari", "sari.putri@example.com", true)
			sari.cookie = &http.Cookie{Name: "pms_session", Value: b.sessionCookie()}
		}
		r := off()
		if i == 0 {
			f.want(r, 204, "deprovision by DELETE")
		} else {
			f.want(r, 200, "deprovision by active=false")
			if r.body["active"] != false {
				t.Fatalf("the user is still active: %v", r.body)
			}
		}
		// Looked at before the person makes any request, because a request from a deactivated user would delete the session itself.
		if f.queryID("SELECT count(*)::text FROM sessions WHERE user_id=$1", id) != "0" || f.call(sari, "GET", "/me", nil).status != 401 {
			t.Fatal("deprovisioning must delete the sessions at once, so reprovisioning cannot revive them")
		}
		if seats() != before {
			t.Fatalf("the seat must be freed, used %d want %d", seats(), before)
		}
		if got := f.queryID("SELECT status::text FROM users WHERE id=$1", id); got != "deactivated" {
			t.Fatalf("status %s", got)
		}
		if location, cookies := s.signIn(newBrowser(f), "", "sub-sari", "sari.putri@example.com", true); cookies != 0 || reasonOf(t, location) != "inactive" {
			t.Fatalf("single sign-on must refuse a deprovisioned user, got %q with %d cookies", location, cookies)
		}
		f.want(f.scimCall(token, "DELETE", "/Users/"+id, nil), 204, "deprovisioning twice is harmless")
	}
	if e := lastAudit(t, f, admin, "scim.user_deprovisioned"); e["actor_name"] != "scim:Okta" {
		t.Fatalf("audit = %v", e)
	}

	// Reprovision: same person, same history, a seat again, and single sign-on works.
	f.want(f.scimCall(token, "PATCH", "/Users/"+id, scimPatchOps(map[string]any{"op": "replace", "value": map[string]any{"active": true}})), 200, "reprovision")
	if seats() != before+1 {
		t.Fatal("the seat is taken again")
	}
	if f.call(sari, "GET", "/me", nil).status != 401 {
		t.Fatal("a session from before the deprovisioning must stay dead")
	}
	if location, cookies := s.signIn(newBrowser(f), "", "sub-sari", "sari.putri@example.com", true); location != "/" || cookies != 1 {
		t.Fatalf("sign-in after reprovisioning: %q %d", location, cookies)
	}
	if e := lastAudit(t, f, admin, "scim.user_reactivated"); e["target_id"] != id {
		t.Fatalf("audit = %v", e)
	}
}

func TestSCIMUsersValidationFiltersAndProtectedAccounts(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	token := f.scimToken(admin, "Okta", "viewer")
	scim := func(method, path string, body any) reply { return f.scimCall(token, method, path, body) }

	a := scim("POST", "/Users", scimNewUser("a@example.com", "ext-a"))
	f.want(a, 201, "a")
	b := scim("POST", "/Users", scimNewUser("b@example.com", "ext-b"))
	f.want(b, 201, "b")
	aID, bID := a.body["id"].(string), b.body["id"].(string)
	if f.queryID("SELECT user_type::text FROM users WHERE id=$1", aID) != "viewer" {
		t.Fatal("the token's default type applies")
	}

	// Refusals on create.
	for name, in := range map[string]map[string]any{
		"not an email":   scimNewUser("nobody", ""),
		"bad active":     {"userName": "c@example.com", "active": "maybe"},
		"too long title": {"userName": "c@example.com", "title": strings.Repeat("t", 201)},
	} {
		if r := scim("POST", "/Users", in); r.status != 400 || r.body["scimType"] != "invalidValue" {
			t.Errorf("%s: %d %v", name, r.status, r.body)
		}
	}
	if r := scim("POST", "/Users", scimNewUser("A@example.com", "")); r.status != 409 || r.body["scimType"] != "uniqueness" {
		t.Errorf("a duplicate email: %d %v", r.status, r.body)
	}
	if r := scim("POST", "/Users", scimNewUser("fresh@example.com", "ext-a")); r.status != 409 {
		t.Errorf("a duplicate externalId: %d", r.status)
	}
	if r := f.scimCall(token, "POST", "/Users", "not an object"); r.status != 400 {
		t.Errorf("a body that is not an object: %d", r.status)
	}
	// A provider with one name field still gets an account.
	one := scim("POST", "/Users", map[string]any{"userName": "single@example.com", "displayName": "Cher"})
	f.want(one, 201, "single name")
	// An inactive user is created without taking a seat.
	off := map[string]any{"userName": "off@example.com", "active": false, "name": map[string]any{"givenName": "O", "familyName": "Ff"}}
	r := scim("POST", "/Users", off)
	f.want(r, 201, "inactive")
	if r.body["active"] != false {
		t.Fatalf("created inactive: %v", r.body)
	}

	// Filters, paging, lookups.
	q := func(filter string) reply { return scim("GET", "/Users?filter="+url.QueryEscape(filter), nil) }
	if got := resources(q(`userName eq "B@example.com"`)); len(got) != 1 || got[0]["id"] != bID {
		t.Errorf("userName filter: %v", got)
	}
	if got := resources(q(`externalId eq "ext-a"`)); len(got) != 1 || got[0]["id"] != aID {
		t.Errorf("externalId filter: %v", got)
	}
	if got := resources(q(`id eq "` + aID + `"`)); len(got) != 1 {
		t.Errorf("id filter: %v", got)
	}
	if r := q(`userName eq "nobody@example.com"`); r.status != 200 || len(resources(r)) != 0 || r.body["totalResults"] != float64(0) {
		t.Errorf("no match: %v", r.body)
	}
	for _, bad := range []string{`userName co "a"`, `emails.value eq "a"`, `userName eq a`, `active eq "true"`, `id eq "x" or id eq "y"`, `userName eq "a"; DROP TABLE users`} {
		if r := q(bad); r.status != 400 || r.body["scimType"] != "invalidFilter" {
			t.Errorf("filter %q: %d %v", bad, r.status, r.body)
		}
	}
	all := scim("GET", "/Users", nil)
	if all.body["totalResults"] != float64(6) { // the bootstrap invitation, the administrator, a, b, single, off; never the public-form placeholder
		t.Fatalf("total = %v", all.body["totalResults"])
	}
	for _, u := range resources(all) {
		if u["id"] == "00000000-0000-4000-8000-0000000000e1" {
			t.Fatal("the public-form placeholder must not be listed")
		}
	}
	page := scim("GET", "/Users?startIndex=2&count=2", nil)
	if len(resources(page)) != 2 || page.body["startIndex"] != float64(2) || page.body["itemsPerPage"] != float64(2) || page.body["totalResults"] != float64(6) {
		t.Errorf("paging: %v", page.body)
	}
	if r := scim("GET", "/Users?count=0", nil); len(resources(r)) != 0 || r.body["totalResults"] != float64(6) {
		t.Errorf("count=0: %v", r.body)
	}
	if r := scim("GET", "/Users?count=100000", nil); r.status != 200 || r.body["itemsPerPage"].(float64) > 200 {
		t.Errorf("count is capped: %v", r.body["itemsPerPage"])
	}
	f.want(scim("GET", "/Users/"+aID, nil), 200, "get")
	f.want(scim("GET", "/Users/00000000-0000-4000-8000-000000000042", nil), 404, "unknown id")
	f.want(scim("GET", "/Users/not-an-id", nil), 404, "malformed id")
	f.want(scim("GET", "/Users/00000000-0000-4000-8000-0000000000e1", nil), 404, "the placeholder")

	// Updates: only a real change is written and audited.
	updates := func() string {
		return f.queryID("SELECT count(*)::text FROM audit_events WHERE action='scim.user_updated' AND target_id=$1", aID)
	}
	same := scimNewUser("a@example.com", "ext-a")
	f.want(scim("PUT", "/Users/"+aID, same), 200, "a put that changes nothing")
	if updates() != "0" {
		t.Fatalf("an unchanged PUT must not be audited")
	}
	same["title"], same["userName"] = "Lead", "a.new@example.com"
	same["name"] = map[string]any{"givenName": "Sari", "familyName": "Dewi"}
	put := scim("PUT", "/Users/"+aID, same)
	f.want(put, 200, "put")
	if put.body["title"] != "Lead" || put.body["userName"] != "a.new@example.com" || put.body["name"].(map[string]any)["familyName"] != "Dewi" || updates() != "1" {
		t.Fatalf("put result %v, audits %s", put.body, updates())
	}
	f.want(scim("PUT", "/Users/"+aID, scimNewUser("b@example.com", "ext-a")), 409, "put onto another user's email")
	patch := scim("PATCH", "/Users/"+aID, scimPatchOps(
		map[string]any{"op": "replace", "path": "name.givenName", "value": "Ayu"},
		map[string]any{"op": "add", "value": map[string]any{"title": "Head", "name": map[string]any{"familyName": "Lestari"}, "urn:custom:attr": "ignored"}},
		map[string]any{"op": "remove", "path": "externalId"},
	))
	f.want(patch, 200, "patch")
	if patch.body["name"].(map[string]any)["givenName"] != "Ayu" || patch.body["name"].(map[string]any)["familyName"] != "Lestari" || patch.body["title"] != "Head" || patch.body["externalId"] != nil {
		t.Fatalf("patch result %v", patch.body)
	}
	for name, body := range map[string]any{
		"no schema":       map[string]any{"Operations": []any{map[string]any{"op": "replace", "path": "title", "value": "x"}}},
		"no operations":   scimPatchOps(),
		"unknown op":      scimPatchOps(map[string]any{"op": "move", "path": "title"}),
		"bad boolean":     scimPatchOps(map[string]any{"op": "replace", "path": "active", "value": "perhaps"}),
		"bad userName":    scimPatchOps(map[string]any{"op": "replace", "path": "userName", "value": "no-at-sign"}),
		"pathless scalar": scimPatchOps(map[string]any{"op": "replace", "value": "x"}),
	} {
		if r := scim("PATCH", "/Users/"+aID, body); r.status != 400 {
			t.Errorf("%s: %d %v", name, r.status, r.body)
		}
	}

	// An administrator cannot be changed or switched off by a provider.
	for method, body := range map[string]any{"PUT": scimNewUser(f.email(admin), ""), "PATCH": scimPatchOps(map[string]any{"op": "replace", "path": "active", "value": false}), "DELETE": nil} {
		if r := scim(method, "/Users/"+admin.id, body); r.status != 403 || r.body["scimType"] != "mutability" {
			t.Errorf("%s an administrator: %d %v", method, r.status, r.body)
		}
	}
	if f.call(admin, "GET", "/me", nil).status != 200 {
		t.Fatal("the administrator was affected")
	}
	// Deprovisioned people keep their rows (history), and a pending invitation is not left usable.
	invited := f.queryID("INSERT INTO users(email,user_type,status) VALUES ('pending@example.com','regular','invited') RETURNING id::text")
	f.exec("INSERT INTO user_tokens(user_id,purpose,token_hash,expires_at) VALUES ($1,'invite',decode(repeat('ab',32),'hex'),now()+interval '1 day')", invited)
	f.want(scim("DELETE", "/Users/"+invited, nil), 204, "deprovision a pending invitation")
	if f.queryID("SELECT count(*)::text FROM user_tokens WHERE user_id=$1", invited) != "0" {
		t.Fatal("the invitation link must stop working")
	}
	f.want(scim("DELETE", "/Users/00000000-0000-4000-8000-000000000042", nil), 404, "delete unknown")
}

func TestSCIMStopsAtTheSeatLimit(t *testing.T) {
	f := newFixtureWithSeats(t, 3) // the pending bootstrap invitation, the administrator and one more
	admin := f.person("admin")
	token := f.scimToken(admin, "Okta", "")
	first := f.scimCall(token, "POST", "/Users", scimNewUser("one@example.com", ""))
	f.want(first, 201, "first")
	if r := f.scimCall(token, "POST", "/Users", scimNewUser("two@example.com", "")); r.status != 409 {
		t.Fatalf("a full account must refuse, got %d %v", r.status, r.body)
	}
	if f.queryID("SELECT count(*)::text FROM users WHERE email='two@example.com'") != "0" {
		t.Fatal("a refused user must not exist")
	}
	f.want(f.scimCall(token, "DELETE", "/Users/"+first.body["id"].(string), nil), 204, "free the seat")
	second := f.scimCall(token, "POST", "/Users", scimNewUser("two@example.com", ""))
	f.want(second, 201, "the freed seat is reusable")
	// Reactivating the first needs a seat that is not there.
	if r := f.scimCall(token, "PATCH", "/Users/"+first.body["id"].(string), scimPatchOps(map[string]any{"op": "replace", "path": "active", "value": true})); r.status != 409 {
		t.Fatalf("reactivation with no seat = %d", r.status)
	}
}

func TestSCIMGroupsFollowTheProvider(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	token := f.scimToken(admin, "Okta", "")
	scim := func(method, path string, body any) reply { return f.scimCall(token, method, path, body) }
	u1, u2 := scim("POST", "/Users", scimNewUser("g1@example.com", "")), scim("POST", "/Users", scimNewUser("g2@example.com", ""))
	f.want(u1, 201, "u1")
	f.want(u2, 201, "u2")
	id1, id2 := u1.body["id"].(string), u2.body["id"].(string)
	member := func(ids ...string) []map[string]any {
		out := []map[string]any{}
		for _, id := range ids {
			out = append(out, map[string]any{"value": id})
		}
		return out
	}
	members := func(r reply) []string {
		out := []string{}
		list, _ := r.body["members"].([]any)
		for _, m := range list {
			out = append(out, m.(map[string]any)["value"].(string))
		}
		return out
	}

	g := scim("POST", "/Groups", map[string]any{"schemas": []string{scimGroup}, "displayName": "Engineering", "externalId": "grp-1", "members": member(id1)})
	f.want(g, 201, "create group")
	gid := g.body["id"].(string)
	if g.header.Get("Location") == "" || len(members(g)) != 1 || g.body["externalId"] != "grp-1" {
		t.Fatalf("group %v", g.body)
	}
	// The group is a real group: the application's own listing sees it.
	if f.queryID("SELECT name FROM groups WHERE id=$1", gid) != "Engineering" {
		t.Fatal("the group is stored with the application's groups")
	}

	for name, in := range map[string]map[string]any{
		"no name":         {"displayName": " "},
		"unknown member":  {"displayName": "X", "members": member("00000000-0000-4000-8000-000000000042")},
		"member not uuid": {"displayName": "X", "members": member("nope")},
		"the placeholder": {"displayName": "X", "members": member("00000000-0000-4000-8000-0000000000e1")},
	} {
		if r := scim("POST", "/Groups", in); r.status != 400 {
			t.Errorf("%s: %d %v", name, r.status, r.body)
		}
	}
	if r := scim("POST", "/Groups", map[string]any{"displayName": "engineering"}); r.status != 409 {
		t.Errorf("a duplicate name (any case): %d", r.status)
	}
	if f.queryID("SELECT count(*)::text FROM groups WHERE lower(name)='x'") != "0" {
		t.Fatal("a refused group must not exist")
	}

	if got := resources(scim("GET", "/Groups?filter="+url.QueryEscape(`displayName eq "ENGINEERING"`), nil)); len(got) != 1 || got[0]["id"] != gid {
		t.Errorf("displayName filter: %v", got)
	}
	if got := resources(scim("GET", "/Groups?filter="+url.QueryEscape(`externalId eq "grp-1"`), nil)); len(got) != 1 {
		t.Errorf("externalId filter: %v", got)
	}
	if r := scim("GET", "/Groups?filter="+url.QueryEscape(`members co "x"`), nil); r.status != 400 {
		t.Errorf("unsupported group filter: %d", r.status)
	}
	f.want(scim("GET", "/Groups/"+gid, nil), 200, "get")
	f.want(scim("GET", "/Groups/00000000-0000-4000-8000-000000000042", nil), 404, "unknown")

	// Membership edits in the shapes Okta and Entra send.
	add := scim("PATCH", "/Groups/"+gid, scimPatchOps(map[string]any{"op": "add", "path": "members", "value": member(id2)}))
	f.want(add, 200, "add")
	if len(members(add)) != 2 {
		t.Fatalf("after add: %v", add.body)
	}
	rm := scim("PATCH", "/Groups/"+gid, scimPatchOps(map[string]any{"op": "remove", "path": `members[value eq "` + id1 + `"]`}))
	f.want(rm, 200, "remove by filter path")
	if m := members(rm); len(m) != 1 || m[0] != id2 {
		t.Fatalf("after remove: %v", m)
	}
	rm = scim("PATCH", "/Groups/"+gid, scimPatchOps(map[string]any{"op": "Remove", "path": "members", "value": member(id2)}))
	f.want(rm, 200, "remove by value")
	if len(members(rm)) != 0 {
		t.Fatalf("after remove by value: %v", rm.body)
	}
	rep := scim("PATCH", "/Groups/"+gid, scimPatchOps(map[string]any{"op": "replace", "path": "members", "value": member(id1, id2, id2)}, map[string]any{"op": "replace", "path": "displayName", "value": "Platform"}))
	f.want(rep, 200, "replace")
	if len(members(rep)) != 2 || rep.body["displayName"] != "Platform" {
		t.Fatalf("after replace: %v", rep.body)
	}
	clr := scim("PATCH", "/Groups/"+gid, scimPatchOps(map[string]any{"op": "remove", "path": "members"}))
	f.want(clr, 200, "remove all")
	if len(members(clr)) != 0 {
		t.Fatalf("remove all: %v", clr.body)
	}
	if r := scim("PATCH", "/Groups/"+gid, scimPatchOps(map[string]any{"op": "add", "path": "members", "value": member("00000000-0000-4000-8000-000000000042")})); r.status != 400 {
		t.Errorf("unknown member by patch: %d", r.status)
	}
	if r := scim("PATCH", "/Groups/"+gid, scimPatchOps(map[string]any{"op": "replace", "path": "owner", "value": "x"})); r.status != 400 || r.body["scimType"] != "noTarget" {
		t.Errorf("unknown path: %d %v", r.status, r.body)
	}
	put := scim("PUT", "/Groups/"+gid, map[string]any{"displayName": "Ops", "members": member(id2)})
	f.want(put, 200, "put")
	if put.body["displayName"] != "Ops" || len(members(put)) != 1 || put.body["externalId"] != nil {
		t.Fatalf("put: %v", put.body)
	}
	f.want(scim("PUT", "/Groups/00000000-0000-4000-8000-000000000042", map[string]any{"displayName": "Z"}), 404, "put unknown")

	// Deleting a group takes its shares with it, like the administrator's own delete does.
	space := f.space("Shared with ops")
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('space',$1,'group',$2,'read')", space, gid)
	f.want(scim("DELETE", "/Groups/"+gid, nil), 204, "delete")
	f.want(scim("DELETE", "/Groups/"+gid, nil), 404, "delete twice")
	if f.queryID("SELECT count(*)::text FROM shares WHERE principal_id=$1", gid) != "0" || f.queryID("SELECT count(*)::text FROM groups WHERE id=$1", gid) != "0" {
		t.Fatal("the group and its shares must be gone")
	}
	for _, action := range []string{"scim.group_created", "scim.group_updated", "scim.group_deleted"} {
		if e := lastAudit(t, f, admin, action); e["actor_name"] != "scim:Okta" {
			t.Errorf("%s: %v", action, e)
		}
	}
}
