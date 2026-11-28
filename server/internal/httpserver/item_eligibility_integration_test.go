package httpserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/clients"
	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

// B3-05 (item eligibility before permission union) and B3-06 (assignment on every path, assignee eligibility).
// Real sessions and database. Items are inserted with explicit ownership; the negative cases deliberately use inconsistent
// locations, which the policy must refuse. These fixtures do not authorize creating such data in production.

// itemIn inserts an item with explicit ownership and one live location.
func (f *fixture) itemIn(space, scope, client, creator string, title string) string {
	f.t.Helper()
	var c any
	if client != "" {
		c = client
	}
	id := f.queryID("INSERT INTO items(type,title,created_by,scope_state,client_id) VALUES ('task',$1,$2,$3,$4::uuid) RETURNING id::text", title, creator, scope, c)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'a')", id, space)
	return id
}

func (f *fixture) addLocation(item, space string) {
	f.t.Helper()
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'b')", item, space)
}

func (f *fixture) itemShare(item, principalType, principal, role string) {
	f.t.Helper()
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,$2::principal_type,$3,$4::share_role)", item, principalType, principal, role)
}

func policyItemCaps(t *testing.T, f *fixture, p *person, item string) policy.Caps {
	t.Helper()
	caps, err := policy.ItemCaps(f.ctx, f.pool, actorOf2(p), item)
	if err != nil {
		t.Fatalf("ItemCaps: %v", err)
	}
	return caps
}

func canView(t *testing.T, f *fixture, p *person, item string) bool {
	return policyItemCaps(t, f, p, item).Has(policy.View)
}

func canEdit(t *testing.T, f *fixture, p *person, item string) bool {
	return policyItemCaps(t, f, p, item).Has(policy.Edit)
}

