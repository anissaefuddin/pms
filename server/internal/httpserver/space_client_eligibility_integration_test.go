package httpserver_test

import (
	"net/http"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/policy"
)

// B3-04: client-owned Space access needs active client membership AND the existing resource grant (ADR §7k).
// Real sessions and a real database. Decisions come from the central policy entry points, and HTTP confirms the visible result.

func actorOf2(p *person) policy.Actor {
	return policy.Actor{ID: p.id, Type: policy.UserType(p.userType)}
}

func decideOn(t *testing.T, f *fixture, p *person, space string, need policy.Capability) policy.Decision {
	t.Helper()
	d, _, err := policy.CanOnSpace(f.ctx, f.pool, actorOf2(p), need, space)
	if err != nil {
		t.Fatalf("CanOnSpace: %v", err)
	}
	return d
}

func (f *fixture) addMember(admin *person, client string, user *person) {
	f.t.Helper()
	f.want(f.call(admin, "POST", "/clients/"+client+"/members", map[string]any{"user_id": user.id}), http.StatusOK, "grant membership")
}

func (f *fixture) revokeMember(admin *person, client string, user *person) {
	f.t.Helper()
	f.want(f.call(admin, "DELETE", "/clients/"+client+"/members/"+user.id, nil), http.StatusOK, "revoke membership")
}

