package httpserver_test

import (
	"net/http"
	"sort"
	"strings"
	"testing"
)

// Customer eligibility lookup (B2-08). Real sessions and a real database. Eligibility is not content access:
// nothing here creates a membership, a Space share or an item right, and the tests check that.

func eligibleIDs(t *testing.T, r reply) []string {
	t.Helper()
	items, ok := r.body["items"].([]any)
	if !ok {
		t.Fatalf("response has no items: status %d body %v", r.status, r.body)
	}
	ids := []string{}
	for _, it := range items {
		ids = append(ids, it.(map[string]any)["id"].(string))
	}
	sort.Strings(ids)
	return ids
}

func sameIDs(got []string, want ...string) bool {
	sort.Strings(want)
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func (f *fixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, sql, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func TestCustomerEligibilityLookup(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	regularAB := f.person("regular")
	regularA := f.person("regular")
	regularNone := f.person("regular")
	regularRevoked := f.person("regular")
	external := f.person("external")
	deactivated := f.person("regular")

	a := f.createClient(admin, "ELIG-A")["id"].(string)
	b := f.createClient(admin, "ELIG-B")["id"].(string)
	c := f.createClient(admin, "ELIG-C")["id"].(string)
	for _, grant := range []struct {
		client string
		user   *person
	}{
		{a, regularAB}, {b, regularAB}, {c, regularAB}, {a, regularA}, {a, deactivated},
	} {
		f.want(f.call(admin, "POST", "/clients/"+grant.client+"/members", map[string]any{"user_id": grant.user.id}), http.StatusOK, "grant")
	}
	f.want(f.call(admin, "POST", "/clients/"+c+"/archive", nil), http.StatusOK, "archive C")
	f.want(f.call(admin, "POST", "/users/"+deactivated.id+"/deactivate", nil), http.StatusOK, "deactivate")
	// The deactivation revokes nothing; the session check refuses the user, which is the rule EL-10 checks.
	// External users cannot be granted, so this row is written directly to test that the lookup still refuses them.
	f.exec(`INSERT INTO client_memberships(client_id,user_id,granted_by) VALUES ($1,$2,$3)`, a, external.id, admin.id)

	const path = "/clients/eligible"

	t.Run("EL-01 unauthenticated lookup is denied", func(t *testing.T) {
		f.want(f.call(nil, "GET", path, nil), http.StatusUnauthorized, "anonymous")
	})

	t.Run("EL-02 regular user sees only active memberships, archived excluded", func(t *testing.T) {
		if got := eligibleIDs(t, f.call(regularAB, "GET", path, nil)); !sameIDs(got, a, b) {
			t.Fatalf("regular with A and B sees %v, want A and B only", got)
		}
		if got := eligibleIDs(t, f.call(regularA, "GET", path, nil)); !sameIDs(got, a) {
			t.Fatalf("regular with A sees %v, want A only", got)
		}
	})

	t.Run("EL-03 user without membership receives an empty list", func(t *testing.T) {
		r := f.call(regularNone, "GET", path, nil)
		f.want(r, http.StatusOK, "no membership")
		if got := eligibleIDs(t, r); len(got) != 0 {
			t.Fatalf("no membership sees %v", got)
		}
	})

	t.Run("EL-04 revoked membership does not provide eligibility; EL-05 regrant restores it once", func(t *testing.T) {
		f.want(f.call(admin, "POST", "/clients/"+a+"/members", map[string]any{"user_id": regularRevoked.id}), http.StatusOK, "grant")
		if got := eligibleIDs(t, f.call(regularRevoked, "GET", path, nil)); !sameIDs(got, a) {
			t.Fatalf("before revoke sees %v, want A", got)
		}
		f.want(f.call(admin, "DELETE", "/clients/"+a+"/members/"+regularRevoked.id, nil), http.StatusOK, "revoke")
		if got := eligibleIDs(t, f.call(regularRevoked, "GET", path, nil)); len(got) != 0 {
			t.Fatalf("revoked user still sees %v", got)
		}
		f.want(f.call(admin, "POST", "/clients/"+a+"/members", map[string]any{"user_id": regularRevoked.id}), http.StatusOK, "regrant")
		if got := eligibleIDs(t, f.call(regularRevoked, "GET", path, nil)); !sameIDs(got, a) {
			t.Fatalf("after regrant sees %v, want A exactly once", got)
		}
		if rows := f.count(`SELECT count(*) FROM client_memberships WHERE client_id=$1 AND user_id=$2`, a, regularRevoked.id); rows != 2 {
			t.Fatalf("history rows = %d, want 2 (revoked and active), so the duplicate check means something", rows)
		}
	})

	t.Run("EL-06 archived customer is excluded despite an active membership", func(t *testing.T) {
		if got := eligibleIDs(t, f.call(regularAB, "GET", path, nil)); sameIDs(got, c) || len(got) != 2 {
			t.Fatalf("archived C was returned: %v", got)
		}
	})

	t.Run("EL-07 administrator default lookup does not imply universal membership", func(t *testing.T) {
		if got := eligibleIDs(t, f.call(admin, "GET", path, nil)); len(got) != 0 {
			t.Fatalf("admin without membership sees %v through the default lookup", got)
		}
	})

	t.Run("EL-08 administrative lookup returns active customers, grants nothing, EL-14 nothing is created", func(t *testing.T) {
		before := f.count(`SELECT count(*) FROM client_memberships WHERE user_id=$1`, admin.id)
		audit := f.count(`SELECT count(*) FROM audit_events`)
		r := f.call(admin, "GET", path+"?scope=administrative", nil)
		f.want(r, http.StatusOK, "administrative lookup")
		if got := eligibleIDs(t, r); !sameIDs(got, a, b) {
			t.Fatalf("administrative lookup = %v, want active A and B (archived C excluded)", got)
		}
		if after := f.count(`SELECT count(*) FROM client_memberships WHERE user_id=$1`, admin.id); after != before {
			t.Fatalf("administrative lookup changed memberships: %d -> %d", before, after)
		}
		if after := f.count(`SELECT count(*) FROM audit_events`); after != audit {
			t.Fatalf("lookup wrote audit rows: %d -> %d", audit, after)
		}
	})

	t.Run("EL-09 non-administrator cannot request the administrative lookup", func(t *testing.T) {
		f.want(f.call(regularAB, "GET", path+"?scope=administrative", nil), http.StatusForbidden, "regular administrative")
		f.want(f.call(external, "GET", path+"?scope=administrative", nil), http.StatusForbidden, "external administrative")
		f.want(f.call(nil, "GET", path+"?scope=administrative", nil), http.StatusUnauthorized, "anonymous administrative")
	})

	t.Run("EL-10 external and deactivated users receive nothing eligible", func(t *testing.T) {
		r := f.call(external, "GET", path, nil)
		f.want(r, http.StatusOK, "external lookup")
		if got := eligibleIDs(t, r); len(got) != 0 {
			t.Fatalf("external user with a direct membership row sees %v", got)
		}
		f.want(f.call(deactivated, "GET", path, nil), http.StatusUnauthorized, "deactivated user")
	})

	t.Run("EL-11 search and limits are applied in the database, and wildcards stay literal", func(t *testing.T) {
		if got := eligibleIDs(t, f.call(regularAB, "GET", path+"?q=ELIG-B", nil)); !sameIDs(got, b) {
			t.Fatalf("search ELIG-B = %v, want B", got)
		}
		if got := eligibleIDs(t, f.call(regularA, "GET", path+"?q=ELIG", nil)); !sameIDs(got, a) {
			t.Fatalf("search ELIG for a user with only A = %v; B must not be counted or shown", got)
		}
		if got := eligibleIDs(t, f.call(regularAB, "GET", path+"?q=ELIG-C", nil)); len(got) != 0 {
			t.Fatalf("archived C matched search: %v", got)
		}
		if got := eligibleIDs(t, f.call(regularAB, "GET", path+"?q=%25", nil)); len(got) != 0 {
			t.Fatalf("a bare percent sign matched %v; it must be literal", got)
		}
		if got := eligibleIDs(t, f.call(regularAB, "GET", path+"?q=_", nil)); len(got) != 0 {
			t.Fatalf("an underscore matched %v; it must be literal", got)
		}
		if r := eligibleIDs(t, f.call(regularAB, "GET", path+"?limit=1", nil)); len(r) != 1 {
			t.Fatalf("limit=1 returned %d", len(r))
		}
		f.want(f.call(regularAB, "GET", path+"?limit=201", nil), http.StatusUnprocessableEntity, "limit above cap")
		f.want(f.call(regularAB, "GET", path+"?q="+strings.Repeat("x", 101), nil), http.StatusUnprocessableEntity, "search too long")
		f.want(f.call(regularAB, "GET", path+"?scope=everything", nil), http.StatusBadRequest, "unknown scope")
	})

	t.Run("EL-12 response carries only picker metadata", func(t *testing.T) {
		r := f.call(regularAB, "GET", path, nil)
		f.want(r, http.StatusOK, "lookup")
		allowed := map[string]bool{"id": true, "code": true, "name": true, "short_name": true}
		for _, it := range r.body["items"].([]any) {
			for key := range it.(map[string]any) {
				if !allowed[key] {
					t.Fatalf("picker option carries %q", key)
				}
			}
		}
	})

	t.Run("EL-13 the management API stays administrator-only", func(t *testing.T) {
		for _, op := range clientOps {
			p := clientPath(op.path, a, regularAB.id)
			f.want(f.call(regularAB, op.method, p, clientBody(op.method, op.path)), http.StatusForbidden, op.method+" "+p)
		}
	})

	t.Run("EL-14 lookups create no membership or audit rows", func(t *testing.T) {
		memberships := f.count(`SELECT count(*) FROM client_memberships`)
		for _, p := range []*person{regularAB, regularA, regularNone, admin} {
			f.call(p, "GET", path, nil)
			f.call(p, "GET", path+"?q=ELIG", nil)
		}
		if after := f.count(`SELECT count(*) FROM client_memberships`); after != memberships {
			t.Fatalf("lookups changed client memberships: %d -> %d", memberships, after)
		}
	})

	t.Run("EL-15 the static route does not shadow or open the customer detail route", func(t *testing.T) {
		f.want(f.call(regularAB, "GET", path, nil), http.StatusOK, "static route for a regular user")
		f.want(f.call(regularAB, "GET", "/clients/"+a, nil), http.StatusForbidden, "detail route stays admin-only")
		f.want(f.call(admin, "GET", "/clients/"+a, nil), http.StatusOK, "detail route for admin")
	})
}
