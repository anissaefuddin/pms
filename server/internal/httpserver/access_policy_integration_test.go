package httpserver_test

import (
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
)

func (f *fixture) lock(admin *person, space string, locked bool) {
	f.t.Helper()
	v := f.call(admin, "GET", "/spaces/"+space, nil)
	f.want(v, 200, "read space")
	f.want(f.call(admin, "PATCH", "/spaces/"+space, map[string]any{"locked": locked}, "If-Match", v.header.Get("ETag")), 200, "lock change")
}

func TestLockedSpaceIsAdminOnlyAndAudited(t *testing.T) {
	f := newFixture(t)
	admin, owner := f.person("admin"), f.person("regular")
	space := f.space("Finance")
	f.share(space, "user", owner.id, "full")

	v := f.call(owner, "GET", "/spaces/"+space, nil)
	if v.body["locked"] != false {
		t.Fatalf("a new space is not locked: %v", v.body)
	}
	etag := v.header.Get("ETag")
	if r := f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"locked": true}, "If-Match", etag); r.status != 403 {
		t.Fatalf("a space owner cannot lock it: %d", r.status)
	}
	if r := f.call(admin, "PATCH", "/spaces/"+space, map[string]any{"locked": "yes"}, "If-Match", etag); r.status != 422 {
		t.Fatalf("locked must be a boolean: %d", r.status)
	}
	locked := f.call(admin, "PATCH", "/spaces/"+space, map[string]any{"locked": true}, "If-Match", etag)
	f.want(locked, 200, "lock")
	if locked.body["locked"] != true || f.call(owner, "GET", "/spaces/"+space, nil).body["locked"] != true {
		t.Fatalf("locked space: %v", locked.body)
	}
	if e := lastAudit(t, f, admin, "policy.space_locked"); e["actor_id"] != admin.id || e["target_id"] != space {
		t.Fatalf("audit: %v", e)
	}
	// A rename by the owner still works and keeps the lock.
	r := f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"name": "Finance 2"}, "If-Match", locked.header.Get("ETag"))
	f.want(r, 200, "rename")
	if r.body["locked"] != true {
		t.Fatal("a rename must not unlock the space")
	}

	// Who has access: the owner, with the full role that includes sharing, cannot change it while locked.
	shares := f.call(owner, "GET", "/spaces/"+space+"/shares", nil)
	other := f.person("regular")
	body := map[string]any{"shares": []map[string]string{{"principal_type": "user", "principal_id": owner.id, "role": "full"}, {"principal_type": "user", "principal_id": other.id, "role": "read"}}}
	if r := f.call(owner, "PUT", "/spaces/"+space+"/shares", body, "If-Match", shares.header.Get("ETag")); r.status != 403 {
		t.Fatalf("a locked space's sharing is admin only: %d %v", r.status, r.body)
	}
	if f.call(other, "GET", "/spaces/"+space, nil).status != 404 {
		t.Fatal("the refused share must not exist")
	}
	r = f.call(admin, "PUT", "/spaces/"+space+"/shares", body, "If-Match", shares.header.Get("ETag"))
	f.want(r, 200, "an administrator can still share")
	f.lock(admin, space, false)
	if e := lastAudit(t, f, admin, "policy.space_unlocked"); e["target_id"] != space {
		t.Fatalf("audit: %v", e)
	}
	shares = f.call(owner, "GET", "/spaces/"+space+"/shares", nil)
	body["shares"] = []map[string]string{{"principal_type": "user", "principal_id": owner.id, "role": "full"}}
	f.want(f.call(owner, "PUT", "/spaces/"+space+"/shares", body, "If-Match", shares.header.Get("ETag")), 200, "unlocked again")
}