// SP-01..SP-28 (ADR §7k). Subtests share one fixture and run in order; revoke and regrant come last.
func TestClientSpaceEligibility(t *testing.T) {
	base := newFixture(t)
	f := base
	admin := f.person("admin")
	clientA := f.createClient(admin, "SP-A")["id"].(string)
	clientB := f.createClient(admin, "SP-B")["id"].(string)
	clientC := f.createClient(admin, "SP-C")["id"].(string)
	a1 := f.scopedSpace("A1", "client", clientA)
	a2 := f.scopedSpace("A2", "client", clientA)
	b1 := f.scopedSpace("B1", "client", clientB)
	c1 := f.scopedSpace("C1", "client", clientC)
	internal := f.scopedSpace("I1", "internal", "")
	unresolved := f.unresolvedSpace("U1") // explicit negative fixture (fail closed)

	full := f.person("regular")
	editor := f.person("regular")
	read := f.person("regular")
	limited := f.person("regular")
	noGrant := f.person("regular")
	fullNoMember := f.person("regular")
	memberFullOnB1 := f.person("regular")
	memberAB := f.person("regular")
	revoked := f.person("regular")
	groupMember := f.person("regular")
	adminNoGrant := f.person("admin")
	adminMember := f.person("admin")
	adminGrant := f.person("admin")
	spaceAdminNoMember := f.person("regular")
	spaceAdminMember := f.person("regular")
	deactivated := f.person("regular")
	memberC := f.person("regular")
	internalFull := f.person("regular")
	personalOwner := f.person("regular")
	personal := f.personalSpace(personalOwner)

	// Memberships (B3-04 eligibility). Each one is an explicit row granted by the Workspace Admin through the API.
	for _, u := range []*person{full, editor, read, limited, noGrant, memberFullOnB1, memberAB, revoked, spaceAdminMember, deactivated, adminMember, memberC} {
		f.addMember(admin, clientA, u)
	}
	f.addMember(admin, clientB, memberAB)
	f.addMember(admin, clientC, memberC)

	// Existing resource grants, unchanged by eligibility.
	f.share(a1, "user", full.id, "full")
	f.share(a1, "user", editor.id, "editor")
	f.share(a1, "user", read.id, "read")
	f.share(a1, "user", limited.id, "limited")
	f.share(a1, "user", fullNoMember.id, "full")
	f.share(a1, "user", revoked.id, "full")
	f.share(a1, "user", memberAB.id, "full")
	f.share(b1, "user", memberFullOnB1.id, "full")
	f.share(a1, "user", deactivated.id, "full")
	f.share(c1, "user", memberC.id, "full")
	f.share(internal, "user", internalFull.id, "full")
	f.share(unresolved, "user", full.id, "full")
	f.share(a1, "user", adminGrant.id, "full")
	group := f.group("SP group", groupMember)
	f.share(a1, "group", group, "full")
	f.exec("INSERT INTO space_admins(space_id,user_id,granted_by) VALUES ($1,$2,$3)", a1, spaceAdminNoMember.id, admin.id)
	f.exec("INSERT INTO space_admins(space_id,user_id,granted_by) VALUES ($1,$2,$3)", a1, spaceAdminMember.id, admin.id)

	t.Run("SP-01 active member with a Full grant keeps Full", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, full, a1, policy.Edit); got != policy.Allowed {
			t.Fatalf("Full member edit = %v", got)
		}
		if got := decideOn(t, f, full, a1, policy.Share); got != policy.Allowed {
			t.Fatalf("Full member share = %v", got)
		}
	})

	t.Run("SP-02 active member without a resource grant has no access", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, noGrant, a1, policy.View); got != policy.NotFound {
			t.Fatalf("member without grant = %v", got)
		}
	})

	t.Run("SP-03 a Full grant without membership is denied", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, fullNoMember, a1, policy.View); got != policy.NotFound {
			t.Fatalf("grant without membership = %v", got)
		}
	})

	t.Run("SP-04 membership in A does not expose client B", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, memberFullOnB1, b1, policy.View); got != policy.NotFound {
			t.Fatalf("member of A with a grant on B1 = %v", got)
		}
	})

	t.Run("SP-05 membership in A and B still needs a separate grant per Space", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, memberAB, a1, policy.View); got != policy.Allowed {
			t.Fatalf("granted A1 = %v", got)
		}
		if got := decideOn(t, f, memberAB, a2, policy.View); got != policy.NotFound {
			t.Fatalf("ungranted A2 = %v", got)
		}
		if got := decideOn(t, f, memberAB, b1, policy.View); got != policy.NotFound {
			t.Fatalf("ungranted B1 = %v", got)
		}
	})

	t.Run("SP-14 Editor, Read Only and Limited match the same grant on an internal Space", func(t *testing.T) {
		f := base.on(t)
		// Control: an internal Space with the same role and no client. Eligibility must not change the capabilities.
		for _, tc := range []struct {
			name string
			p    *person
			role string
		}{{"editor", editor, "editor"}, {"read", read, "read"}, {"limited", limited, "limited"}} {
			control := f.person("regular")
			ctrlSpace := f.scopedSpace("control "+tc.name, "internal", "")
			f.share(ctrlSpace, "user", control.id, tc.role)
			for _, need := range policy.AllCapabilities {
				got := decideOn(t, f, tc.p, a1, need)
				want := decideOn(t, f, control, ctrlSpace, need)
				if got != want {
					t.Fatalf("%s %s: client %v, internal control %v", tc.name, need, got, want)
				}
			}
		}
	})

	t.Run("SP-08 Workspace Admin without membership or grant is denied", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, adminNoGrant, a1, policy.View); got != policy.NotFound {
			t.Fatalf("admin = %v", got)
		}
	})

	t.Run("SP-09 Workspace Admin with membership but no grant is denied", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, adminMember, a1, policy.View); got != policy.NotFound {
			t.Fatalf("admin member without grant = %v", got)
		}
	})

	t.Run("SP-10 Workspace Admin with a grant but no membership is denied", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, adminGrant, a1, policy.View); got != policy.NotFound {
			t.Fatalf("admin grant without membership = %v", got)
		}
	})

	t.Run("SP-11 explicit Space Admin without membership is denied", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, spaceAdminNoMember, a1, policy.View); got != policy.NotFound {
			t.Fatalf("space admin without membership = %v", got)
		}
	})

	t.Run("SP-12 explicit Space Admin with membership keeps Space Admin capabilities", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, spaceAdminMember, a1, policy.Share); got != policy.Allowed {
			t.Fatalf("space admin with membership share = %v", got)
		}
	})

	t.Run("SP-13 a group grant cannot bypass membership", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, groupMember, a1, policy.View); got != policy.NotFound {
			t.Fatalf("group grant without membership = %v", got)
		}
	})

	t.Run("SP-15 an inactive actor cannot reach a client-owned Space", func(t *testing.T) {
		f := base.on(t)
		f.want(f.call(admin, "POST", "/users/"+deactivated.id+"/deactivate", nil), http.StatusOK, "deactivate")
		if got := decideOn(t, f, deactivated, a1, policy.View); got != policy.NotFound {
			t.Fatalf("deactivated member = %v", got)
		}
	})

	t.Run("SP-16 an unresolved Space is denied even with a Full grant", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, full, unresolved, policy.View); got != policy.NotFound {
			t.Fatalf("unresolved with grant = %v", got)
		}
	})

	t.Run("SP-17 an internal Space keeps explicit access, without membership", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, internalFull, internal, policy.Edit); got != policy.Allowed {
			t.Fatalf("explicit internal grant = %v", got)
		}
		if got := decideOn(t, f, noGrant, internal, policy.View); got != policy.NotFound {
			t.Fatalf("internal Space without grant = %v", got)
		}
	})

	t.Run("SP-18 a Personal Space stays private to its owner", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, personalOwner, personal, policy.Edit); got != policy.Allowed {
			t.Fatalf("owner = %v", got)
		}
		if got := decideOn(t, f, adminNoGrant, personal, policy.View); got != policy.NotFound {
			t.Fatalf("admin on personal = %v", got)
		}
		if got := decideOn(t, f, full, personal, policy.View); got != policy.NotFound {
			t.Fatalf("other user on personal = %v", got)
		}
	})

	t.Run("SP-19 archived Customer keeps historical read for an authorized active member", func(t *testing.T) {
		f := base.on(t)
		f.want(f.call(admin, "POST", "/clients/"+clientC+"/archive", nil), http.StatusOK, "archive")
		if got := decideOn(t, f, memberC, c1, policy.View); got != policy.Allowed {
			t.Fatalf("archived historical read = %v", got)
		}
	})

	t.Run("SP-20 revoked member cannot reach an archived Customer Space", func(t *testing.T) {
		f := base.on(t)
		f.revokeMember(admin, clientC, memberC)
		if got := decideOn(t, f, memberC, c1, policy.View); got != policy.NotFound {
			t.Fatalf("revoked on archived = %v", got)
		}
	})

	t.Run("SP-21 the Space list omits ineligible Spaces", func(t *testing.T) {
		f := base.on(t)
		r := f.call(full, "GET", "/spaces", nil)
		f.want(r, http.StatusOK, "list")
		ids := listedIDs(t, r)
		if !containsID(ids, a1) {
			t.Fatalf("list lacks the granted Space A1")
		}
		for _, hidden := range []string{a2, b1, c1, internal, unresolved} {
			if containsID(ids, hidden) {
				t.Fatalf("list exposes %s", hidden)
			}
		}
	})

	t.Run("SP-22 SpaceAccessFor denies an ineligible Space", func(t *testing.T) {
		f := base.on(t)
		_, found, err := policy.SpaceAccessFor(f.ctx, f.pool, actorOf2(full), b1)
		if err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("SP-23 CanOnSpace denies every capability on an ineligible Space", func(t *testing.T) {
		f := base.on(t)
		for _, need := range policy.AllCapabilities {
			if got := decideOn(t, f, full, b1, need); got != policy.NotFound {
				t.Fatalf("%s on B1 = %v", need, got)
			}
		}
	})

	t.Run("SP-24 direct lookup follows the central denial", func(t *testing.T) {
		f := base.on(t)
		f.want(f.call(full, "GET", "/spaces/"+b1, nil), http.StatusNotFound, "direct lookup of B1")
		f.want(f.call(full, "GET", "/spaces/"+a1, nil), http.StatusOK, "direct lookup of A1")
	})

	t.Run("SP-25 list counts only what is visible", func(t *testing.T) {
		f := base.on(t)
		r := f.call(memberAB, "GET", "/spaces", nil)
		f.want(r, http.StatusOK, "list")
		ids := listedIDs(t, r)
		if containsID(ids, a2) || containsID(ids, b1) || !containsID(ids, a1) {
			t.Fatalf("count/visibility wrong: %v", ids)
		}
	})

	t.Run("SP-27 D-17.1: an administrator without a grant cannot open an internal Space", func(t *testing.T) {
		f := base.on(t)
		f.want(f.call(adminNoGrant, "GET", "/spaces/"+internal, nil), http.StatusNotFound, "admin on internal")
	})

	t.Run("SP-28 creation still needs the destination Space rights", func(t *testing.T) {
		f := base.on(t)
		f.want(f.call(full, "POST", "/items", itemBody("task", "sp28-ok", a1, "")), http.StatusCreated, "create in granted A1")
		f.want(f.call(full, "POST", "/items", itemBody("task", "sp28-denied", b1, "")), http.StatusNotFound, "create in ungranted B1")
	})

	// SP-06 and SP-07 change membership: revoked means the grant exists but eligibility does not; regrant restores it.
	t.Run("SP-06 revoked membership denies an existing explicit grant at once", func(t *testing.T) {
		f := base.on(t)
		if got := decideOn(t, f, revoked, a1, policy.View); got != policy.Allowed {
			t.Fatalf("before revoke = %v", got)
		}
		f.revokeMember(admin, clientA, revoked)
		f.want(f.call(revoked, "GET", "/spaces/"+a1, nil), http.StatusNotFound, "after revoke, immediately")
		if n := f.count("SELECT count(*) FROM shares WHERE target_type='space' AND target_id=$1 AND principal_id=$2", a1, revoked.id); n != 1 {
			t.Fatalf("revoke changed the resource grant: %d rows", n)
		}
	})

	t.Run("SP-07 regrant restores eligibility without touching the resource grant", func(t *testing.T) {
		f := base.on(t)
		f.addMember(admin, clientA, revoked)
		if got := decideOn(t, f, revoked, a1, policy.View); got != policy.Allowed {
			t.Fatalf("after regrant = %v", got)
		}
	})

	t.Run("SP-26 a membership revoke is observed by the next authorization", func(t *testing.T) {
		f := base.on(t)
		f.revokeMember(admin, clientA, revoked)
		if got := decideOn(t, f, revoked, a1, policy.Edit); got != policy.NotFound {
			t.Fatalf("next request after revoke = %v", got)
		}
	})
}

// listedIDs returns the id of every item in a list response.
func listedIDs(t *testing.T, r reply) []string {
	t.Helper()
	items, ok := r.body["items"].([]any)
	if !ok {
		t.Fatalf("no items in response: %v", r.body)
	}
	var ids []string
	for _, it := range items {
		ids = append(ids, it.(map[string]any)["id"].(string))
	}
	return ids
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// on returns the same fixture reporting to t. f.want and f.call fail t, so a subtest must use its own t, or a failure
// would stop the parent and skip the siblings.
func (f *fixture) on(t *testing.T) *fixture {
	g := *f
	g.t = t
	return &g
}