// IT-01..IT-15: item eligibility, shares, groups, parents, admins, Personal, unresolved, revocation, archive.
func TestItemEligibility(t *testing.T) {
	base := newFixture(t)
	f := base
	admin := f.person("admin")
	clientA := f.createClient(admin, "IT-A")["id"].(string)
	clientB := f.createClient(admin, "IT-B")["id"].(string)
	a1 := f.scopedSpace("A1", "client", clientA)
	b1 := f.scopedSpace("B1", "client", clientB)
	internal := f.scopedSpace("I1", "internal", "")
	unresolved := f.unresolvedSpace("U1")
	creator := f.person("regular")
	member := f.person("regular")
	noGrant := f.person("regular")
	noMember := f.person("regular")
	memberB := f.person("regular")
	revoked := f.person("regular")
	groupMember := f.person("regular")
	directUser := f.person("regular")
	adminNoMember := f.person("admin")
	adminMember := f.person("admin")
	internalUser := f.person("regular")
	owner := f.person("regular")
	personal := f.personalSpace(owner)
	f.addMember(admin, clientA, creator)
	f.addMember(admin, clientA, member)
	f.addMember(admin, clientA, noGrant)
	f.addMember(admin, clientA, directUser)
	f.addMember(admin, clientA, revoked)
	f.addMember(admin, clientA, adminMember)
	f.addMember(admin, clientA, owner)
	f.addMember(admin, clientB, memberB)
	f.share(a1, "user", creator.id, "full")
	f.share(a1, "user", member.id, "full")
	f.share(a1, "user", noMember.id, "full")
	f.share(a1, "user", memberB.id, "full")
	f.share(a1, "user", revoked.id, "full")
	f.share(a1, "user", adminNoMember.id, "full")
	f.share(a1, "user", owner.id, "full")
	f.share(internal, "user", internalUser.id, "full")
	group := f.group("IT group", groupMember)
	f.share(a1, "group", group, "full")
	f.share(b1, "user", memberB.id, "full")

	item := f.itemIn(a1, "client", clientA, creator.id, "it-main")
	inRevoked := f.itemIn(a1, "client", clientA, creator.id, "it-revoked")
	f.revokeMember(admin, clientA, revoked)

	t.Run("IT-01 active member with a valid grant keeps access", func(t *testing.T) {
		f := base.on(t)
		if !canView(t, f, member, item) || !canEdit(t, f, member, item) {
			t.Fatal("eligible member lost access")
		}
	})
	t.Run("IT-02 membership without a resource grant has no access", func(t *testing.T) {
		f := base.on(t)
		if canView(t, f, noGrant, item) {
			t.Fatal("membership alone granted access")
		}
	})
	t.Run("IT-03 a Full grant without membership is denied", func(t *testing.T) {
		f := base.on(t)
		if canView(t, f, noMember, item) {
			t.Fatal("grant without membership granted access")
		}
	})
	t.Run("IT-04 membership in Client B does not reach a Client A item", func(t *testing.T) {
		f := base.on(t)
		if canView(t, f, memberB, item) {
			t.Fatal("Client B member reached Client A")
		}
	})
	t.Run("IT-05 a direct item share cannot bypass membership", func(t *testing.T) {
		f := base.on(t)
		f.itemShare(item, "user", noMember.id, "full")
		if canView(t, f, noMember, item) {
			t.Fatal("direct item share bypassed membership")
		}
	})
	t.Run("IT-06 a direct item share for an eligible member remains valid", func(t *testing.T) {
		f := base.on(t)
		f.itemShare(item, "user", directUser.id, "full")
		if !canView(t, f, directUser, item) {
			t.Fatal("eligible direct share was lost")
		}
	})
	t.Run("IT-07 a group grant cannot bypass membership", func(t *testing.T) {
		f := base.on(t)
		if canView(t, f, groupMember, item) {
			t.Fatal("group grant bypassed membership")
		}
	})
	t.Run("IT-08 parent inheritance cannot bypass membership", func(t *testing.T) {
		f := base.on(t)
		parent := f.itemIn(a1, "client", clientA, creator.id, "it-parent")
		child := f.itemIn(a1, "client", clientA, creator.id, "it-child")
		f.exec("UPDATE item_locations SET parent_item_id=$1 WHERE item_id=$2", parent, child)
		f.itemShare(parent, "user", noMember.id, "full")
		if canView(t, f, noMember, child) {
			t.Fatal("inherited share bypassed membership")
		}
		f.itemShare(parent, "user", noGrant.id, "read")
		if !canView(t, f, noGrant, child) {
			t.Fatal("eligible inherited share was lost")
		}
	})
	t.Run("IT-09 a Workspace Admin without membership is denied", func(t *testing.T) {
		f := base.on(t)
		if canView(t, f, adminNoMember, item) {
			t.Fatal("admin without membership reached client content")
		}
	})
	t.Run("IT-10 a Workspace Admin with membership but no grant is denied", func(t *testing.T) {
		f := base.on(t)
		if canView(t, f, adminMember, item) {
			t.Fatal("admin member reached the item without a grant")
		}
	})
	t.Run("IT-11 an internal item keeps its explicit-access permissions", func(t *testing.T) {
		f := base.on(t)
		ii := f.itemIn(internal, "internal", "", creator.id, "it-internal")
		if !canEdit(t, f, internalUser, ii) {
			t.Fatal("explicit internal grant lost")
		}
		if canView(t, f, noGrant, ii) {
			t.Fatal("internal item visible without a grant")
		}
	})
	t.Run("IT-12 Personal Space privacy is unchanged", func(t *testing.T) {
		f := base.on(t)
		pi := f.itemIn(personal, "internal", "", owner.id, "it-personal")
		if !canView(t, f, owner, pi) {
			t.Fatal("owner lost a personal item")
		}
		if canView(t, f, admin, pi) || canView(t, f, member, pi) {
			t.Fatal("personal item visible to someone else")
		}
	})
	t.Run("IT-13 an unresolved item is denied", func(t *testing.T) {
		f := base.on(t)
		u := f.itemIn(a1, "unresolved", "", creator.id, "it-unresolved")
		if canView(t, f, creator, u) {
			t.Fatal("unresolved item reached")
		}
		uu := f.itemIn(unresolved, "internal", "", creator.id, "it-in-unresolved")
		if canView(t, f, creator, uu) {
			t.Fatal("item located only in an unresolved Space reached")
		}
	})
	t.Run("IT-14 revocation denies the next item authorization at once", func(t *testing.T) {
		f := base.on(t)
		if canView(t, f, revoked, inRevoked) {
			t.Fatal("revoked member still reaches the item")
		}
		f.want(f.call(revoked, "GET", "/items/"+inRevoked, nil), http.StatusNotFound, "HTTP after revoke")
	})
	t.Run("IT-15 an archived Customer keeps historical access for an authorized active member", func(t *testing.T) {
		f := base.on(t)
		f.want(f.call(admin, "POST", "/clients/"+clientA+"/archive", nil), http.StatusOK, "archive")
		if !canView(t, f, member, item) {
			t.Fatal("archived historical read lost for an authorized member")
		}
	})
}

