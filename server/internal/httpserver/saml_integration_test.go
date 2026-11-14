package httpserver_test

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/sso"
)

type samlSetup struct {
	f     *fixture
	idp   *fakeSAMLIdP
	admin *person
}

func newSAMLSetup(t *testing.T) *samlSetup {
	t.Helper()
	f := newFixture(t)
	s := &samlSetup{f: f, idp: newFakeSAMLIdP(t), admin: f.person("admin")}
	f.want(s.put(nil), 200, "configure saml")
	return s
}

func (s *samlSetup) put(over map[string]any) reply {
	body := map[string]any{"name": "Company SAML", "enabled": true, "idp_metadata_xml": s.idp.metadata(), "auto_link": true, "allowed_domains": []string{}}
	for k, v := range over {
		body[k] = v
	}
	return s.f.call(s.admin, "PUT", "/sso/saml", body)
}

// start begins a sign-in (or a link) and returns what the provider received.
func (s *samlSetup) start(b *browserSession, query string) authnRequest {
	s.f.t.Helper()
	status, redirect, _ := b.get("/api/v1/auth/sso/saml/start" + query)
	if status != 302 {
		s.f.t.Fatalf("start = %d", status)
	}
	return s.idp.request(redirect)
}

// answer posts a response the way the provider's page does.
func (s *samlSetup) answer(b *browserSession, req authnRequest, encoded string, withSession bool) (location string, sessions int) {
	s.f.t.Helper()
	status, location, cookies := b.post(req.acs, url.Values{"SAMLResponse": {encoded}, "RelayState": {req.relay}}, withSession)
	if status != 302 {
		s.f.t.Fatalf("acs = %d", status)
	}
	return location, sessionCount(cookies)
}

