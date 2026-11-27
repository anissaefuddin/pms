package httpserver_test

import (
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
)

// B3-07 (membership revocation, next authorization) and B3-08 (stable ownership for move, locations, reparent, restore,
// reorder), plus the B3-06 follow-up. Real sessions and database. Each subtest creates its own users and items, so a revocation
// in one subtest cannot disturb another. Negative fixtures are inconsistent on purpose; they do not authorize such data.

type mutationWorld struct {
	f       *fixture
	admin   *person
	clientA string
	clientB string
	a1      string // client A
	a2      string // client A
	b1      string // client B
	intl    string // internal
	unres   string // unresolved
}

func newMutationWorld(t *testing.T) mutationWorld {
	f := newFixture(t)
	admin := f.person("admin")
	w := mutationWorld{f: f, admin: admin}
	w.clientA = f.createClient(admin, "MUT-A")["id"].(string)
	w.clientB = f.createClient(admin, "MUT-B")["id"].(string)
	w.a1 = f.scopedSpace("A1", "client", w.clientA)
	w.a2 = f.scopedSpace("A2", "client", w.clientA)
	w.b1 = f.scopedSpace("B1", "client", w.clientB)
	w.intl = f.scopedSpace("I1", "internal", "")
	w.unres = f.unresolvedSpace("U1")
	return w
}

// member creates an active member of client A with a Full grant on A1 and A2 (created by the admin).
func (w mutationWorld) member(f *fixture, name string) *person {
	p := f.person("regular")
	f.addMember(w.admin, w.clientA, p)
	f.share(w.a1, "user", p.id, "full")
	f.share(w.a2, "user", p.id, "full")
	return p
}

func (w mutationWorld) locate(t *testing.T, item string) (string, string) {
	t.Helper()
	var scope string
	var client *string
	if err := w.f.pool.QueryRow(w.f.ctx, "SELECT scope_state, client_id::text FROM items WHERE id=$1", item).Scan(&scope, &client); err != nil {
		t.Fatal(err)
	}
	c := "-"
	if client != nil {
		c = *client
	}
	return scope, c
}

func (w mutationWorld) places(t *testing.T, item string) int {
	t.Helper()
	return w.f.count("SELECT count(*) FROM item_locations WHERE item_id=$1 AND deleted_root_id IS NULL", item)
}

// move replaces an item's locations through the real endpoint.
func move(f *fixture, p *person, item string, locations ...map[string]any) reply {
	return f.call(p, "PUT", "/items/"+item+"/locations", map[string]any{"locations": locations})
}

func loc(space string) map[string]any { return map[string]any{"space_id": space} }

