package httpserver_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/sso"
)

type ssoSetup struct {
	f     *fixture
	idp   *fakeIdP
	admin *person
}

func newSSOSetup(t *testing.T) *ssoSetup {
	t.Helper()
	f := newFixture(t)
	s := &ssoSetup{f: f, idp: newFakeIdP(t), admin: f.person("admin")}
	s.put(map[string]any{})
	return s
}

// put saves the provider, filling in the fixture's identity provider; fields in over replace the defaults.
func (s *ssoSetup) put(over map[string]any) reply {
	body := map[string]any{"name": "Company login", "enabled": true, "issuer": s.idp.srv.URL, "client_id": s.idp.clientID, "client_secret": s.idp.secret,
		"auto_link": true, "allowed_domains": []string{}}
	for k, v := range over {
		body[k] = v
	}
	r := s.f.call(s.admin, "PUT", "/sso/oidc", body)
	if len(over) == 0 {
		s.f.want(r, 200, "configure sso")
	}
	return r
}

// signIn runs the whole browser flow and returns where the callback sent the browser and the browser used.
func (s *ssoSetup) signIn(b *browserSession, next, sub, email string, verified bool) (location string, cookies int) {
	s.f.t.Helper()
	target := "/auth/sso/oidc/start"
	if next != "" {
		target += "?next=" + url.QueryEscape(next)
	}
	status, auth, _ := b.get("/api/v1" + target)
	if status != 302 {
		s.f.t.Fatalf("start = %d", status)
	}
	back := s.idp.approve(auth, sub, email, "Test Person", verified)
	status, location, set := b.get(back)
	if status != 302 {
		s.f.t.Fatalf("callback = %d", status)
	}
	for _, c := range set {
		if c.Name == "pms_session" && c.Value != "" {
			cookies++
		}
	}
	return location, cookies
}

func reasonOf(t *testing.T, location string) string {
	t.Helper()
	u, err := url.Parse(location)
	if err != nil || (u.Path != "/login" && u.Path != "/settings/security") {
		t.Fatalf("not an SSO error redirect: %q", location)
	}
	return u.Query().Get("sso_error")
}