func TestSAMLConfigurationAndMetadata(t *testing.T) {
	f := newFixture(t)
	idp := newFakeSAMLIdP(t)
	admin, regular := f.person("admin"), f.person("regular")
	put := func(who *person, over map[string]any) reply {
		body := map[string]any{"name": "Company SAML", "enabled": false, "idp_metadata_xml": idp.metadata(), "auto_link": true, "allowed_domains": []string{}}
		for k, v := range over {
			body[k] = v
		}
		return f.call(who, "PUT", "/sso/saml", body)
	}
	f.want(put(regular, nil), 403, "regular user")
	f.want(f.call(nil, "GET", "/sso/saml", nil), 401, "anonymous")

	// The service provider metadata is public and holds nothing secret.
	resp, err := http.Get(f.srv.URL + "/api/v1/auth/sso/saml/metadata")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	status, header, body := resp.StatusCode, resp.Header, string(raw)
	if status != 200 || !strings.Contains(header.Get("Content-Type"), "xml") {
		t.Fatalf("metadata = %d %v", status, header)
	}
	for _, want := range []string{`entityID="` + publicURL + `/api/v1/auth/sso/saml/metadata"`, `Location="` + publicURL + `/api/v1/auth/sso/saml/acs"`, `WantAssertionsSigned="true"`, `HTTP-POST`} {
		if !strings.Contains(body, want) {
			t.Errorf("metadata lacks %s:\n%s", want, body)
		}
	}
	empty := f.call(admin, "GET", "/sso/saml", nil)
	if empty.body["configured"] != false || empty.body["entity_id"] == nil || empty.body["acs_url"] == nil {
		t.Fatalf("empty view = %v", empty.body)
	}

	// Metadata must hold what a sign-in needs.
	cert := strings.Split(strings.Split(idp.metadata(), "<ds:X509Certificate>")[1], "</ds:X509Certificate>")[0]
	for name, xml := range map[string]string{
		"not xml":        "hello",
		"empty":          "",
		"no entity id":   `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata"><IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"/></EntityDescriptor>`,
		"no idp":         `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="x"><SPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"/></EntityDescriptor>`,
		"no redirect":    strings.Replace(idp.metadata(), "HTTP-Redirect", "HTTP-POST", 1),
		"no certificate": strings.Replace(idp.metadata(), cert, "", 1),
		"too large":      strings.Repeat("x", 300<<10),
		"doctype":        `<?xml version="1.0"?><!DOCTYPE a [<!ENTITY b "c">]>` + idp.metadata(),
	} {
		if r := put(admin, map[string]any{"idp_metadata_xml": xml}); r.status != 422 {
			t.Errorf("%s: status %d, want 422", name, r.status)
		}
	}
	// Outside tests the provider's sign-on address must be public https.
	sso.AllowInsecure = false
	f.want(put(admin, map[string]any{"idp_metadata_xml": strings.Replace(idp.metadata(), idp.srv.URL+"/sso", "https://localhost/sso", 1)}), 422, "local sign-on address")
	f.want(put(admin, map[string]any{"idp_metadata_xml": strings.Replace(idp.metadata(), idp.srv.URL+"/sso", "http://idp.example.com/sso", 1)}), 422, "plain http sign-on address")
	sso.AllowInsecure = true
	f.want(put(admin, map[string]any{"issuer": "https://x.example"}), 422, "oidc fields are refused")
	f.want(put(admin, map[string]any{"client_secret": "x"}), 422, "saml has no secret")
	f.want(put(admin, map[string]any{"allowed_domains": []string{"a@b"}}), 422, "bad domain")

	saved := put(admin, map[string]any{"enabled": true, "email_attribute": "mail"})
	f.want(saved, 200, "save")
	if saved.body["has_metadata"] != true || saved.body["idp_entity_id"] != idp.entityID || saved.body["idp_sign_on_url"] != idp.srv.URL+"/sso" || saved.body["enabled"] != true {
		t.Fatalf("view = %v", saved.body)
	}
	kept := put(admin, map[string]any{"idp_metadata_xml": "", "enabled": false})
	f.want(kept, 200, "saving without metadata keeps the stored one")
	if kept.body["has_metadata"] != true || kept.body["enabled"] != false {
		t.Fatalf("kept = %v", kept.body)
	}
	f.want(put(admin, map[string]any{"enabled": true}), 200, "enable again")
	if strings.Contains(saved.raw(), "BEGIN") || strings.Contains(strings.ToLower(saved.raw()), "x509certificate") {
		t.Fatal("the view must not echo the metadata document")
	}
	if list := f.call(nil, "GET", "/auth/sso/providers", nil).items(); len(list) != 1 || list[0]["kind"] != "saml" {
		t.Fatalf("providers = %v", list)
	}
	if e := lastAudit(t, f, admin, "sso.configured"); e["target_id"] != "saml" || e["details"].(map[string]any)["idp_entity_id"] != idp.entityID {
		t.Fatalf("audit = %v", e)
	}
	f.want(f.call(admin, "DELETE", "/sso/saml", nil), 204, "delete")
}