// ML-01..ML-08: multi-location union, with inconsistent locations that must contribute nothing.
func TestItemMultiLocationEligibility(t *testing.T) {
	base := newFixture(t)
	f := base
	admin := f.person("admin")
	clientA := f.createClient(admin, "ML-A")["id"].(string)
	clientB := f.createClient(admin, "ML-B")["id"].(string)
	a1 := f.scopedSpace("A1", "client", clientA)
	a2 := f.scopedSpace("A2", "client", clientA)
	b1 := f.scopedSpace("B1", "client", clientB)
	internal := f.scopedSpace("I1", "internal", "")
	unresolved := f.unresolvedSpace("U1")
	personalOwner := f.person("regular")
	personal := f.personalSpace(personalOwner)
	both := f.person("regular")
	editorA := f.person("regular")
	creator := f.person("regular")
	outsider := f.person("regular")
	f.addMember(admin, clientA, both)
	f.addMember(admin, clientB, both)
	f.addMember(admin, clientA, editorA)
	f.addMember(admin, clientA, personalOwner)
	f.addMember(admin, clientA, creator)
	f.share(a1, "user", both.id, "editor")
	f.share(b1, "user", both.id, "full")
	f.share(internal, "user", both.id, "full")
	f.share(unresolved, "user", both.id, "full")
	f.share(a1, "user", editorA.id, "editor")
	f.share(b1, "user", editorA.id, "full")
	f.share(a1, "user", creator.id, "full")
	f.share(a2, "user", creator.id, "full")
	f.share(b1, "user", outsider.id, "full")

	t.Run("ML-01 valid same-client locations keep the existing union", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, creator.id, "ml-same")
		f.addLocation(it, a2)
		if !canEdit(t, f, creator, it) {
			t.Fatal("same-client union lost")
		}
	})
	t.Run("ML-02 a different-client location cannot elevate permissions", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, creator.id, "ml-other")
		f.addLocation(it, b1)
		if policyItemCaps(t, f, editorA, it).Has(policy.Share) {
			t.Fatal("a Client B location gave Share")
		}
	})
	t.Run("ML-03 membership in both Customers cannot legitimize a mismatched location", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, creator.id, "ml-both")
		f.addLocation(it, b1)
		if policyItemCaps(t, f, both, it).Has(policy.Share) {
			t.Fatal("membership in both Customers legitimized a mismatched location")
		}
	})
	t.Run("ML-04 an internal location cannot elevate a client-owned item", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, creator.id, "ml-internal")
		f.addLocation(it, internal)
		if policyItemCaps(t, f, both, it).Has(policy.Share) {
			t.Fatal("an internal location elevated a client item")
		}
	})
	t.Run("ML-05 an unresolved location contributes nothing", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, creator.id, "ml-unresolved")
		f.addLocation(it, unresolved)
		if policyItemCaps(t, f, both, it).Has(policy.Share) {
			t.Fatal("an unresolved location contributed")
		}
	})
	t.Run("ML-06 a client-owned item gains nothing through a Personal Space", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(personal, "client", clientA, personalOwner.id, "ml-personal")
		if canView(t, f, personalOwner, it) {
			t.Fatal("a client item in a Personal Space was reachable")
		}
	})
	t.Run("ML-07 no valid authorization path denies access", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(b1, "client", clientA, creator.id, "ml-none")
		if canView(t, f, outsider, it) {
			t.Fatal("no valid path, yet access")
		}
	})
	t.Run("ML-08 a denied location's metadata is not exposed by the item response", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, creator.id, "ml-metadata")
		f.addLocation(it, b1)
		r := f.call(editorA, "GET", "/items/"+it, nil)
		if r.status != http.StatusOK {
			t.Fatalf("visible item status %d", r.status)
		}
		var body strings.Builder
		fmt.Fprint(&body, r.body)
		for _, leak := range []string{b1, "B1"} {
			if strings.Contains(body.String(), leak) {
				t.Fatalf("response discloses the denied location %q", leak)
			}
		}
	})
}