func TestSSOConfigurationAdminOnlyEncryptedAndAudited(t *testing.T) {
	f := newFixture(t)
	idp := newFakeIdP(t)
	admin, regular := f.person("admin"), f.person("regular")
	body := func(over map[string]any) map[string]any {
		b := map[string]any{"name": "Company login", "enabled": false, "issuer": idp.srv.URL, "client_id": idp.clientID, "client_secret": idp.secret, "auto_link": true, "allowed_domains": []string{}}
		for k, v := range over {
			b[k] = v
		}
		return b
	}
	f.want(f.call(regular, "GET", "/sso/oidc", nil), 403, "regular reads")
	f.want(f.call(regular, "PUT", "/sso/oidc", body(nil)), 403, "regular writes")
	f.want(f.call(regular, "DELETE", "/sso/oidc", nil), 403, "regular deletes")
	f.want(f.call(nil, "GET", "/sso/oidc", nil), 401, "anonymous")
	f.want(f.call(admin, "GET", "/sso/ldap", nil), 404, "unknown kind")
	view := f.call(admin, "GET", "/sso/oidc", nil)
	f.want(view, 200, "empty config")
	if view.body["configured"] != false || view.body["key_configured"] != true || !strings.HasSuffix(view.body["redirect_url"].(string), "/api/v1/auth/sso/oidc/callback") {
		t.Fatalf("empty view = %v", view.body)
	}
	if r := f.call(nil, "GET", "/auth/sso/providers", nil); r.status != 200 || len(r.items()) != 0 {
		t.Fatalf("nothing is offered before configuration: %+v", r)
	}

	// Validation. Outside tests the issuer must be https and public.
	sso.AllowInsecure = false
	for name, issuer := range map[string]string{"plain http": "http://idp.example.com", "localhost": "https://localhost", "loopback": "https://127.0.0.1", "private": "https://10.1.2.3",
		"link-local": "https://169.254.169.254", "internal name": "https://metadata.internal", "credentials": "https://user:pw@idp.example.com", "query": "https://idp.example.com/?x=1", "not a url": "idp"} {
		f.want(f.call(admin, "PUT", "/sso/oidc", body(map[string]any{"issuer": issuer})), 422, name)
	}
	sso.AllowInsecure = true
	for name, over := range map[string]map[string]any{
		"blank name":       {"name": "  "},
		"no client id":     {"client_id": ""},
		"bad domain":       {"allowed_domains": []string{"a@b.com"}},
		"enable no secret": {"enabled": true, "client_secret": ""},
		"dead issuer":      {"enabled": true, "issuer": "http://127.0.0.1:1"},
	} {
		f.want(f.call(admin, "PUT", "/sso/oidc", body(over)), 422, name)
	}
	f.want(f.call(admin, "PUT", "/sso/oidc", map[string]any{"name": "x", "extra": 1}), 400, "unknown field")

	// A disabled provider can be saved, and is not offered. Enabling proves discovery works.
	f.want(f.call(admin, "PUT", "/sso/oidc", body(nil)), 200, "save disabled")
	if r := f.call(nil, "GET", "/auth/sso/providers", nil); len(r.items()) != 0 {
		t.Fatal("a disabled provider must not be offered")
	}
	saved := f.call(admin, "PUT", "/sso/oidc", body(map[string]any{"enabled": true, "client_secret": nil}))
	f.want(saved, 200, "enable keeping the stored secret")
	if saved.body["secret_set"] != true || saved.body["enabled"] != true {
		t.Fatalf("view = %v", saved.body)
	}
	if _, leaked := saved.body["client_secret"]; leaked || strings.Contains(strings.ToLower(f.call(admin, "GET", "/sso/oidc", nil).raw()), strings.ToLower(idp.secret)) {
		t.Fatal("the client secret must never be returned")
	}
	if stored := f.queryID("SELECT secret_enc FROM sso_providers"); !strings.HasPrefix(stored, "v1:") || strings.Contains(stored, idp.secret) {
		t.Fatalf("the secret must be stored encrypted, got %q", stored)
	}
	if list := f.call(nil, "GET", "/auth/sso/providers", nil).items(); len(list) != 1 || list[0]["kind"] != "oidc" || list[0]["name"] != "Company login" || len(list[0]) != 2 {
		t.Fatalf("providers = %v", list)
	}
	// The trail records the change without the secret.
	e := lastAudit(t, f, admin, "sso.configured")
	if strings.Contains(strings.ToLower(jsonString(e)), strings.ToLower(idp.secret)) || e["details"].(map[string]any)["issuer"] != idp.srv.URL || e["details"].(map[string]any)["secret_changed"] != false {
		t.Fatalf("audit entry = %v", e)
	}
	// Clearing the secret while enabled is refused; deleting removes the provider and its links.
	f.want(f.call(admin, "PUT", "/sso/oidc", body(map[string]any{"enabled": true, "client_secret": ""})), 422, "enabled without a secret")
	f.want(f.call(admin, "DELETE", "/sso/oidc", nil), 204, "delete")
	f.want(f.call(admin, "DELETE", "/sso/oidc", nil), 404, "delete twice")
	if lastAudit(t, f, admin, "sso.removed")["target_id"] != "oidc" {
		t.Fatal("removal must be audited")
	}
	if list := f.call(nil, "GET", "/auth/sso/providers", nil).items(); len(list) != 0 {
		t.Fatal("a removed provider must not be offered")
	}
}