func TestSAMLSignInLinksAndRefusesForgeries(t *testing.T) {
	s := newSAMLSetup(t)
	f := s.f
	alice := f.withPassword(f.person("regular"))
	email := f.email(alice)
	admin2 := f.person("admin")
	adminEmail := f.email(admin2)

	// A good response, signed on the assertion: first sign-in links by email, the session is alice's.
	b := newBrowser(f)
	req := s.start(b, "?next="+url.QueryEscape("/spaces/x"))
	if req.acs != publicURL+"/api/v1/auth/sso/saml/acs" || req.issuer != publicURL+"/api/v1/auth/sso/saml/metadata" {
		t.Fatalf("request = %+v", req)
	}
	location, sessions := s.answer(b, req, s.idp.build(s.idp.defaults(req, "alice-id", strings.ToUpper(email))), false)
	if location != "/spaces/x" || sessions != 1 {
		t.Fatalf("sign-in ended at %q with %d sessions", location, sessions)
	}
	if got := f.queryID("SELECT user_id::text FROM user_identities WHERE subject='alice-id'"); got != alice.id {
		t.Fatalf("linked to %s", got)
	}
	if me := f.call(&person{cookie: sessionCookieOf(b)}, "GET", "/me", nil); me.status != 200 || me.body["id"] != alice.id {
		t.Fatalf("session = %+v", me)
	}
	if d := lastAudit(t, f, s.admin, "auth.login")["details"].(map[string]any); d["method"] != "sso:saml" {
		t.Fatalf("login method = %v", d)
	}
	// A response signed on the envelope instead of the assertion is accepted too, and a linked subject needs no email.
	req = s.start(newBrowser(f), "")
	b2 := newBrowser(f)
	req = s.start(b2, "")
	r := s.idp.defaults(req, "alice-id", "")
	r.sign = "response"
	if location, sessions := s.answer(b2, req, s.idp.build(r), false); location != "/" || sessions != 1 {
		t.Fatalf("response-level signature: %q %d", location, sessions)
	}

	refused := func(name, reason string, mutate func(*samlResponse)) {
		t.Helper()
		bb := newBrowser(f)
		req := s.start(bb, "")
		r := s.idp.defaults(req, "alice-id", email)
		mutate(&r)
		location, sessions := s.answer(bb, req, s.idp.build(r), false)
		if got := reasonOf(t, location); got != reason || sessions != 0 || bb.sessionCookie() != "" {
			t.Errorf("%s: reason %q sessions %d, want %q and no session", name, got, sessions, reason)
		}
		if d := lastAudit(t, f, s.admin, "sso.login_failed")["details"].(map[string]any); d["reason"] != reason {
			t.Errorf("%s: audit reason %v", name, d["reason"])
		}
	}
	now := time.Now()
	// Assertions that are not what the provider would send.
	refused("not signed at all", "failed", func(r *samlResponse) { r.sign = "none" })
	refused("signed by another key", "failed", func(r *samlResponse) { r.signer = &s.idp.other })
	refused("signed with another key on the response", "failed", func(r *samlResponse) { r.sign, r.signer = "response", &s.idp.other })
	refused("name changed after signing", "failed", func(r *samlResponse) {
		r.tamper = func(x string) string { return strings.Replace(x, ">alice-id<", ">"+"admin-id"+"<", 1) }
	})
	refused("email changed after signing", "failed", func(r *samlResponse) {
		r.tamper = func(x string) string { return strings.Replace(x, strings.ToLower(email), adminEmail, 1) }
	})
	refused("signature removed", "failed", func(r *samlResponse) {
		r.tamper = func(x string) string {
			stripped := regexp.MustCompile(`(?s)<(?:ds:)?Signature[ >].*?</(?:ds:)?Signature>`).ReplaceAllString(x, "")
			if stripped == x {
				t.Fatal("no signature to remove")
			}
			return stripped
		}
	})
	refused("document type declaration", "failed", func(r *samlResponse) {
		r.tamper = func(x string) string { return `<!DOCTYPE r [<!ENTITY e "x">]>` + x }
	})
	refused("expired", "failed", func(r *samlResponse) { r.notOnOrAfter, r.confirmUntil = now.Add(-time.Hour), now.Add(-time.Hour) })
	refused("not yet valid", "failed", func(r *samlResponse) { r.notBefore = now.Add(time.Hour) })
	refused("no time window", "failed", func(r *samlResponse) { r.noConditions = true })
	refused("confirmation expired", "failed", func(r *samlResponse) { r.confirmUntil = now.Add(-time.Hour) })
	refused("another audience", "failed", func(r *samlResponse) { r.audience = "https://other.example/sp" })
	refused("no audience restriction", "failed", func(r *samlResponse) { r.audience = "-" })
	refused("another issuer", "failed", func(r *samlResponse) { r.issuer = "https://evil.example/entity" })
	refused("another destination", "failed", func(r *samlResponse) { r.sign, r.destination = "both", "https://evil.example/acs" })
	refused("another recipient", "failed", func(r *samlResponse) { r.recipient = "https://evil.example/acs" })
	refused("answers another request", "failed", func(r *samlResponse) { r.inResponseTo = "id-someone-elses-request" })
	refused("provider-initiated (no request)", "failed", func(r *samlResponse) { r.inResponseTo = "-" })
	refused("no subject confirmation", "failed", func(r *samlResponse) { r.noConfirm = true })
	refused("provider reports failure", "failed", func(r *samlResponse) { r.status = "urn:oasis:names:tc:SAML:2.0:status:Responder" })
	refused("transient name id", "failed", func(r *samlResponse) { r.nameIDFormat = "urn:oasis:names:tc:SAML:2.0:nameid-format:transient" })
	refused("empty name id", "failed", func(r *samlResponse) { r.nameID = "  " })
	// Signature wrapping: a second, unsigned assertion for an administrator is placed before the signed one for alice.
	bb := newBrowser(f)
	wreq := s.start(bb, "")
	good := s.idp.defaults(wreq, "alice-id", email)
	evil := s.idp.defaults(wreq, "admin-id", adminEmail)
	evilXML := s.idp.assertion(evil, "_evil")
	good.extraAssertion = evilXML
	location, sessions = s.answer(bb, wreq, s.idp.build(good), false)
	if sessions == 1 {
		if me := f.call(&person{cookie: sessionCookieOf(bb)}, "GET", "/me", nil); me.body["id"] != alice.id {
			t.Fatalf("a wrapped assertion must never sign in as someone else, got %v", me.body["id"])
		}
	} else if reasonOf(t, location) == "" {
		t.Fatalf("unexpected end of the wrapping attempt: %q", location)
	}
	if f.queryID("SELECT count(*)::text FROM user_identities WHERE subject='admin-id'") != "0" {
		t.Fatal("the unsigned assertion must not link anything")
	}

	// A response is single use: the state is spent, replaying it fails even with the right cookie.
	replay := newBrowser(f)
	rreq := s.start(replay, "")
	encoded := s.idp.build(s.idp.defaults(rreq, "alice-id", email))
	if _, n := s.answer(replay, rreq, encoded, false); n != 1 {
		t.Fatal("the first answer must work")
	}
	before := f.queryID("SELECT count(*)::text FROM sessions")
	if location, n := s.answer(replay, rreq, encoded, false); reasonOf(t, location) != "state" || n != 0 {
		t.Fatalf("a replayed answer: %q %d", location, n)
	}
	if f.queryID("SELECT count(*)::text FROM sessions") != before {
		t.Fatal("a replay must not create a session")
	}
	// Answers need the browser that started the flow, a known state, and a live one.
	other, starter := newBrowser(f), newBrowser(f)
	oreq := s.start(starter, "")
	if location, n := s.answer(other, oreq, s.idp.build(s.idp.defaults(oreq, "alice-id", email)), false); reasonOf(t, location) != "state" || n != 0 {
		t.Fatalf("another browser: %q %d", location, n)
	}
	ureq := s.start(newBrowser(f), "")
	ub := newBrowser(f)
	if location, _ := s.answer(ub, authnRequest{id: ureq.id, acs: ureq.acs, relay: "forged-relay-state"}, s.idp.build(s.idp.defaults(ureq, "alice-id", email)), false); reasonOf(t, location) != "state" {
		t.Fatalf("an unknown RelayState: %q", location)
	}
	xb := newBrowser(f)
	xreq := s.start(xb, "")
	f.exec("UPDATE sso_states SET expires_at = now() - interval '1 second'")
	if location, n := s.answer(xb, xreq, s.idp.build(s.idp.defaults(xreq, "alice-id", email)), false); reasonOf(t, location) != "state" || n != 0 {
		t.Fatalf("an expired flow: %q %d", location, n)
	}

	// The rules after the identity is proven are the same as for OIDC.
	run := func(name, reason, sub, mail string) {
		t.Helper()
		refused(name, reason, func(r *samlResponse) { r.nameID, r.email = sub, mail })
	}
	run("no such user, and none is created", "unlinked", "s-unknown", "nobody@example.org")
	deactivated := f.person("regular")
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", deactivated.id)
	run("deactivated user", "inactive", "s-deact", f.email(deactivated))
	f.want(s.put(map[string]any{"allowed_domains": []string{"corp.example"}}), 200, "restrict domains")
	run("domain not allowed", "domain", "s-dom", email)
	f.want(s.put(map[string]any{"allowed_domains": []string{}, "auto_link": false}), 200, "no auto link")
	run("auto link off", "unlinked", "s-nolink", email)
	f.want(s.put(map[string]any{"allowed_domains": []string{}, "auto_link": true, "enabled": false}), 200, "turn off")
	if status, _, _ := newBrowser(f).get("/api/v1/auth/sso/saml/start"); status != 404 {
		t.Fatalf("a disabled provider cannot be started, got %d", status)
	}
}

