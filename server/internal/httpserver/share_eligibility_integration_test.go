package httpserver_test

import (
	"net/http"
	"testing"
)

// B3-11: a direct share on a client-owned Space or item needs an active member of its Customer. Membership is eligibility,
// never a grant. Removing a share is never refused, and an existing share does not give access once its holder is revoked.
func TestDirectSharesRequireCustomerEligibility(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	client := f.createClient(admin, "SH-A")["id"].(string)
	member, second := f.person("regular"), f.person("regular")
	outsider := f.person("regular")
	revoked := f.person("regular")
	f.addMember(admin, client, admin) // Workspace Admin still needs membership to act on client work (D-17.1)
	f.addMember(admin, client, member)
	f.addMember(admin, client, second)
	f.addMember(admin, client, revoked)
	space := f.scopedSpace("SH client", "client", client)
	f.share(space, "user", admin.id, "full")
	f.share(space, "user", member.id, "read")
	item := f.itemIn(space, "client", client, admin.id, "Shared item")
	entry := func(p *person, role string) map[string]any {
		return map[string]any{"principal_type": "user", "principal_id": p.id, "role": role}
	}
	putSpace := func(shares ...map[string]any) reply {
		etag := f.call(admin, "GET", "/spaces/"+space, nil).header.Get("ETag")
		return f.call(admin, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": shares}, "If-Match", etag)
	}

	t.Run("SH-01 a share to an active member is accepted", func(t *testing.T) {
		f := f.on(t)
		f.want(putSpace(entry(admin, "full"), entry(member, "read"), entry(second, "read")), http.StatusOK, "member recipient")
	})
	t.Run("SH-02 a share to a non-member is refused and writes nothing", func(t *testing.T) {
		f := f.on(t)
		f.want(putSpace(entry(admin, "full"), entry(member, "read"), entry(outsider, "read")), http.StatusUnprocessableEntity, "non-member recipient")
		if n := f.queryID("SELECT count(*)::text FROM shares WHERE target_id=$1 AND principal_id=$2", space, outsider.id); n != "0" {
			t.Fatalf("a refused share was stored: %s", n)
		}
	})
	t.Run("SH-03 a share to a revoked member is refused", func(t *testing.T) {
		f := f.on(t)
		f.revokeMember(admin, client, revoked)
		f.want(putSpace(entry(admin, "full"), entry(member, "read"), entry(revoked, "read")), http.StatusUnprocessableEntity, "revoked recipient")
	})
	t.Run("SH-04 removing a share is always allowed and gives no access once revoked", func(t *testing.T) {
		f := f.on(t)
		f.want(putSpace(entry(admin, "full")), http.StatusOK, "remove member share")
		f.want(f.call(member, "GET", "/spaces/"+space, nil), http.StatusNotFound, "removed member has no access")
	})
	t.Run("SH-05 an item share to a non-member is refused", func(t *testing.T) {
		f := f.on(t)
		res := f.call(admin, "PUT", "/items/"+item+"/shares", map[string]any{"shares": []map[string]any{entry(outsider, "read")}}, "If-Match", `"1"`)
		f.want(res, http.StatusUnprocessableEntity, "item share to non-member")
		if n := f.queryID("SELECT count(*)::text FROM shares WHERE target_id=$1 AND principal_id=$2", item, outsider.id); n != "0" {
			t.Fatalf("a refused item share was stored: %s", n)
		}
	})
	t.Run("SH-06 creating an item with a share to a non-member is refused", func(t *testing.T) {
		f := f.on(t)
		// Positive control first: the same body with an eligible member is accepted. Tasks take no shares, so a folder is used.
		f.want(f.call(admin, "POST", "/items", map[string]any{
			"type": "folder", "title": "Shared with member", "locations": []map[string]any{{"space_id": space}},
			"shares": []map[string]any{entry(member, "read")},
		}), http.StatusCreated, "create with eligible share")
		res := f.call(admin, "POST", "/items", map[string]any{
			"type": "folder", "title": "Shared on create", "locations": []map[string]any{{"space_id": space}},
			"shares": []map[string]any{entry(outsider, "read")},
		})
		f.want(res, http.StatusUnprocessableEntity, "create with ineligible share")
		if n := f.queryID("SELECT count(*)::text FROM items WHERE title='Shared on create'"); n != "0" {
			t.Fatalf("a refused create left an item: %s", n)
		}
	})
	t.Run("SH-07 an internal Space accepts a share to any active user", func(t *testing.T) {
		f := f.on(t)
		internal := f.internalSpace("SH internal")
		f.want(f.call(admin, "PUT", "/spaces/"+internal+"/shares", map[string]any{"shares": []map[string]any{
			entry(admin, "full"), entry(outsider, "read"),
		}}, "If-Match", f.call(admin, "GET", "/spaces/"+internal+"/shares", nil).header.Get("ETag")), http.StatusOK, "internal share")
	})
	t.Run("SH-08 a group with a non-member gives that member no access to the client Space", func(t *testing.T) {
		f := f.on(t)
		group := f.group("SH mixed group", member, outsider)
		f.share(space, "group", group, "read")
		f.want(f.call(member, "GET", "/spaces/"+space, nil), http.StatusOK, "member through the group")
		f.want(f.call(outsider, "GET", "/spaces/"+space, nil), http.StatusNotFound, "non-member through the group")
	})
}

// B3-11: an invitation or a public join cannot hand a new person access to a client-owned Space. The positive controls use an
// internal Space, so a refusal is not an artefact of the request shape.
func TestSelfServiceAccessRespectsCustomerScope(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	client := f.createClient(admin, "SS-A")["id"].(string)
	f.addMember(admin, client, admin)
	clientSpace := f.scopedSpace("SS client", "client", client)
	internal := f.internalSpace("SS internal")
	f.exec("UPDATE spaces SET visibility='public' WHERE id=ANY($1::uuid[])", []string{clientSpace, internal})
	f.share(clientSpace, "user", admin.id, "full")
	f.share(internal, "user", admin.id, "full")
	outsider := f.person("regular")

	t.Run("SS-01 an invitation cannot share a client-owned Space", func(t *testing.T) {
		f := f.on(t)
		res := f.call(admin, "POST", "/users/invitations", map[string]any{
			"email": "ss01@example.com", "user_type": "regular",
			"share": []map[string]any{{"target_type": "space", "target_id": clientSpace, "role": "read"}},
		})
		f.want(res, http.StatusUnprocessableEntity, "invitation to client-owned Space")
		if n := f.queryID("SELECT count(*)::text FROM users WHERE email='ss01@example.com'"); n != "0" {
			t.Fatalf("a refused invitation created a user: %s", n)
		}
	})
	t.Run("SS-02 an invitation to an internal Space is accepted", func(t *testing.T) {
		f := f.on(t)
		f.want(f.call(admin, "POST", "/users/invitations", map[string]any{
			"email": "ss02@example.com", "user_type": "regular",
			"share": []map[string]any{{"target_type": "space", "target_id": internal, "role": "read"}},
		}), http.StatusCreated, "invitation to internal Space")
	})
	t.Run("SS-03 a public join gives no access to a client-owned Space", func(t *testing.T) {
		f := f.on(t)
		f.call(outsider, "POST", "/spaces/"+clientSpace+"/join", nil)
		f.want(f.call(outsider, "GET", "/spaces/"+clientSpace, nil), http.StatusNotFound, "joined client Space stays hidden")
	})
	t.Run("SS-04 a public join to an internal Space works", func(t *testing.T) {
		f := f.on(t)
		f.want(f.call(outsider, "POST", "/spaces/"+internal+"/join", nil), http.StatusOK, "join internal Space")
		f.want(f.call(outsider, "GET", "/spaces/"+internal, nil), http.StatusOK, "joined internal Space is visible")
	})
}