// RV-01..RV-08 membership revocation.
func TestMembershipRevocationTakesEffectAtOnce(t *testing.T) {
	w := newMutationWorld(t)
	f := w.f

	t.Run("RV-01 the same session loses Space access after revoke", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "rv1")
		f.want(f.call(m, "GET", "/spaces/"+w.a1, nil), http.StatusOK, "before revoke")
		f.revokeMember(w.admin, w.clientA, m)
		f.want(f.call(m, "GET", "/spaces/"+w.a1, nil), http.StatusNotFound, "after revoke, same session")
	})

	t.Run("RV-02 the same session loses item access after revoke", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "rv2")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "rv2-item")
		f.want(f.call(m, "GET", "/items/"+item, nil), http.StatusOK, "before revoke")
		f.revokeMember(w.admin, w.clientA, m)
		f.want(f.call(m, "GET", "/items/"+item, nil), http.StatusNotFound, "after revoke, same session")
	})

	t.Run("RV-03 direct and group shares cannot bypass revocation", func(t *testing.T) {
		f := w.f.on(t)
		direct := f.person("regular")
		f.addMember(w.admin, w.clientA, direct)
		item := f.itemIn(w.a1, "client", w.clientA, direct.id, "rv3-item")
		f.itemShare(item, "user", direct.id, "full")
		group := f.person("regular")
		f.addMember(w.admin, w.clientA, group)
		g := f.group("rv3 group", group)
		f.itemShare(item, "group", g, "full")
		f.revokeMember(w.admin, w.clientA, direct)
		f.revokeMember(w.admin, w.clientA, group)
		if canView(t, f, direct, item) || canView(t, f, group, item) {
			t.Fatal("a direct or group share survived revocation")
		}
	})

	t.Run("RV-04 a Workspace Admin cannot bypass revocation", func(t *testing.T) {
		f := w.f.on(t)
		adm := f.person("admin")
		f.addMember(w.admin, w.clientA, adm)
		f.share(w.a1, "user", adm.id, "full")
		item := f.itemIn(w.a1, "client", w.clientA, adm.id, "rv4-item")
		f.want(f.call(adm, "GET", "/items/"+item, nil), http.StatusOK, "member admin before revoke")
		f.revokeMember(w.admin, w.clientA, adm)
		f.want(f.call(adm, "GET", "/items/"+item, nil), http.StatusNotFound, "admin after revoke")
	})

	t.Run("RV-05 regrant restores only what the existing grant permits", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "rv5")
		noGrant := f.person("regular")
		f.addMember(w.admin, w.clientA, noGrant)
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "rv5-item")
		f.revokeMember(w.admin, w.clientA, m)
		f.revokeMember(w.admin, w.clientA, noGrant)
		f.addMember(w.admin, w.clientA, m)
		f.addMember(w.admin, w.clientA, noGrant)
		f.want(f.call(m, "GET", "/items/"+item, nil), http.StatusOK, "regrant with grant")
		f.want(f.call(noGrant, "GET", "/items/"+item, nil), http.StatusNotFound, "regrant without grant")
	})

	t.Run("RV-06 shares and historical memberships and assignments stay intact", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "rv6")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "rv6-item")
		f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", item, m.id)
		shares := f.count("SELECT count(*) FROM shares WHERE principal_id=$1", m.id)
		f.revokeMember(w.admin, w.clientA, m)
		f.addMember(w.admin, w.clientA, m)
		if f.count("SELECT count(*) FROM shares WHERE principal_id=$1", m.id) != shares {
			t.Fatal("revocation changed a resource share")
		}
		if f.count("SELECT count(*) FROM item_assignees WHERE item_id=$1 AND user_id=$2", item, m.id) != 1 {
			t.Fatal("revocation deleted an assignment")
		}
		if f.count("SELECT count(*) FROM client_memberships WHERE client_id=$1 AND user_id=$2", w.clientA, m.id) != 2 {
			t.Fatal("membership history was not kept (expected the revoked row and the active row)")
		}
	})

	t.Run("RV-07 authorization after revoke reads the database, not a stale result", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "rv7")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "rv7-item")
		if !canView(t, f, m, item) {
			t.Fatal("member lost access before revoke")
		}
		f.revokeMember(w.admin, w.clientA, m)
		if canView(t, f, m, item) {
			t.Fatal("a result from before the revoke was still served")
		}
	})

	t.Run("RV-08 a new assignment of a revoked person is rejected", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "rv8")
		target := f.person("regular")
		f.addMember(w.admin, w.clientA, target)
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "rv8-item")
		f.revokeMember(w.admin, w.clientA, target)
		version := f.queryID("SELECT version::text FROM items WHERE id=$1", item)
		f.want(f.call(m, "PATCH", "/items/"+item, map[string]any{"assignee_ids": []string{target.id}}, "If-Match", `"`+version+`"`),
			http.StatusUnprocessableEntity, "revoked target")
	})
	_ = f
}