func sessionCookieOf(b *browserSession) *http.Cookie {
	return &http.Cookie{Name: "pms_session", Value: b.sessionCookie()}
}

// The answer is a cross-site POST. Only that one route skips the Origin check; everything else keeps it.
func TestSAMLAnswerRouteIsTheOnlyOriginException(t *testing.T) {
	s := newSAMLSetup(t)
	f := s.f
	b := newBrowser(f)
	req := s.start(b, "")
	if status, _, _ := b.post(req.acs, url.Values{"SAMLResponse": {"x"}, "RelayState": {req.relay}}, false); status != 302 {
		t.Fatalf("the answer route must be reachable from the provider's page, got %d", status)
	}
	if status, _, _ := b.post("/api/v1/auth/logout", url.Values{}, true); status != 403 {
		t.Fatalf("other POST routes must still check the Origin, got %d", status)
	}
	if status, _, _ := b.post("/api/v1/auth/sso/saml/acs/extra", url.Values{}, true); status == 302 || status == 200 {
		t.Fatalf("a look-alike path must not be exempt, got %d", status)
	}
	// The answer is only read for SAML; a GET or another kind has no such route.
	if status, _, _ := newBrowser(f).get("/api/v1/auth/sso/saml/callback"); status != 404 {
		t.Fatalf("saml has no GET callback, got %d", status)
	}
}

