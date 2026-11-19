package httpserver_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// auditActions lists the recorded actions oldest first, optionally narrowed by a query string such as "?category=auth".
func auditActions(t *testing.T, f *fixture, p *person, query string) []string {
	t.Helper()
	var out []string
	cursor := ""
	for range 20 {
		sep := "?"
		if query != "" {
			sep = "&"
		}
		r := f.call(p, "GET", "/audit-events"+query+sep+"limit=3"+cursor, nil)
		f.want(r, 200, "list audit")
		for _, e := range r.items() {
			out = append([]string{e["action"].(string)}, out...) // pages come newest first
		}
		next, _ := r.body["next_cursor"].(string)
		if next == "" {
			return out
		}
		cursor = "&cursor=" + next
	}
	t.Fatal("pagination never ended")
	return nil
}

func lastAudit(t *testing.T, f *fixture, p *person, action string) map[string]any {
	t.Helper()
	r := f.call(p, "GET", "/audit-events?limit=1&action="+action, nil)
	f.want(r, 200, "audit "+action)
	if len(r.items()) != 1 {
		t.Fatalf("no %s event recorded", action)
	}
	return r.items()[0]
}

func TestAuditTrailRecordsSecurityChangesForAdminsOnly(t *testing.T) {
	f := newFixture(t)
	admin, alice, bob := f.person("admin"), f.withPassword(f.person("regular")), f.person("regular")
	space := f.space("Audit space")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	email := f.email(alice)

	// Login: a wrong password and a right one, then a password change and logout by the user.
	f.want(f.call(nil, "POST", "/auth/login", map[string]any{"email": email, "password": "not the password"}), 401, "wrong password")
	session := sessionOf(f.loginAs(email))
	if session == nil {
		t.Fatal("login failed")
	}
	failed := lastAudit(t, f, admin, "auth.login_failed")
	if failed["actor_id"] != nil || failed["actor_name"] != email || failed["details"].(map[string]any)["reason"] != "password" {
		t.Fatalf("failed login entry = %v", failed)
	}
	ok := lastAudit(t, f, admin, "auth.login")
	if ok["actor_id"] != alice.id || ok["actor_name"] != email || ok["ip"] == "" || ok["details"].(map[string]any)["method"] != "password" {
		t.Fatalf("login entry = %v", ok)
	}
	changedPassword := f.call(session, "POST", "/auth/password/change", map[string]any{"current_password": twoFactorPassword, "new_password": "Another-Horse-77"})
	f.want(changedPassword, 204, "change password")
	f.want(f.call(sessionOf(changedPassword), "POST", "/auth/logout", nil), 204, "logout") // the change rotated the session token

	// Users: invite, type change, deactivate, reactivate.
	invited := f.call(admin, "POST", "/users/invitations", map[string]any{"email": "newbie@example.com", "user_type": "regular"})
	f.want(invited, 201, "invite")
	f.want(f.call(admin, "PATCH", "/users/"+bob.id, map[string]any{"user_type": "viewer"}), 200, "type")
	f.want(f.call(admin, "POST", "/users/"+bob.id+"/deactivate", nil), 200, "deactivate")
	f.want(f.call(admin, "POST", "/users/"+bob.id+"/reactivate", nil), 200, "reactivate")
	if e := lastAudit(t, f, admin, "user.type_changed"); e["target_id"] != bob.id || e["actor_id"] != admin.id ||
		e["details"].(map[string]any)["from"] != "regular" || e["details"].(map[string]any)["to"] != "viewer" {
		t.Fatalf("type change entry = %v", e)
	}
	// A change that changes nothing records nothing.
	before := len(auditActions(t, f, admin, "?category=user"))
	f.want(f.call(admin, "PATCH", "/users/"+bob.id, map[string]any{"user_type": "viewer"}), 200, "same type again")
	if after := len(auditActions(t, f, admin, "?category=user")); after != before {
		t.Fatalf("an unchanged type was recorded (%d -> %d)", before, after)
	}

	// Shares: add, change role, remove, on a Space and on an Item.
	// The Space keeps its admin as the full holder: a list without any full holder is refused (last-holder rule).
	shares := func(list []map[string]any, version string) reply {
		// The admin's own full share is kept in every list, so the Space always has someone with full access.
		list = append([]map[string]any{{"principal_type": "user", "principal_id": admin.id, "role": "full"}}, list...)
		return f.call(admin, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": list}, "If-Match", version)
	}
	current := f.call(admin, "GET", "/spaces/"+space+"/shares", nil)
	f.want(current, 200, "shares")
	v := current.header.Get("ETag")
	r := shares([]map[string]any{{"principal_type": "user", "principal_id": alice.id, "role": "read"}}, v)
	f.want(r, 200, "add share")
	r = shares([]map[string]any{{"principal_type": "user", "principal_id": alice.id, "role": "editor"}}, r.header.Get("ETag"))
	f.want(r, 200, "change share")
	changed := lastAudit(t, f, admin, "share.space_changed")["details"].(map[string]any)
	if c := changed["changed"].([]any); len(c) != 1 || c[0].(map[string]any)["from"] != "read" || c[0].(map[string]any)["role"] != "editor" {
		t.Fatalf("role change details = %v", changed)
	}
	f.want(shares([]map[string]any{}, r.header.Get("ETag")), 200, "remove share")
	if d := lastAudit(t, f, admin, "share.space_changed")["details"].(map[string]any); len(d["removed"].([]any)) != 1 {
		t.Fatalf("removal details = %v", d)
	}
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Shared task',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	cur := f.call(admin, "GET", "/items/"+item+"/shares", nil)
	f.want(f.call(admin, "PUT", "/items/"+item+"/shares", map[string]any{"shares": []map[string]any{{"principal_type": "user", "principal_id": bob.id, "role": "read"}}}, "If-Match", cur.header.Get("ETag")), 200, "item share")
	if e := lastAudit(t, f, admin, "share.item_changed"); e["target_type"] != "item" || e["target_id"] != item {
		t.Fatalf("item share entry = %v", e)
	}
	// Replacing a list with the same list is not a change.
	n := len(auditActions(t, f, admin, "?category=share"))
	cur = f.call(admin, "GET", "/items/"+item+"/shares", nil)
	f.want(f.call(admin, "PUT", "/items/"+item+"/shares", map[string]any{"shares": []map[string]any{{"principal_type": "user", "principal_id": bob.id, "role": "read"}}}, "If-Match", cur.header.Get("ETag")), 200, "same list")
	if m := len(auditActions(t, f, admin, "?category=share")); m != n {
		t.Fatalf("an unchanged share list was recorded (%d -> %d)", n, m)
	}

	// Two-factor changes land in the same trail.
	carol := f.withPassword(f.person("regular"))
	f.enableTwoFactor(carol)
	if e := lastAudit(t, f, admin, "auth.2fa_enabled"); e["actor_id"] != carol.id {
		t.Fatalf("2fa entry = %v", e)
	}

	// Filters and pagination: every recorded action shows up once across pages, categories and actors narrow the list.
	all := auditActions(t, f, admin, "")
	for _, want := range []string{"auth.login_failed", "auth.login", "auth.password_changed", "auth.logout", "user.invited", "user.type_changed", "user.deactivated", "user.reactivated", "share.space_changed", "share.item_changed", "auth.2fa_enabled"} {
		if !slices.Contains(all, want) {
			t.Errorf("%s was not recorded; got %v", want, all)
		}
	}
	if got := auditActions(t, f, admin, "?category=share"); len(got) != 4 {
		t.Errorf("share category = %v, want 4 entries", got)
	}
	for _, a := range auditActions(t, f, admin, "?category=auth") {
		if !strings.HasPrefix(a, "auth.") {
			t.Errorf("category filter returned %s", a)
		}
	}
	if got := auditActions(t, f, admin, "?actor_id="+alice.id); len(got) != 3 { // login, password change, logout
		t.Errorf("alice's entries = %v", got)
	}
	f.want(f.call(admin, "GET", "/audit-events?from=2999-01-01T00:00:00Z", nil), 200, "future window")
	if r := f.call(admin, "GET", "/audit-events?from=2999-01-01T00:00:00Z", nil); len(r.items()) != 0 {
		t.Error("a window in the future must be empty")
	}
	// A category is a literal prefix: LIKE wildcards do not widen it.
	if r := f.call(admin, "GET", "/audit-events?category=%25", nil); len(r.items()) != 0 {
		t.Errorf("'%%' matched %d entries", len(r.items()))
	}

	// Passwords never reach the trail.
	var dump strings.Builder
	for _, e := range f.call(admin, "GET", "/audit-events?limit=200", nil).items() {
		dump.WriteString(fmt.Sprint(e))
	}
	for _, secret := range []string{twoFactorPassword, "Another-Horse-77", "not the password"} {
		if strings.Contains(dump.String(), secret) {
			t.Errorf("the trail contains %q", secret)
		}
	}

	// Only admins read it; bad parameters are 422.
	dave := f.person("regular") // alice and bob lost their sessions above
	f.want(f.call(dave, "GET", "/audit-events", nil), 403, "regular user")
	f.want(f.call(nil, "GET", "/audit-events", nil), 401, "anonymous")
	f.want(f.call(dave, "GET", "/audit-events/export", nil), 403, "regular user export")
	for _, bad := range []string{"?actor_id=nope", "?from=yesterday", "?to=1", "?limit=0", "?limit=201", "?cursor=abc", "?cursor=-4"} {
		f.want(f.call(admin, "GET", "/audit-events"+bad, nil), 422, bad)
	}
}

func TestAuditExportIsFormulaSafeAndAudited(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	// The attempted email of a failed login is attacker-controlled text; in a spreadsheet it must stay text.
	f.want(f.call(nil, "POST", "/auth/login", map[string]any{"email": "=HYPERLINK(\"http://evil.example\",\"x\")", "password": "x"}), 401, "failed login")
	status, header, body := f.rawGet(admin, "/audit-events/export?category=auth")
	if status != 200 || header.Get("Content-Type") != "text/csv; charset=utf-8" || header.Get("Content-Disposition") == "" {
		t.Fatalf("export = %d %v", status, header)
	}
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if lines[0] != "time,actor,action,target_type,target_id,ip,details" || len(lines) != 2 {
		t.Fatalf("csv = %q", body)
	}
	if !strings.Contains(lines[1], `"'=hyperlink(`) || strings.Contains(lines[1], ",=hyperlink") {
		t.Fatalf("a formula cell must be defused: %s", lines[1])
	}
	// Reading the trail in bulk leaves a mark, with the row count.
	if e := lastAudit(t, f, admin, "audit.exported"); e["details"].(map[string]any)["rows"] != 1.0 || e["actor_id"] != admin.id {
		t.Fatalf("export entry = %v", e)
	}
}