// AS-01..AS-12: assignment authority (DEC-10, Full on every path) and assignee eligibility.
func TestAssignmentEnforcement(t *testing.T) {
	base := newFixture(t)
	f := base
	admin := f.person("admin")
	clientA := f.createClient(admin, "AS-A")["id"].(string)
	a1 := f.scopedSpace("A1", "client", clientA)
	a2 := f.scopedSpace("A2", "client", clientA)
	a3 := f.scopedSpace("A3", "client", clientA)
	unresolved := f.unresolvedSpace("U1")
	full := f.person("regular")
	editor := f.person("regular")
	read := f.person("regular")
	limitedPath := f.person("regular")
	target := f.person("regular")
	outsider := f.person("regular")
	revokedTarget := f.person("regular")
	stale := f.person("regular")
	for _, p := range []*person{full, editor, read, limitedPath, target, stale} {
		f.addMember(admin, clientA, p)
	}
	f.addMember(admin, clientA, revokedTarget)
	f.revokeMember(admin, clientA, revokedTarget)
	f.share(a1, "user", full.id, "full")
	f.share(a1, "user", editor.id, "editor")
	f.share(a1, "user", read.id, "read")
	f.share(a1, "user", limitedPath.id, "limited")
	f.share(a2, "user", full.id, "editor") // AS-02: the second place is an Editor place for this actor
	f.share(a3, "user", full.id, "read")   // AS-03: the second place is a Read Only place for this actor
	f.share(a2, "user", limitedPath.id, "full")

	assign := func(f *fixture, p *person, item string, ids []string, want int, what string) reply {
		f.t.Helper()
		version := f.queryID("SELECT version::text FROM items WHERE id=$1", item)
		r := f.call(p, "PATCH", "/items/"+item, map[string]any{"assignee_ids": ids}, "If-Match", `"`+version+`"`)
		f.want(r, want, what)
		return r
	}

	t.Run("AS-01 Full on every path permits assigning another eligible member", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, full.id, "as-full")
		assign(f, full, it, []string{target.id}, http.StatusOK, "Full assigns")
	})
	t.Run("AS-02 Full plus Editor on another place refuses assigning others", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, full.id, "as-full-editor")
		f.addLocation(it, a2)
		assign(f, full, it, []string{target.id}, http.StatusForbidden, "Editor place refuses")
	})
	t.Run("AS-03 Full plus Read Only on another place refuses assigning others", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, full.id, "as-full-read")
		f.addLocation(it, a3)
		assign(f, full, it, []string{target.id}, http.StatusForbidden, "Read Only refuses")
	})
	t.Run("AS-04 a Limited place cannot be discarded to elevate assignment", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, full.id, "as-limited")
		f.addLocation(it, a2)
		assign(f, limitedPath, it, []string{target.id}, http.StatusForbidden, "Limited place refuses")
	})
	t.Run("AS-05 an item with no valid path fails closed", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(unresolved, "internal", "", full.id, "as-unresolved")
		assign(f, full, it, []string{target.id}, http.StatusNotFound, "unresolved path is not visible, so it is not found (fail closed)")
	})
	t.Run("AS-06 a direct or group share cannot bypass DEC-10", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, full.id, "as-direct")
		f.itemShare(it, "user", editor.id, "editor")
		assign(f, editor, it, []string{target.id}, http.StatusForbidden, "editor direct share")
	})
	t.Run("AS-07 an Editor may still assign themself", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, full.id, "as-self")
		f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,'user',$2,'editor')", it, editor.id)
		assign(f, editor, it, []string{editor.id}, http.StatusOK, "self-assignment")
	})
	t.Run("AS-08 an assignee without membership is rejected", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, full.id, "as-nonmember")
		assign(f, full, it, []string{outsider.id}, http.StatusUnprocessableEntity, "non-member target")
		if f.count("SELECT count(*) FROM item_assignees WHERE item_id=$1 AND user_id=$2", it, outsider.id) != 0 {
			t.Fatal("a refused assignee was written")
		}
	})
	t.Run("AS-09 a revoked target is rejected", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, full.id, "as-revoked")
		assign(f, full, it, []string{revokedTarget.id}, http.StatusUnprocessableEntity, "revoked target")
	})
	t.Run("AS-10 a valid assignment creates no membership or resource grant", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, full.id, "as-noop")
		m := f.count("SELECT count(*) FROM client_memberships")
		s := f.count("SELECT count(*) FROM shares")
		assign(f, full, it, []string{target.id}, http.StatusOK, "assign")
		if f.count("SELECT count(*) FROM client_memberships") != m || f.count("SELECT count(*) FROM shares") != s {
			t.Fatal("assignment created a membership or a grant")
		}
	})
	t.Run("AS-11 a stale assignee can still be removed", func(t *testing.T) {
		f := base.on(t)
		it := f.itemIn(a1, "client", clientA, full.id, "as-stale")
		f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", it, stale.id)
		f.revokeMember(admin, clientA, stale)
		assign(f, full, it, []string{}, http.StatusOK, "remove stale assignee")
	})
	t.Run("AS-12 create with an ineligible assignee is refused and writes nothing", func(t *testing.T) {
		f := base.on(t)
		before := f.count("SELECT count(*) FROM items")
		r := f.call(full, "POST", "/items", map[string]any{"type": "task", "title": "as12", "assignee_ids": []string{outsider.id},
			"locations": []map[string]any{{"space_id": a1}}})
		f.want(r, http.StatusUnprocessableEntity, "create with ineligible assignee")
		if f.count("SELECT count(*) FROM items") != before {
			t.Fatal("a refused create wrote an item")
		}
	})
}