func TestSAMLLinkNeedsTheStateAndTheBrowserOfWhoStartedIt(t *testing.T) {
	s := newSAMLSetup(t)
	f := s.f
	alice, bob := f.withPassword(f.person("regular")), f.person("regular")
	// A browser signed in as alice starts a link; the provider's cross-site POST carries no session cookie.
	b := newBrowser(f).use(alice)
	req := s.start(b, "?link=1&next="+url.QueryEscape("/settings/security"))
	location, _ := s.answer(b, req, s.idp.build(s.idp.defaults(req, "alice-saml", "whoever@example.org")), false)
	if location != "/settings/security?sso=linked" {
		t.Fatalf("link ended at %q", location)
	}
	if got := f.queryID("SELECT user_id::text FROM user_identities WHERE subject='alice-saml'"); got != alice.id {
		t.Fatalf("linked to %s", got)
	}
	// A subject linked to alice cannot be taken by bob's link flow.
	bb := newBrowser(f).use(bob)
	req = s.start(bb, "?link=1")
	if location, _ := s.answer(bb, req, s.idp.build(s.idp.defaults(req, "alice-saml", "x@example.org")), false); reasonOf(t, location) != "already_linked" {
		t.Fatalf("a linked subject must not move, got %q", location)
	}
	// Without a session there is nothing to link to.
	if status, location, _ := newBrowser(f).get("/api/v1/auth/sso/saml/start?link=1"); status != 302 || location != "/login" {
		t.Fatalf("link without a session = %d %q", status, location)
	}
	_ = sso.StateTTL
}