// LOC-01..LOC-18: stable ownership for move, additional location, reparent, restore and reorder.
func TestMutationsKeepStableOwnership(t *testing.T) {
	w := newMutationWorld(t)
	f := w.f
	admin := w.admin
	_ = admin

	t.Run("LOC-01 a same-client move succeeds", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc1")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "loc1")
		f.want(move(f, m, item, loc(w.a2)), http.StatusOK, "move A1 to A2")
		if f.count("SELECT count(*) FROM item_locations WHERE item_id=$1 AND space_id=$2 AND deleted_root_id IS NULL", item, w.a2) != 1 {
			t.Fatal("the move did not land")
		}
	})

	t.Run("LOC-02 a cross-client move is rejected by ownership", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc2")
		f.addMember(w.admin, w.clientB, m) // a member of both: the rejection is about ownership, not visibility
		f.share(w.b1, "user", m.id, "full")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "loc2")
		f.want(move(f, m, item, loc(w.b1)), http.StatusUnprocessableEntity, "A to B")
		if scope, c := w.locate(t, item); scope != "client" || c != w.clientA {
			t.Fatal("ownership changed")
		}
	})

	t.Run("LOC-03 client and internal moves are rejected in both directions", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc3")
		f.share(w.intl, "user", m.id, "full")
		clientItem := f.itemIn(w.a1, "client", w.clientA, m.id, "loc3-client")
		f.want(move(f, m, clientItem, loc(w.intl)), http.StatusUnprocessableEntity, "client to internal")
		internalItem := f.itemIn(w.intl, "internal", "", m.id, "loc3-internal")
		f.want(move(f, m, internalItem, loc(w.a1)), http.StatusUnprocessableEntity, "internal to client")
	})

	t.Run("LOC-04 a same-client additional location succeeds and LOC-18 keeps the multi-location item", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc4")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "loc4")
		f.want(move(f, m, item, loc(w.a1), loc(w.a2)), http.StatusOK, "add A2, keep A1")
		if w.places(t, item) != 2 {
			t.Fatalf("places = %d, want 2", w.places(t, item))
		}
	})

	t.Run("LOC-05 an incompatible additional location is rejected and nothing is written", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc5")
		f.share(w.intl, "user", m.id, "full")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "loc5")
		f.want(move(f, m, item, loc(w.a1), loc(w.intl)), http.StatusUnprocessableEntity, "add internal")
		if w.places(t, item) != 1 {
			t.Fatal("a refused location was written")
		}
	})

	t.Run("LOC-06 a client-owned item cannot enter a Personal Space", func(t *testing.T) {
		f := w.f.on(t)
		owner := w.member(f, "loc6")
		personal := f.personalSpace(owner)
		item := f.itemIn(w.a1, "client", w.clientA, owner.id, "loc6")
		f.want(move(f, owner, item, loc(personal)), http.StatusUnprocessableEntity, "personal")
	})

	t.Run("LOC-07 unresolved source or destination is refused", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc7")
		u := f.itemIn(w.a1, "unresolved", "", m.id, "loc7-src")
		f.want(move(f, m, u, loc(w.a2)), http.StatusNotFound, "unresolved source is not visible")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "loc7-dst")
		f.want(move(f, m, item, loc(w.unres)), http.StatusNotFound, "unresolved destination is not visible")
	})

	t.Run("LOC-08 a compatible reparent succeeds", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc8")
		parent := f.itemIn(w.a2, "client", w.clientA, m.id, "loc8-parent")
		child := f.itemIn(w.a1, "client", w.clientA, m.id, "loc8-child")
		f.want(move(f, m, child, map[string]any{"space_id": w.a2, "parent_item_id": parent}), http.StatusOK, "reparent")
	})

	t.Run("LOC-09 an incompatible descendant rejects the whole subtree move", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc9")
		root := f.itemIn(w.a1, "client", w.clientA, m.id, "loc9-root")
		child := f.itemIn(w.a1, "client", w.clientA, m.id, "loc9-child")
		f.exec("UPDATE item_locations SET parent_item_id=$1 WHERE item_id=$2", root, child)
		f.exec("UPDATE items SET scope_state='internal', client_id=NULL WHERE id=$1", child) // inconsistent descendant
		f.want(move(f, m, root, loc(w.a1), loc(w.a2)), http.StatusUnprocessableEntity, "subtree with incompatible descendant")
		if w.places(t, root) != 1 {
			t.Fatal("a refused subtree move left a partial location")
		}
	})

	t.Run("LOC-10 a compatible restore succeeds", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc10")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "loc10")
		f.want(f.call(m, "DELETE", "/items/"+item, nil), http.StatusNoContent, "delete")
		f.want(f.call(m, "POST", "/recycle-bin/"+item+"/restore", nil), http.StatusOK, "restore")
	})

	t.Run("LOC-11 restoring into an archived Customer is refused", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc11")
		arch := f.createClient(w.admin, "MUT-ARCH")["id"].(string)
		space := f.scopedSpace("Arch Space", "client", arch)
		f.addMember(w.admin, arch, m)
		f.share(space, "user", m.id, "full")
		item := f.itemIn(space, "client", arch, m.id, "loc11")
		f.want(f.call(m, "DELETE", "/items/"+item, nil), http.StatusNoContent, "delete")
		f.want(f.call(w.admin, "POST", "/clients/"+arch+"/archive", nil), http.StatusOK, "archive")
		f.want(f.call(m, "POST", "/recycle-bin/"+item+"/restore", nil), http.StatusConflict, "restore into archived customer")
	})

	t.Run("LOC-12 reorder keeps its rank-only scope and cannot target an incompatible sibling", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc12")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "loc12")
		sibling := f.itemIn(w.intl, "internal", "", m.id, "loc12-sibling")
		f.want(f.call(m, "POST", "/items/"+item+"/reorder", map[string]any{"space_id": w.a1, "after_item_id": sibling}), http.StatusNotFound, "incompatible sibling")
		if scope, c := w.locate(t, item); scope != "client" || c != w.clientA {
			t.Fatal("reorder changed ownership")
		}
	})

	t.Run("LOC-13 the existing destination permission still applies", func(t *testing.T) {
		f := w.f.on(t)
		editor := f.person("regular")
		f.addMember(w.admin, w.clientA, editor)
		f.share(w.a1, "user", editor.id, "editor")
		f.share(w.a2, "user", editor.id, "read")
		item := f.itemIn(w.a1, "client", w.clientA, editor.id, "loc13")
		f.want(move(f, editor, item, loc(w.a2)), http.StatusForbidden, "destination read only")
	})

	t.Run("LOC-14 a Workspace Admin without membership or grant cannot move a client item", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc14")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "loc14")
		f.want(move(f, w.admin, item, loc(w.a2)), http.StatusNotFound, "admin")
	})

	t.Run("LOC-15 successful mutations leave ownership unchanged", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc15")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "loc15")
		f.want(move(f, m, item, loc(w.a1), loc(w.a2)), http.StatusOK, "add")
		if scope, c := w.locate(t, item); scope != "client" || c != w.clientA {
			t.Fatal("ownership changed by a move")
		}
	})

	t.Run("LOC-16 a failed move leaves no partial location or history", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc16")
		f.share(w.intl, "user", m.id, "full")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "loc16")
		acts := f.count("SELECT count(*) FROM activity WHERE item_id=$1", item)
		f.want(move(f, m, item, loc(w.a2), loc(w.intl)), http.StatusUnprocessableEntity, "one valid, one incompatible")
		if w.places(t, item) != 1 || f.count("SELECT count(*) FROM activity WHERE item_id=$1", item) != acts {
			t.Fatal("a refused move left a partial location or history")
		}
	})

	t.Run("LOC-17 an archived Customer receives no new active location, but history stays readable", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "loc17")
		arch := f.createClient(w.admin, "MUT-ARCH17")["id"].(string)
		src := f.scopedSpace("Arch src", "client", arch)
		dst := f.scopedSpace("Arch dst", "client", arch)
		f.addMember(w.admin, arch, m)
		f.share(src, "user", m.id, "full")
		f.share(dst, "user", m.id, "full")
		item := f.itemIn(src, "client", arch, m.id, "loc17")
		f.want(f.call(w.admin, "POST", "/clients/"+arch+"/archive", nil), http.StatusOK, "archive")
		f.want(move(f, m, item, loc(dst)), http.StatusConflict, "new location under archived customer")
		f.want(f.call(m, "GET", "/items/"+item, nil), http.StatusOK, "historical read")
	})

	_ = f
}