// AS-13: the assignee check (inside creation) holds the target's membership FOR SHARE, so a revoke waits for the creation to commit.
func TestAssigneeEligibilityIsSerializedWithRevoke(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	clientA := f.createClient(admin, "AS13")["id"].(string)
	a1 := f.scopedSpace("A1", "client", clientA)
	creator := f.person("regular")
	target := f.person("regular")
	f.addMember(admin, clientA, creator)
	f.addMember(admin, clientA, target)
	f.share(a1, "user", creator.id, "full")

	tx, err := f.pool.BeginTx(f.ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	actor := policy.Actor{ID: creator.id, Type: policy.Regular}
	fields := map[string]json.RawMessage{"type": json.RawMessage(`"task"`), "title": json.RawMessage(`"as13"`),
		"assignee_ids": json.RawMessage(fmt.Sprintf(`[%q]`, target.id)),
		"locations":    json.RawMessage(fmt.Sprintf(`[{"space_id":%q}]`, a1))}
	if _, err := items.CreateTx(f.ctx, tx, actor, fields, &notifications.Batch{}); err != nil {
		t.Fatalf("create with an eligible assignee: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := clients.New(f.pool).Revoke(f.ctx, admin.id, clientA, target.id, func(context.Context, pgx.Tx, clients.Event) error { return nil })
		done <- err
	}()
	waitForLockWait(t, f)
	if err := tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("revoke after the creation: %v", err)
	}
	if got := f.count("SELECT count(*) FROM items i JOIN item_assignees a ON a.item_id=i.id WHERE i.title='as13' AND a.user_id=$1", target.id); got != 1 {
		t.Fatalf("the assignment that committed first is missing (%d)", got)
	}
}