func TestSSOSignInLinksBySubjectAndKeepsPasswordLogin(t *testing.T) {
	s := newSSOSetup(t)
	f := s.f
	alice := f.withPassword(f.person("regular"))
	email := f.email(alice)

	b := newBrowser(f)
	location, cookies := s.signIn(b, "", "sub-alice", strings.ToUpper(email), true) // email case differs at the provider
	if location != "/" || cookies != 1 {
		t.Fatalf("sign-in ended at %q with %d session cookies", location, cookies)
	}
	if got := f.queryID("SELECT user_id::text FROM user_identities WHERE subject='sub-alice'"); got != alice.id {
		t.Fatalf("identity linked to %s", got)
	}
	me := f.call(&person{cookie: &http.Cookie{Name: "pms_session", Value: b.sessionCookie()}}, "GET", "/me", nil)
	if me.status != 200 || me.body["id"] != alice.id {
		t.Fatalf("the session must belong to alice: %+v", me)
	}
	for _, want := range []string{"sso.linked", "sso.login", "auth.login"} {
		if lastAudit(t, f, s.admin, want)["actor_id"] != alice.id {
			t.Errorf("%s was not recorded for alice", want)
		}
	}
	if d := lastAudit(t, f, s.admin, "auth.login")["details"].(map[string]any); d["method"] != "sso:oidc" {
		t.Fatalf("login method = %v", d)
	}

	// The next sign-in is matched by subject: a changed email at the provider moves nothing.
	if location, _ := s.signIn(newBrowser(f), "", "sub-alice", "someone.else@example.org", false); location != "/" {
		t.Fatalf("a linked identity signs in whatever its email claims, got %q", location)
	}
	other := f.person("regular")
	if location, _ := s.signIn(newBrowser(f), "", "sub-mallory", f.email(other), false); reasonOf(t, location) != "unverified" {
		t.Fatalf("an unverified email must not link, got %q", location)
	}

	// next is honoured only for paths on this site.
	if location, _ := s.signIn(newBrowser(f), "/spaces/abc?view=1", "sub-alice", email, true); location != "/spaces/abc?view=1" {
		t.Fatalf("next = %q", location)
	}
	for _, evil := range []string{"//evil.example/x", "https://evil.example/x", `/\evil.example`, "javascript:alert(1)", "/a\r\nSet-Cookie: x=y", strings.Repeat("/a", 400)} {
		if location, _ := s.signIn(newBrowser(f), evil, "sub-alice", email, true); location != "/" {
			t.Errorf("next %q led to %q", evil, location)
		}
	}

	// A browser that held a session gets a new one: the old token stops working.
	held := newBrowser(f).use(alice)
	old := held.sessionCookie()
	s.signIn(held, "", "sub-alice", email, true)
	if held.sessionCookie() == old || f.call(alice, "GET", "/me", nil).status != 401 {
		t.Fatal("signing in must replace the session the browser held")
	}

	// Local sign-in keeps working while SSO is on, and two-factor still guards it (SSO is the identity provider's job).
	if r := f.loginAs(email); r.status != 200 {
		t.Fatalf("password login = %d", r.status)
	}
	f.want(s.put(map[string]any{"enabled": false}), 200, "turn off")
	if status, _, _ := newBrowser(f).get("/api/v1/auth/sso/oidc/start"); status != 404 {
		t.Fatalf("a disabled provider cannot be started, got %d", status)
	}
	if status, _, _ := newBrowser(f).get("/api/v1/auth/sso/ldap/start"); status != 404 {
		t.Fatalf("an unknown kind = %d", status)
	}
}