func TestLockedSpaceKeepsItemsAmongThePeopleWhoHaveTheSpace(t *testing.T) {
	f := newFixture(t)
	admin, owner, member, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	viaGroup, groupie := f.person("regular"), f.person("regular")
	inGroup, outGroup := f.group("Finance team", groupie), f.group("Everyone else", outsider)
	space, elsewhere := f.space("Finance"), f.space("Public")
	f.share(space, "user", owner.id, "full")
	f.share(space, "user", member.id, "read")
	f.share(space, "group", inGroup, "read")
	f.share(elsewhere, "user", owner.id, "full")
	item := customFieldItem(f, admin, space, "Payroll")
	f.lock(admin, space, true)
	path := "/items/" + item + "/shares"
	put := func(actor *person, principalType, id string) reply {
		cur := f.call(actor, "GET", path, nil)
		f.want(cur, 200, "read item shares")
		list := []map[string]string{}
		for _, s := range cur.items() {
			list = append(list, map[string]string{"principal_type": s["principal_type"].(string), "principal_id": s["principal_id"].(string), "role": s["role"].(string)})
		}
		list = append(list, map[string]string{"principal_type": principalType, "principal_id": id, "role": "read"})
		return f.call(actor, "PUT", path, map[string]any{"shares": list}, "If-Match", cur.header.Get("ETag"))
	}

	if r := put(owner, "user", outsider.id); r.status != 403 {
		t.Fatalf("sharing a locked space's item with someone outside = %d %v", r.status, r.body)
	}
	if r := put(owner, "group", outGroup); r.status != 403 {
		t.Fatalf("with a group outside = %d", r.status)
	}
	f.want(put(owner, "user", member.id), 200, "a person who has the space")
	f.want(put(owner, "user", groupie.id), 200, "a person who has the space through a group")
	f.want(put(owner, "group", inGroup), 200, "a group that has the space")
	f.want(put(owner, "user", admin.id), 200, "an administrator always has the space")
	if r := put(admin, "user", outsider.id); r.status != 403 {
		t.Fatalf("the rule holds for administrators too, until they unlock: %d", r.status)
	}
	_ = viaGroup
	if f.call(outsider, "GET", "/items/"+item, nil).status != 404 {
		t.Fatal("the refused share must not exist")
	}

	// New items created inside the space follow the same rule.
	create := func(actor *person, shareWith string) reply {
		return f.call(actor, "POST", "/items", map[string]any{"type": "project", "title": "New", "locations": []map[string]any{{"space_id": space}},
			"shares": []map[string]string{{"principal_type": "user", "principal_id": shareWith, "role": "read"}}})
	}
	if r := create(owner, outsider.id); r.status != 403 {
		t.Fatalf("creating with an outside share = %d %v", r.status, r.body)
	}
	if f.queryID("SELECT count(*)::text FROM items WHERE title='New'") != "0" {
		t.Fatal("a refused create must leave nothing behind")
	}
	f.want(create(owner, member.id), 201, "creating with an inside share")

	// An item cannot be placed in another space by someone who is not an administrator, but can move inside its own.
	loc := func(space string) map[string]any { return map[string]any{"space_id": space, "parent_item_id": nil} }
	loc2 := f.call(owner, "PUT", "/items/"+item+"/locations", map[string]any{"locations": []any{loc(space), loc(elsewhere)}})
	if loc2.status != 403 {
		t.Fatalf("adding another space to a locked item = %d %v", loc2.status, loc2.body)
	}
	f.want(f.call(owner, "PUT", "/items/"+item+"/locations", map[string]any{"locations": []any{loc(space)}}), 200, "a change that stays in the space")
	f.want(f.call(admin, "PUT", "/items/"+item+"/locations", map[string]any{"locations": []any{loc(space), loc(elsewhere)}}), 200, "an administrator may")

	// Unlocking lifts every rule.
	f.lock(admin, space, false)
	f.want(put(owner, "user", outsider.id), 200, "after unlocking")
}

