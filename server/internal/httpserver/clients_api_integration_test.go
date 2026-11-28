package httpserver_test

import (
	"fmt"
	"net/http"
	"testing"
)

// Customer Master HTTP API and audit trail (B2-04..06). Uses the shared fixture: real sessions, real database.

const zeroUUID = "00000000-0000-4000-8000-000000000404"

// clientOps lists the eight operations; {c} and {u} are replaced by the caller.
var clientOps = []struct{ method, path string }{
	{"GET", "/clients"},
	{"POST", "/clients"},
	{"GET", "/clients/{c}"},
	{"PATCH", "/clients/{c}"},
	{"POST", "/clients/{c}/archive"},
	{"GET", "/clients/{c}/members"},
	{"POST", "/clients/{c}/members"},
	{"DELETE", "/clients/{c}/members/{u}"},
}

func clientPath(path, client, user string) string {
	out := path
	for _, pair := range [][2]string{{"{c}", client}, {"{u}", user}} {
		for i := 0; i+len(pair[0]) <= len(out); i++ {
			if out[i:i+len(pair[0])] == pair[0] {
				out = out[:i] + pair[1] + out[i+len(pair[0]):]
				break
			}
		}
	}
	return out
}

func clientBody(method, path string) any {
	switch {
	case method == "POST" && path == "/clients":
		return map[string]any{"code": "PROBE-" + path[len(path)-3:], "name": "Probe"}
	case method == "PATCH":
		return map[string]any{"name": "Probe"}
	case method == "POST" && len(path) > 0 && path[len(path)-8:] == "/members":
		return map[string]any{"user_id": zeroUUID}
	}
	return nil
}

// auditCount counts audit rows for one action and target. The actor filter is optional.
func auditCount(f *fixture, action, target string, actor string) int {
	f.t.Helper()
	var n int
	err := f.pool.QueryRow(f.ctx, `SELECT count(*)::int FROM audit_events WHERE action=$1 AND target_id=$2 AND ($3='' OR actor_id=NULLIF($3,'')::uuid)`,
		action, target, actor).Scan(&n)
	if err != nil {
		f.t.Fatal(err)
	}
	return n
}

func (f *fixture) createClient(p *person, code string) map[string]any {
	f.t.Helper()
	r := f.call(p, "POST", "/clients", map[string]any{"code": code, "name": "Customer " + code})
	f.want(r, http.StatusCreated, "create "+code)
	return r.body
}

// API-01, API-02, API-03, API-04, API-05: every operation refuses everyone but a signed-in Workspace Admin.
func TestClientAPIRefusesNonAdministrators(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	client := f.createClient(admin, "AUTH-PROBE")["id"].(string)
	regular := f.person("regular")
	withMembership := f.person("regular")
	grantRes := f.call(admin, "POST", "/clients/"+client+"/members", map[string]any{"user_id": withMembership.id})
	f.want(grantRes, http.StatusOK, "grant eligibility")
	space := f.space("Clients auth")
	fullHolder := f.person("regular")
	f.share(space, "user", fullHolder.id, "full")
	spaceAdmin := f.person("regular")
	f.exec(`INSERT INTO space_admins(space_id,user_id,granted_by) VALUES ($1,$2,$3)`, space, spaceAdmin.id, admin.id)
	editor := f.person("regular")
	f.share(space, "user", editor.id, "editor")
	deactivated := f.person("admin")
	f.want(f.call(admin, "POST", "/users/"+deactivated.id+"/deactivate", nil), http.StatusOK, "deactivate")

	denied := map[string]struct {
		p    *person
		want func(int) bool
	}{
		"unauthenticated":           {nil, func(s int) bool { return s == 401 }},
		"regular":                   {regular, func(s int) bool { return s == 403 }},
		"regular with membership":   {withMembership, func(s int) bool { return s == 403 }},
		"space admin":               {spaceAdmin, func(s int) bool { return s == 403 }},
		"full resource grant":       {fullHolder, func(s int) bool { return s == 403 }},
		"editor resource grant":     {editor, func(s int) bool { return s == 403 }},
		"deactivated administrator": {deactivated, func(s int) bool { return s == 401 || s == 403 }},
	}
	for name, tc := range denied {
		for _, op := range clientOps {
			path := clientPath(op.path, client, withMembership.id)
			r := f.call(tc.p, op.method, path, clientBody(op.method, op.path))
			if !tc.want(r.status) {
				t.Fatalf("%s %s as %s: status %d, want a refusal", op.method, path, name, r.status)
			}
			if r.body["id"] != nil && r.status < 400 {
				t.Fatalf("%s as %s returned data", name, op.method)
			}
		}
	}

	// API-06: the administrator can use the list operation.
	f.want(f.call(admin, "GET", "/clients", nil), http.StatusOK, "admin lists clients")

	// Membership granted to the regular user did not make them an administrator.
	f.want(f.call(withMembership, "GET", "/clients", nil), http.StatusForbidden, "membership is not administration")
}