// CON: lock ordering against archive, revocation and concurrent mutations. The waiting side is observed in the database.
func TestMutationsAreSerializedWithArchiveAndRevoke(t *testing.T) {
	w := newMutationWorld(t)

	t.Run("CON-1 an archive that holds the Customer row makes a waiting location change refuse", func(t *testing.T) {
		f := w.f.on(t)
		// Its own Customer: this subtest archives it, and the others must keep the shared one active.
		cl := f.createClient(w.admin, "CON-ARCH")["id"].(string)
		src := f.scopedSpace("con1 src", "client", cl)
		dst := f.scopedSpace("con1 dst", "client", cl)
		m := f.person("regular")
		f.addMember(w.admin, cl, m)
		f.share(src, "user", m.id, "full")
		f.share(dst, "user", m.id, "full")
		item := f.itemIn(src, "client", cl, m.id, "con1")
		tx, err := f.pool.BeginTx(f.ctx, pgx.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx)
		if _, err := tx.Exec(f.ctx, "UPDATE clients SET status='archived', archived_at=now(), archived_by=$2 WHERE id=$1", cl, w.admin.id); err != nil {
			t.Fatal(err)
		}
		done := make(chan reply, 1)
		go func() { done <- move(f, m, item, loc(dst)) }()
		waitForLockWait(t, f)
		if err := tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}
		if r := <-done; r.status != http.StatusConflict {
			t.Fatalf("location change after archive: status %d, want 409", r.status)
		}
	})

	t.Run("CON-2 a revoke that holds the membership makes a waiting location change refuse", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "con2")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "con2")
		tx, err := f.pool.BeginTx(f.ctx, pgx.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx)
		if _, err := tx.Exec(f.ctx, "UPDATE client_memberships SET status='revoked', revoked_at=now(), revoked_by=$3 WHERE client_id=$1 AND user_id=$2 AND status='active'", w.clientA, m.id, w.admin.id); err != nil {
			t.Fatal(err)
		}
		done := make(chan reply, 1)
		go func() { done <- move(f, m, item, loc(w.a2)) }()
		waitForLockWait(t, f)
		if err := tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}
		if r := <-done; r.status != http.StatusNotFound {
			t.Fatalf("location change after revoke: status %d, want 404", r.status)
		}
	})

	t.Run("CON-3 two location changes on one item are serialized and both see consistent ownership", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "con3")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "con3")
		tx, err := f.pool.BeginTx(f.ctx, pgx.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx)
		if _, err := tx.Exec(f.ctx, "SELECT 1 FROM items WHERE id=$1 FOR UPDATE", item); err != nil {
			t.Fatal(err)
		}
		done := make(chan reply, 1)
		go func() { done <- move(f, m, item, loc(w.a1), loc(w.a2)) }()
		waitForLockWait(t, f)
		if err := tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}
		if r := <-done; r.status != http.StatusOK {
			t.Fatalf("serialized change: status %d", r.status)
		}
		if scope, c := w.locate(t, item); scope != "client" || c != w.clientA {
			t.Fatal("ownership changed under concurrent change")
		}
	})

}