func TestIPAllowlistGuardsSignInAndEveryRequest(t *testing.T) {
	f := newFixture(t)
	admin, alice := f.person("admin"), f.withPassword(f.person("regular"))
	email := f.email(alice)
	here := []string{"X-Forwarded-For", "203.0.113.7"}
	get := func() reply { return f.call(admin, "GET", "/access-policy", nil, here...) }
	policy := get()
	f.want(policy, 200, "read policy")
	if policy.body["ip_allowlist_enabled"] != false || policy.body["your_ip"] != "203.0.113.7" || policy.body["override_active"] != false {
		t.Fatalf("policy: %v", policy.body)
	}
	put := func(body map[string]any, headers ...string) reply {
		cur := f.call(admin, "GET", "/access-policy", nil, here...)
		return f.call(admin, "PUT", "/access-policy", body, append([]string{"If-Match", cur.header.Get("ETag")}, headers...)...)
	}

	// Only administrators, and only sensible lists.
	f.want(f.call(nil, "GET", "/access-policy", nil), 401, "anonymous")
	f.want(f.call(alice, "GET", "/access-policy", nil), 403, "regular read")
	f.want(f.call(alice, "PUT", "/access-policy", map[string]any{"ip_allowlist_enabled": false, "ip_allowlist": []string{}}, "If-Match", `"1"`), 403, "regular write")
	f.want(f.call(admin, "PUT", "/access-policy", map[string]any{"ip_allowlist_enabled": false, "ip_allowlist": []string{}}), 428, "If-Match required")
	for name, body := range map[string]map[string]any{
		"not an address": {"ip_allowlist_enabled": false, "ip_allowlist": []string{"office"}},
		"bad prefix":     {"ip_allowlist_enabled": false, "ip_allowlist": []string{"10.0.0.0/33"}},
		"empty but on":   {"ip_allowlist_enabled": true, "ip_allowlist": []string{}},
		"too many":       {"ip_allowlist_enabled": false, "ip_allowlist": make([]string, 51)},
	} {
		if r := put(body, here...); r.status != 422 {
			t.Errorf("%s: %d %v", name, r.status, r.body)
		}
	}
	// A list that would lock the saver out is refused, and nothing is stored.
	r := put(map[string]any{"ip_allowlist_enabled": true, "ip_allowlist": []string{"198.51.100.0/24"}}, here...)
	if r.status != 422 || r.body["code"] != "self_lockout" || !strings.Contains(r.body["detail"].(string), "203.0.113.7") {
		t.Fatalf("self lockout: %d %v", r.status, r.body)
	}
	if get().body["ip_allowlist_enabled"] != false {
		t.Fatal("a refused policy must not be stored")
	}

	on := put(map[string]any{"ip_allowlist_enabled": true, "ip_allowlist": []string{"203.0.113.7", "198.51.100.0/24", "2001:db8::/32", "198.51.100.5"}}, here...)
	f.want(on, 200, "turn on")
	if got := on.body["ip_allowlist"].([]any); len(got) != 4 || got[0] != "203.0.113.7/32" {
		t.Fatalf("entries are stored as networks: %v", got)
	}
	if f.call(admin, "PUT", "/access-policy", map[string]any{"ip_allowlist_enabled": false, "ip_allowlist": []string{}}, "If-Match", `"1"`, "X-Forwarded-For", "203.0.113.7").status != 409 {
		t.Fatal("a stale version must conflict")
	}

	audit := func(action string) map[string]any { // the administrator reads from an allowed address
		r := f.call(admin, "GET", "/audit-events?limit=1&action="+action, nil, here...)
		f.want(r, 200, "audit "+action)
		if len(r.items()) != 1 {
			t.Fatalf("no %s event", action)
		}
		return r.items()[0]
	}
	if e := audit("policy.ip_allowlist_changed"); e["actor_id"] != admin.id {
		t.Fatalf("audit: %v", e)
	}
	denied := func(what string, r reply) {
		t.Helper()
		if r.status != 403 || r.body["code"] != "ip_not_allowed" {
			t.Errorf("%s from outside: %d %v", what, r.status, r.body)
		}
	}
	outside := []string{"X-Forwarded-For", "192.0.2.50"}
	// A signed-in session is useless from outside, and fine from inside (IPv4, a network, IPv6, an IPv4-mapped form).
	denied("a session request", f.call(alice, "GET", "/me", nil, outside...))
	denied("anonymous login", f.call(nil, "POST", "/auth/login", map[string]any{"email": email, "password": "whatever"}, outside...))
	f.want(f.call(alice, "GET", "/me", nil, "X-Forwarded-For", "198.51.100.77"), 200, "inside the network")
	f.want(f.call(alice, "GET", "/me", nil, "X-Forwarded-For", "2001:db8:1::5"), 200, "inside the IPv6 network")
	f.want(f.call(alice, "GET", "/me", nil, "X-Forwarded-For", "::ffff:198.51.100.9"), 200, "an IPv4-mapped address")
	denied("a lookalike address", f.call(alice, "GET", "/me", nil, "X-Forwarded-For", "198.51.101.1"))
	denied("garbage in the header", f.call(alice, "GET", "/me", nil, "X-Forwarded-For", "not-an-ip"))
	// The wrong password is not even looked at from outside: no audit of a failed login by password, one by address.
	if e := audit("auth.login_failed"); e["details"].(map[string]any)["reason"] != "ip_not_allowed" {
		t.Fatalf("audit: %v", e)
	}
	if r := f.call(nil, "POST", "/auth/login", map[string]any{"email": email, "password": "wrong"}, "X-Forwarded-For", "198.51.100.1"); r.status != 401 {
		t.Fatalf("login from inside still checks the password: %d", r.status)
	}
	if r := f.loginAsFrom(email, "198.51.100.1"); r.status != 200 {
		t.Fatalf("login from inside: %d", r.status)
	}
	// Endpoints that carry their own credential are not part of the policy.
	f.want(f.call(nil, "GET", "/auth/sso/providers", nil, outside...), 200, "the sign-in page can still load")

	// Turning it off from an allowed address restores access at once.
	off := put(map[string]any{"ip_allowlist_enabled": false, "ip_allowlist": []string{"203.0.113.7"}}, here...)
	f.want(off, 200, "turn off")
	f.want(f.call(alice, "GET", "/me", nil, outside...), 200, "no longer enforced")
}