// Every refusal ends the same way: back at the sign-in page with a fixed reason, no session, and a record in the trail.
func TestSSORefusesEverythingThatIsNotAProvenActiveUser(t *testing.T) {
	s := newSSOSetup(t)
	f := s.f
	alice := f.person("regular")
	email := f.email(alice)
	deactivated := f.person("regular")
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", deactivated.id)
	invited := f.person("regular")
	f.exec("UPDATE users SET status='invited', password_hash=NULL WHERE id=$1", invited.id)
	users := f.queryID("SELECT count(*)::text FROM users")

	refused := func(name, reason string, b *browserSession, location string, cookies int) {
		t.Helper()
		if got := reasonOf(t, location); got != reason {
			t.Errorf("%s: reason %q, want %q", name, got, reason)
		}
		if cookies != 0 || b.sessionCookie() != "" {
			t.Errorf("%s: a session was started", name)
		}
		if d := lastAudit(t, f, s.admin, "sso.login_failed")["details"].(map[string]any); d["reason"] != reason {
			t.Errorf("%s: audit reason %v, want %q", name, d["reason"], reason)
		}
	}
	run := func(name, reason string, sub, mail string, verified bool) {
		t.Helper()
		b := newBrowser(f)
		location, cookies := s.signIn(b, "", sub, mail, verified)
		refused(name, reason, b, location, cookies)
	}

	run("unverified email", "unverified", "u1", email, false)
	run("no such user, and none is created", "unlinked", "u2", "nobody@example.org", true)
	run("deactivated user", "inactive", "u3", f.email(deactivated), true)
	run("invited user", "inactive", "u4", f.email(invited), true)
	if f.queryID("SELECT count(*)::text FROM users") != users {
		t.Fatal("signing in must never create a user")
	}
	if f.queryID("SELECT count(*)::text FROM user_identities") != "0" {
		t.Fatal("a refused sign-in must not link anything")
	}
	f.want(s.put(map[string]any{"allowed_domains": []string{"corp.example"}}), 200, "restrict domains")
	run("domain not allowed", "domain", "u5", email, true)
	f.want(s.put(map[string]any{"auto_link": false}), 200, "no auto link")
	run("auto link off", "unlinked", "u6", email, true)
	f.want(s.put(map[string]any{}), 200, "back to defaults")

	// A subject that was linked keeps working until the user is deactivated, then stops, whatever the provider says.
	s.signIn(newBrowser(f), "", "u7", email, true)
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", alice.id)
	run("linked user deactivated", "inactive", "u7", email, true)
	f.exec("UPDATE users SET status='active' WHERE id=$1", alice.id)

	// The flow itself.
	start := func(b *browserSession) string {
		t.Helper()
		status, auth, _ := b.get("/api/v1/auth/sso/oidc/start")
		if status != 302 {
			t.Fatalf("start = %d", status)
		}
		return auth
	}
	before := s.idp.exchanges()
	b := newBrowser(f)
	_, auth0, startCookies := b.get("/api/v1/auth/sso/oidc/start")
	back := s.idp.approve(auth0, "u8", email, "Test", true)
	status, location, _ := b.get(back)
	if status != 302 || location != "/" {
		t.Fatalf("a normal flow must work, got %d %q", status, location)
	}
	// Replaying the callback: the state is spent. No exchange happens for any state failure.
	mid := s.idp.exchanges()
	firstSession := b.sessionCookie()
	bu, _ := url.Parse(f.srv.URL)
	for _, c := range startCookies { // the browser still has the cookie that started the flow: the spent state itself must be refused
		if c.Name == "pms_sso" {
			b.jar.SetCookies(bu, []*http.Cookie{{Name: "pms_sso", Value: c.Value, Path: "/api/v1/auth/sso"}})
		}
	}
	_, location, cookies := b.get(back)
	if reasonOf(t, location) != "state" || sessionCount(cookies) != 0 || b.sessionCookie() != firstSession {
		t.Errorf("a replayed callback must be refused without a new session, got %q", location)
	}
	if d := lastAudit(t, f, s.admin, "sso.login_failed")["details"].(map[string]any); d["reason"] != "state" {
		t.Errorf("replay audit reason %v", d["reason"])
	}
	if strings.Contains(location, "sso=") {
		t.Fatal("unexpected link marker")
	}
	for name, mutate := range map[string]func(string) string{
		"unknown state": func(u string) string { return strings.Replace(u, "state=", "state=x", 1) },
		"no state":      func(u string) string { return strings.Split(u, "&state=")[0] },
	} {
		bb := newBrowser(f)
		a := s.idp.approve(start(bb), "u9", email, "Test", true)
		_, location, c := bb.get(mutate(a))
		refused(name, "state", bb, location, sessionCount(c))
	}
	bb := newBrowser(f) // another browser, without the cookie that started the flow
	other := newBrowser(f)
	a := s.idp.approve(start(bb), "u9", email, "Test", true)
	_, location, c := other.get(a)
	refused("state used in another browser", "state", other, location, sessionCount(c))
	expired := newBrowser(f)
	a = s.idp.approve(start(expired), "u9", email, "Test", true)
	f.exec("UPDATE sso_states SET expires_at = now() - interval '1 second'")
	_, location, c = expired.get(a)
	refused("expired state", "state", expired, location, sessionCount(c))
	if s.idp.exchanges() != mid {
		t.Fatal("no code may be exchanged when the state is wrong")
	}
	_ = before

	// The provider's answer: errors, no code, a broken token endpoint, and every way an ID token can be wrong.
	callback := func(name, reason string, mutate func(string) string) {
		t.Helper()
		bk := newBrowser(f)
		auth := start(bk)
		u := s.idp.approve(auth, "u10", email, "Test", true)
		_, location, c := bk.get(mutate(u))
		refused(name, reason, bk, location, sessionCount(c))
	}
	callback("provider error", "denied", func(u string) string {
		q, _ := url.Parse(u)
		return q.Path + "?" + url.Values{"error": {"access_denied"}, "state": {q.Query().Get("state")}}.Encode()
	})
	callback("no code", "failed", func(u string) string { return strings.Replace(u, "code=", "nocode=", 1) })
	callback("empty code", "failed", func(u string) string { return strings.Replace(u, "code=", "code=&x=", 1) })
	callback("unknown code", "failed", func(u string) string {
		q, _ := url.Parse(u)
		v := q.Query()
		v.Set("code", "forged")
		return q.Path + "?" + v.Encode()
	})
	s.idp.mu.Lock()
	s.idp.reject = 500
	s.idp.mu.Unlock()
	callback("token endpoint down", "failed", func(u string) string { return u })
	s.idp.mu.Lock()
	s.idp.reject = 0
	s.idp.mu.Unlock()
	now := time.Now()
	for name, tweak := range map[string]func(map[string]any){
		"nonce mismatch":   func(c map[string]any) { c["nonce"] = "another-nonce" },
		"no nonce":         func(c map[string]any) { delete(c, "nonce") },
		"wrong audience":   func(c map[string]any) { c["aud"] = "someone-else" },
		"wrong issuer":     func(c map[string]any) { c["iss"] = "https://evil.example" },
		"expired token":    func(c map[string]any) { c["exp"] = now.Add(-time.Hour).Unix() },
		"no subject":       func(c map[string]any) { delete(c, "sub") },
		"subject too long": func(c map[string]any) { c["sub"] = strings.Repeat("s", 300) },
	} {
		s.idp.setTweak(tweak)
		callback(name, "failed", func(u string) string { return u })
	}
	s.idp.setTweak(nil)
	s.idp.mu.Lock()
	s.idp.signer = s.idp.other
	s.idp.mu.Unlock()
	callback("token signed with another key", "failed", func(u string) string { return u })
	s.idp.mu.Lock()
	s.idp.signer = s.idp.key
	s.idp.mu.Unlock()

	// The provider can be switched off between the start and the end of a sign-in.
	late := newBrowser(f)
	a = s.idp.approve(start(late), "u11", email, "Test", true)
	f.want(s.put(map[string]any{"enabled": false}), 200, "switch off mid-flow")
	_, location, c = late.get(a)
	refused("provider turned off mid-flow", "unavailable", late, location, sessionCount(c))
}