// API-07..API-17: the contract, the domain rules through HTTP, and the response shape.
func TestClientAPIContract(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	admin2 := f.person("admin")
	member := f.person("regular")
	external := f.person("external")

	t.Run("API-07 create and retrieve", func(t *testing.T) {
		c := f.createClient(admin, "  api-one ")
		if c["code"] != "API-ONE" || c["status"] != "active" || c["created_by"] != admin.id {
			t.Fatalf("created = %v", c)
		}
		got := f.call(admin, "GET", "/clients/"+c["id"].(string), nil)
		f.want(got, http.StatusOK, "get")
		if got.body["code"] != "API-ONE" {
			t.Fatalf("get = %v", got.body)
		}
	})

	t.Run("API-08 update changes only name and short name; code and id are refused", func(t *testing.T) {
		c := f.createClient(admin, "API-TWO")
		id := c["id"].(string)
		f.want(f.call(admin, "PATCH", "/clients/"+id, map[string]any{"name": "Renamed", "short_name": "RN"}), http.StatusOK, "rename")
		f.want(f.call(admin, "PATCH", "/clients/"+id, map[string]any{"code": "CHANGED"}), http.StatusBadRequest, "code in PATCH")
		f.want(f.call(admin, "PATCH", "/clients/"+id, map[string]any{"id": zeroUUID}), http.StatusBadRequest, "id in PATCH")
		if n := f.queryID(`SELECT code FROM clients WHERE id=$1`, id); n != "API-TWO" {
			t.Fatalf("code changed to %s", n)
		}
	})

	t.Run("API-09 duplicate normalized code is a conflict", func(t *testing.T) {
		f.createClient(admin, "API-DUP")
		r := f.call(admin, "POST", "/clients", map[string]any{"code": " api-dup ", "name": "Again"})
		f.want(r, http.StatusConflict, "duplicate")
		if r.body["code"] != "duplicate_code" {
			t.Fatalf("problem code = %v", r.body["code"])
		}
	})

	t.Run("API-10 malformed ids and input are refused with the right status", func(t *testing.T) {
		f.want(f.call(admin, "GET", "/clients/not-a-uuid", nil), http.StatusBadRequest, "bad id")
		f.want(f.call(admin, "POST", "/clients/not-a-uuid/archive", nil), http.StatusBadRequest, "bad archive id")
		f.want(f.call(admin, "POST", "/clients", map[string]any{"code": "X-BAD", "name": "  "}), http.StatusUnprocessableEntity, "blank name")
		f.want(f.call(admin, "POST", "/clients", map[string]any{"code": "X-BAD", "name": "N", "surprise": 1}), http.StatusBadRequest, "unknown field")
		f.want(f.call(admin, "GET", "/clients?limit=abc", nil), http.StatusBadRequest, "limit not a number")
		f.want(f.call(admin, "GET", "/clients?limit=500", nil), http.StatusUnprocessableEntity, "limit above cap")
		f.want(f.call(admin, "GET", "/clients?status=deleted", nil), http.StatusUnprocessableEntity, "unknown status filter")
		f.want(f.call(admin, "GET", "/clients/"+zeroUUID, nil), http.StatusNotFound, "missing client")
	})

	t.Run("API-11 list filters by state", func(t *testing.T) {
		live := f.createClient(admin, "API-LIVE")
		old := f.createClient(admin, "API-OLD")
		f.want(f.call(admin, "POST", "/clients/"+old["id"].(string)+"/archive", nil), http.StatusOK, "archive")
		count := func(q string) (int, map[string]bool) {
			r := f.call(admin, "GET", "/clients?"+q, nil)
			f.want(r, http.StatusOK, "list "+q)
			ids := map[string]bool{}
			for _, it := range r.body["items"].([]any) {
				ids[it.(map[string]any)["id"].(string)] = true
			}
			return len(ids), ids
		}
		_, activeIDs := count("status=active")
		_, archivedIDs := count("status=archived")
		_, allIDs := count("")
		if !activeIDs[live["id"].(string)] || activeIDs[old["id"].(string)] {
			t.Fatal("active filter wrong")
		}
		if !archivedIDs[old["id"].(string)] || archivedIDs[live["id"].(string)] {
			t.Fatal("archived filter wrong")
		}
		if !allIDs[live["id"].(string)] || !allIDs[old["id"].(string)] {
			t.Fatal("all must include both")
		}
	})

	t.Run("API-12 archive keeps identity and membership history", func(t *testing.T) {
		c := f.createClient(admin, "API-ARCH")
		id := c["id"].(string)
		f.want(f.call(admin, "POST", "/clients/"+id+"/members", map[string]any{"user_id": member.id}), http.StatusOK, "grant")
		r := f.call(admin, "POST", "/clients/"+id+"/archive", nil)
		f.want(r, http.StatusOK, "archive")
		if r.body["id"] != id || r.body["code"] != "API-ARCH" || r.body["status"] != "archived" {
			t.Fatalf("archived = %v", r.body)
		}
		members := f.call(admin, "GET", "/clients/"+id+"/members", nil)
		f.want(members, http.StatusOK, "members after archive")
		if n := len(members.body["items"].([]any)); n != 1 {
			t.Fatalf("membership lost by archive: %d rows", n)
		}
	})

	t.Run("API-13 archived customer rejects update and new grants", func(t *testing.T) {
		c := f.createClient(admin, "API-RO")
		id := c["id"].(string)
		f.want(f.call(admin, "POST", "/clients/"+id+"/archive", nil), http.StatusOK, "archive")
		r := f.call(admin, "PATCH", "/clients/"+id, map[string]any{"name": "Nope"})
		f.want(r, http.StatusConflict, "update archived")
		if r.body["code"] != "customer_archived" {
			t.Fatalf("problem code = %v", r.body["code"])
		}
		g := f.call(admin, "POST", "/clients/"+id+"/members", map[string]any{"user_id": member.id})
		f.want(g, http.StatusConflict, "grant to archived")
	})

	t.Run("API-14 grant, revoke and regrant keep history", func(t *testing.T) {
		c := f.createClient(admin, "API-HIST")
		id := c["id"].(string)
		u := f.person("regular")
		first := f.call(admin, "POST", "/clients/"+id+"/members", map[string]any{"user_id": u.id})
		f.want(first, http.StatusOK, "grant")
		f.want(f.call(admin, "DELETE", "/clients/"+id+"/members/"+u.id, nil), http.StatusOK, "revoke")
		again := f.call(admin, "POST", "/clients/"+id+"/members", map[string]any{"user_id": u.id})
		f.want(again, http.StatusOK, "regrant")
		if again.body["id"] == first.body["id"] {
			t.Fatal("regrant must be a new row")
		}
		active := f.call(admin, "GET", "/clients/"+id+"/members", nil)
		all := f.call(admin, "GET", "/clients/"+id+"/members?include_revoked=true", nil)
		if len(active.body["items"].([]any)) != 1 || len(all.body["items"].([]any)) != 2 {
			t.Fatalf("active %d, with history %d", len(active.body["items"].([]any)), len(all.body["items"].([]any)))
		}
		f.want(f.call(admin, "GET", "/clients/"+id+"/members?include_revoked=maybe", nil), http.StatusBadRequest, "bad flag")
	})

	t.Run("API-15 ineligible and nonexistent users are refused safely", func(t *testing.T) {
		c := f.createClient(admin, "API-ELIG")
		id := c["id"].(string)
		r := f.call(admin, "POST", "/clients/"+id+"/members", map[string]any{"user_id": external.id})
		f.want(r, http.StatusUnprocessableEntity, "external user")
		if r.body["code"] != "user_not_eligible" {
			t.Fatalf("problem code = %v", r.body["code"])
		}
		f.want(f.call(admin, "POST", "/clients/"+id+"/members", map[string]any{"user_id": zeroUUID}), http.StatusNotFound, "nonexistent user")
		f.want(f.call(admin, "POST", "/clients/"+id+"/members", map[string]any{"user_id": "bad"}), http.StatusBadRequest, "malformed user id")
	})

	t.Run("API-16 repeated mutations change nothing", func(t *testing.T) {
		c := f.createClient(admin, "API-IDEM")
		id := c["id"].(string)
		u := f.person("regular")
		g1 := f.call(admin, "POST", "/clients/"+id+"/members", map[string]any{"user_id": u.id})
		g2 := f.call(admin2, "POST", "/clients/"+id+"/members", map[string]any{"user_id": u.id})
		f.want(g2, http.StatusOK, "repeat grant")
		if g1.body["id"] != g2.body["id"] || g2.body["granted_by"] != admin.id {
			t.Fatalf("repeat grant changed the row: %v vs %v", g1.body, g2.body)
		}
		r1 := f.call(admin, "DELETE", "/clients/"+id+"/members/"+u.id, nil)
		r2 := f.call(admin2, "DELETE", "/clients/"+id+"/members/"+u.id, nil)
		f.want(r2, http.StatusOK, "repeat revoke")
		if r1.body["revoked_at"] != r2.body["revoked_at"] || r2.body["revoked_by"] != admin.id {
			t.Fatalf("repeat revoke rewrote history: %v vs %v", r1.body, r2.body)
		}
		a1 := f.call(admin, "POST", "/clients/"+id+"/archive", nil)
		a2 := f.call(admin2, "POST", "/clients/"+id+"/archive", nil)
		f.want(a2, http.StatusOK, "repeat archive")
		if a1.body["archived_at"] != a2.body["archived_at"] || a2.body["archived_by"] != admin.id {
			t.Fatalf("repeat archive overwrote metadata")
		}
	})

	t.Run("API-17 responses carry no credentials or unrelated user data", func(t *testing.T) {
		c := f.createClient(admin, "API-SAFE")
		id := c["id"].(string)
		u := f.person("regular")
		f.want(f.call(admin, "POST", "/clients/"+id+"/members", map[string]any{"user_id": u.id}), http.StatusOK, "grant")
		allowed := map[string]bool{"id": true, "client_id": true, "user_id": true, "user_name": true, "status": true,
			"granted_at": true, "granted_by": true, "revoked_at": true, "revoked_by": true}
		list := f.call(admin, "GET", "/clients/"+id+"/members", nil)
		for _, it := range list.body["items"].([]any) {
			for k := range it.(map[string]any) {
				if !allowed[k] {
					t.Fatalf("membership exposes %q", k)
				}
			}
		}
		for k := range c {
			switch k {
			case "password_hash", "email", "tokens", "sessions":
				t.Fatalf("client exposes %q", k)
			}
		}
	})
}