func (f *fixture) loginAsFrom(email, ip string) reply {
	f.t.Helper()
	return f.call(nil, "POST", "/auth/login", map[string]any{"email": email, "password": twoFactorPassword}, "X-Forwarded-For", ip)
}

func TestIPAllowlistAlsoGuardsSingleSignOnAndHasAWayBack(t *testing.T) {
	s := newSSOSetup(t)
	f := s.f
	alice := f.withPassword(f.person("regular"))
	email := f.email(alice)
	inside := []string{"X-Forwarded-For", "203.0.113.9"}
	cur := f.call(s.admin, "GET", "/access-policy", nil, inside...)
	f.want(f.call(s.admin, "PUT", "/access-policy", map[string]any{"ip_allowlist_enabled": true, "ip_allowlist": []string{"203.0.113.0/24"}}, append([]string{"If-Match", cur.header.Get("ETag")}, inside...)...), 200, "turn on")
	// The browser in the SSO helper arrives from 127.0.0.1, which the list does not hold.
	location, cookies := s.signIn(newBrowser(f), "", "sub-alice", email, true)
	if cookies != 0 || reasonOf(t, location) != "ip" {
		t.Fatalf("single sign-on from outside: %q with %d session cookies", location, cookies)
	}
	if f.queryID("SELECT count(*)::text FROM user_identities") != "0" {
		t.Fatal("a refused sign-in must not link anything")
	}
	if f.queryID("SELECT details->>'reason' FROM audit_events WHERE action='sso.login_failed' ORDER BY id DESC LIMIT 1") != "ip" {
		t.Fatal("the refusal must be audited with its reason")
	}

	// The operator's way back after locking everyone out: the environment switch, then a restart.
	g := newFixtureWith(t, 1000, func(c *config.Config) { c.DisableIPAllowlist = true })
	admin := g.person("admin")
	g.exec("UPDATE access_policy SET ip_allowlist_enabled=true, ip_allowlist='{203.0.113.0/24}'")
	r := g.call(admin, "GET", "/access-policy", nil)
	g.want(r, 200, "the override lets the administrator in")
	if r.body["override_active"] != true || r.body["ip_allowlist_enabled"] != true {
		t.Fatalf("policy under override: %v", r.body)
	}
}