func TestSSOLinkAndUnlinkNeedTheSameSignedInPerson(t *testing.T) {
	s := newSSOSetup(t)
	f := s.f
	alice, bob := f.withPassword(f.person("regular")), f.person("regular")
	// Today an active user always has a password; provisioning (SCIM) may one day create people who only use SSO, so the guard is tested
	// with the constraint lifted in this test's own database.
	f.exec("ALTER TABLE users DROP CONSTRAINT users_active_has_password")
	f.exec("UPDATE users SET password_hash=NULL WHERE id=$1", bob.id)

	link := func(b *browserSession, sub string) (location string) {
		t.Helper()
		status, auth, _ := b.get("/api/v1/auth/sso/oidc/start?link=1&next=" + url.QueryEscape("/settings/security"))
		if status != 302 {
			t.Fatalf("link start = %d", status)
		}
		_, location, _ = b.get(s.idp.approve(auth, sub, "irrelevant@example.org", "X", false))
		return location
	}
	// Without a session the link flow goes to the sign-in page and starts nothing.
	if status, location, _ := newBrowser(f).get("/api/v1/auth/sso/oidc/start?link=1"); status != 302 || location != "/login" {
		t.Fatalf("link without a session = %d %q", status, location)
	}
	if got := link(newBrowser(f).use(alice), "sub-a"); got != "/settings/security?sso=linked" {
		t.Fatalf("link ended at %q", got)
	}
	// The email claim is irrelevant when linking: the person is already signed in.
	if got := f.queryID("SELECT user_id::text FROM user_identities WHERE subject='sub-a'"); got != alice.id {
		t.Fatalf("linked to %s", got)
	}
	mine := f.call(alice, "GET", "/me/identities", nil)
	if len(mine.items()) != 1 || mine.items()[0]["kind"] != "oidc" || mine.items()[0]["provider_name"] != "Company login" {
		t.Fatalf("identities = %v", mine.body)
	}
	if len(f.call(bob, "GET", "/me/identities", nil).items()) != 0 {
		t.Fatal("identities are personal")
	}
	// Someone else cannot take an identity that is already linked, and the failed attempt links nothing.
	if reasonOf(t, link(newBrowser(f).use(bob), "sub-a")) != "already_linked" {
		t.Fatal("a linked subject must not move")
	}
	if f.queryID("SELECT user_id::text FROM user_identities WHERE subject='sub-a'") != alice.id {
		t.Fatal("the identity moved")
	}
	// The person who finishes must be the person who started.
	started := newBrowser(f).use(alice)
	_, auth, _ := started.get("/api/v1/auth/sso/oidc/start?link=1")
	back := s.idp.approve(auth, "sub-b", "x@example.org", "X", false)
	u, _ := url.Parse(f.srv.URL)
	jar := started.jar
	jar.SetCookies(u, []*http.Cookie{{Name: "pms_session", Value: bob.cookie.Value, Path: "/"}}) // same browser, now signed in as bob
	if _, location, _ := started.get(back); reasonOf(t, location) != "session" {
		t.Fatalf("a link finished by another session must be refused, got %q", location)
	}
	if f.queryID("SELECT count(*)::text FROM user_identities WHERE subject='sub-b'") != "0" {
		t.Fatal("nothing may be linked by a different session")
	}

	// Unlinking: the person needs a password, otherwise they could never sign in again.
	f.exec("INSERT INTO user_identities(provider_id,subject,user_id) SELECT id,'sub-bob',$1 FROM sso_providers", bob.id)
	f.want(f.call(bob, "DELETE", "/me/identities/oidc", nil), 409, "no password")
	f.want(f.call(alice, "DELETE", "/me/identities/oidc", nil), 204, "unlink")
	f.want(f.call(alice, "DELETE", "/me/identities/oidc", nil), 404, "unlink twice")
	f.want(f.call(nil, "GET", "/me/identities", nil), 401, "anonymous")
	if lastAudit(t, f, s.admin, "sso.unlinked")["actor_id"] != alice.id {
		t.Fatal("unlinking must be audited")
	}
	// Once unlinked, the subject signs in again only through the email rule.
	if reasonOf(t, func() string { l, _ := s.signIn(newBrowser(f), "", "sub-a", "another@example.org", true); return l }()) != "unlinked" {
		t.Fatal("an unlinked subject with an unknown email must be refused")
	}
}

func sessionCount(cookies []*http.Cookie) int {
	n := 0
	for _, c := range cookies {
		if c.Name == "pms_session" && c.Value != "" {
			n++
		}
	}
	return n
}