// AF: the B3-06 follow-up. Full on a valid place with an unresolved or inconsistent place fails closed for assign-other.
func TestAssignmentFollowUpFailsClosed(t *testing.T) {
	w := newMutationWorld(t)

	t.Run("AF-1 Full on a valid place plus an unresolved place refuses assigning another", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "af1")
		target := f.person("regular")
		f.addMember(w.admin, w.clientA, target)
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "af1")
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'z')", item, w.unres)
		version := f.queryID("SELECT version::text FROM items WHERE id=$1", item)
		f.want(f.call(m, "PATCH", "/items/"+item, map[string]any{"assignee_ids": []string{target.id}}, "If-Match", `"`+version+`"`),
			http.StatusForbidden, "unresolved place")
	})

	t.Run("AF-2 Full on a valid place alone still assigns another (ordinary authority is unchanged)", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "af2")
		target := f.person("regular")
		f.addMember(w.admin, w.clientA, target)
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "af2")
		version := f.queryID("SELECT version::text FROM items WHERE id=$1", item)
		f.want(f.call(m, "PATCH", "/items/"+item, map[string]any{"assignee_ids": []string{target.id}}, "If-Match", `"`+version+`"`),
			http.StatusOK, "valid assignment")
	})

	t.Run("AF-3 automatic assignment: an eligible performer is recorded in the same transaction", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "af3")
		f.exec("UPDATE statuses SET auto_assign=true WHERE name='In Progress'")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "af3")
		status := f.queryID("SELECT id::text FROM statuses WHERE name='In Progress'")
		version := f.queryID("SELECT version::text FROM items WHERE id=$1", item)
		f.want(f.call(m, "PATCH", "/items/"+item, map[string]any{"status_id": status}, "If-Match", `"`+version+`"`), http.StatusOK, "status change")
		if f.count("SELECT count(*) FROM item_assignees WHERE item_id=$1 AND user_id=$2", item, m.id) != 1 {
			t.Fatal("the performer was not recorded as an assignee")
		}
	})

	t.Run("AF-4 a refused status change with automatic assignment leaves no partial change", func(t *testing.T) {
		f := w.f.on(t)
		m := w.member(f, "af4")
		outsider := f.person("regular")
		f.share(w.a1, "user", outsider.id, "full") // Full on the place, but not a member: the item is not visible to them
		f.exec("UPDATE statuses SET auto_assign=true WHERE name='In Progress'")
		item := f.itemIn(w.a1, "client", w.clientA, m.id, "af4")
		status := f.queryID("SELECT id::text FROM statuses WHERE name='In Progress'")
		before := f.queryID("SELECT COALESCE(status_id::text,'') FROM items WHERE id=$1", item)
		version := f.queryID("SELECT version::text FROM items WHERE id=$1", item)
		f.want(f.call(outsider, "PATCH", "/items/"+item, map[string]any{"status_id": status}, "If-Match", `"`+version+`"`), http.StatusNotFound, "non-member")
		if f.queryID("SELECT COALESCE(status_id::text,'') FROM items WHERE id=$1", item) != before || f.count("SELECT count(*) FROM item_assignees WHERE item_id=$1", item) != 0 {
			t.Fatal("a refused status change left a partial change")
		}
	})
}