// AUD-01..AUD-06: audit events are correct, come from the session, are not duplicated, and move with the change.
func TestClientAuditTrail(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	admin2 := f.person("admin")
	regular := f.person("regular")
	u := f.person("regular")

	t.Run("AUD-01 each real mutation records its action and actor", func(t *testing.T) {
		c := f.createClient(admin, "AUD-ONE")
		id := c["id"].(string)
		if n := auditCount(f, "client.created", id, admin.id); n != 1 {
			t.Fatalf("client.created rows = %d", n)
		}
		f.want(f.call(admin, "PATCH", "/clients/"+id, map[string]any{"name": "Audit renamed"}), http.StatusOK, "update")
		if n := auditCount(f, "client.updated", id, admin.id); n != 1 {
			t.Fatalf("client.updated rows = %d", n)
		}
		f.want(f.call(admin, "POST", "/clients/"+id+"/members", map[string]any{"user_id": u.id}), http.StatusOK, "grant")
		if n := auditCount(f, "client.member_granted", id, admin.id); n != 1 {
			t.Fatalf("client.member_granted rows = %d", n)
		}
		f.want(f.call(admin, "DELETE", "/clients/"+id+"/members/"+u.id, nil), http.StatusOK, "revoke")
		if n := auditCount(f, "client.member_revoked", id, admin.id); n != 1 {
			t.Fatalf("client.member_revoked rows = %d", n)
		}
		f.want(f.call(admin, "POST", "/clients/"+id+"/archive", nil), http.StatusOK, "archive")
		if n := auditCount(f, "client.archived", id, admin.id); n != 1 {
			t.Fatalf("client.archived rows = %d", n)
		}
		var details string
		if err := f.pool.QueryRow(f.ctx, `SELECT details::text FROM audit_events WHERE action='client.member_granted' AND target_id=$1`, id).Scan(&details); err != nil {
			t.Fatal(err)
		}
		if !containsAll(details, u.id) {
			t.Fatalf("grant details lack the target user: %s", details)
		}
	})

	t.Run("AUD-02 the actor is the session, not a field in the request", func(t *testing.T) {
		r := f.call(admin, "POST", "/clients", map[string]any{"code": "AUD-FORGE", "name": "X", "created_by": admin2.id})
		f.want(r, http.StatusBadRequest, "forged created_by refused")
		if n := f.queryID(`SELECT count(*)::text FROM clients WHERE code='AUD-FORGE'`); n != "0" {
			t.Fatal("a refused request created a customer")
		}
		c := f.createClient(admin2, "AUD-TWO")
		if n := auditCount(f, "client.created", c["id"].(string), admin2.id); n != 1 {
			t.Fatalf("actor of the second administrator's create = %d", n)
		}
	})

	t.Run("AUD-03 operations that change nothing write no event", func(t *testing.T) {
		c := f.createClient(admin, "AUD-NOOP")
		id := c["id"].(string)
		f.want(f.call(admin, "POST", "/clients/"+id+"/members", map[string]any{"user_id": u.id}), http.StatusOK, "grant")
		f.want(f.call(admin, "POST", "/clients/"+id+"/archive", nil), http.StatusOK, "archive")
		before := f.queryID(`SELECT count(*)::text FROM audit_events WHERE target_id=$1`, id)
		f.want(f.call(admin2, "POST", "/clients/"+id+"/archive", nil), http.StatusOK, "repeat archive")
		f.want(f.call(admin2, "PATCH", "/clients/"+id, map[string]any{"name": "AUD-NOOP"}), http.StatusConflict, "update archived is refused")
		if after := f.queryID(`SELECT count(*)::text FROM audit_events WHERE target_id=$1`, id); after != before {
			t.Fatalf("no-op calls wrote %s new events", after)
		}
		live := f.createClient(admin, "AUD-NOOP2")
		lid := live["id"].(string)
		f.want(f.call(admin, "POST", "/clients/"+lid+"/members", map[string]any{"user_id": u.id}), http.StatusOK, "grant")
		before2 := f.queryID(`SELECT count(*)::text FROM audit_events WHERE target_id=$1`, lid)
		f.want(f.call(admin2, "POST", "/clients/"+lid+"/members", map[string]any{"user_id": u.id}), http.StatusOK, "repeat grant")
		f.want(f.call(admin2, "PATCH", "/clients/"+lid, map[string]any{"name": "Customer AUD-NOOP2"}), http.StatusOK, "same name")
		if after := f.queryID(`SELECT count(*)::text FROM audit_events WHERE target_id=$1`, lid); after != before2 {
			t.Fatalf("repeat grant or same-name update wrote %s new events (before %s)", after, before2)
		}
	})

	t.Run("AUD-04 refused and failed mutations write no success event", func(t *testing.T) {
		before := f.queryID(`SELECT count(*)::text FROM audit_events WHERE action LIKE 'client.%'`)
		f.want(f.call(regular, "POST", "/clients", map[string]any{"code": "AUD-DENIED", "name": "X"}), http.StatusForbidden, "regular create")
		f.createClient(admin, "AUD-DUP")
		f.want(f.call(admin, "POST", "/clients", map[string]any{"code": "AUD-DUP", "name": "X"}), http.StatusConflict, "duplicate")
		after := f.queryID(`SELECT count(*)::text FROM audit_events WHERE action LIKE 'client.%'`)
		if after != fmt.Sprint(atoi(before)+1) {
			t.Fatalf("client events %s -> %s: want exactly one (the successful create)", before, after)
		}
	})

	t.Run("AUD-05 a failed audit write aborts the mutation", func(t *testing.T) {
		f.exec(`CREATE OR REPLACE FUNCTION fail_client_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$`)
		f.exec(`CREATE TRIGGER fail_client_audit BEFORE INSERT ON audit_events FOR EACH ROW WHEN (NEW.action = 'client.created') EXECUTE FUNCTION fail_client_audit()`)
		defer f.exec(`DROP TRIGGER IF EXISTS fail_client_audit ON audit_events`)
		r := f.call(admin, "POST", "/clients", map[string]any{"code": "AUD-ATOM", "name": "X"})
		f.want(r, http.StatusInternalServerError, "audit failure")
		if n := f.queryID(`SELECT count(*)::text FROM clients WHERE code='AUD-ATOM'`); n != "0" {
			t.Fatal("a customer committed without its audit event")
		}
	})

	t.Run("AUD-06 membership changes do not touch shares, admins or groups", func(t *testing.T) {
		c := f.createClient(admin, "AUD-RES")
		id := c["id"].(string)
		space := f.space("Resource guard")
		count := func() string {
			return f.queryID(`SELECT (SELECT count(*) FROM shares)||'/'||(SELECT count(*) FROM space_admins)||'/'||(SELECT count(*) FROM group_members)`)
		}
		before := count()
		f.want(f.call(admin, "POST", "/clients/"+id+"/members", map[string]any{"user_id": u.id}), http.StatusOK, "grant")
		f.want(f.call(admin, "DELETE", "/clients/"+id+"/members/"+u.id, nil), http.StatusOK, "revoke")
		if after := count(); after != before {
			t.Fatalf("membership changed resource rows: %s -> %s", before, after)
		}
		_ = space
	})
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		found := false
		for i := 0; i+len(p) <= len(s); i++ {
			if s[i:i+len(p)] == p {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func atoi(s string) int {
	var n int
	fmt.Sscan(s, &n)
	return n
}

// AUD-07: a Workspace Admin without a grant on a Space gains no item or Space visibility from customer management or from
// granting themselves membership. Membership is eligibility only (D-17.1).
func TestClientManagementGrantsNoContentAccess(t *testing.T) {
	f := newFixture(t)
	unlinked := f.unlinkedAdmin()
	owner := f.person("regular")
	secret := f.spaceWithoutAdmins("Client content")
	status := f.queryID("SELECT id::text FROM statuses WHERE grp='active' ORDER BY name LIMIT 1")
	item := f.queryID(`INSERT INTO items(type,title,created_by,status_id) VALUES ('task','Confidential task',$1,$2) RETURNING id::text`, owner.id, status)
	f.exec(`INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')`, item, secret)

	sharesFor := func() string {
		return f.queryID(`SELECT count(*)::text FROM shares WHERE principal_id=$1`, unlinked.id)
	}
	beforeShares := sharesFor()
	f.want(f.call(unlinked, "GET", "/items/"+item, nil), http.StatusNotFound, "no grant, no item before")
	f.want(f.call(unlinked, "GET", "/spaces/"+secret, nil), http.StatusNotFound, "no grant, no space before")

	c := f.createClient(unlinked, "AUD-NOACCESS")
	id := c["id"].(string)
	f.want(f.call(unlinked, "POST", "/clients/"+id+"/members", map[string]any{"user_id": unlinked.id}), http.StatusOK, "self-membership")
	f.want(f.call(unlinked, "POST", "/clients/"+id+"/archive", nil), http.StatusOK, "archive")

	f.want(f.call(unlinked, "GET", "/items/"+item, nil), http.StatusNotFound, "still no item after customer work")
	f.want(f.call(unlinked, "GET", "/spaces/"+secret, nil), http.StatusNotFound, "still no space after customer work")
	if after := sharesFor(); after != beforeShares {
		t.Fatalf("customer work created %s share rows for the admin (was %s)", after, beforeShares)
	}
}
