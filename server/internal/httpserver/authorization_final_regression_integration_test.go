package httpserver_test

import (
	"net/http"
	"testing"
)

// Final authorization regression for D-17 (Explicit Access Only). The workspace admin has no implicit access to a
// shared Space; access comes from shares, groups and explicit Space Admin, bounded by the user type ceiling.
// The name matches the CI job authorization-matrix (-run 'Authorization|AccessRules|Matrix|Capabilit').
func TestAuthorizationFinalRegressionExplicitAccessOnly(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	owner := f.person("regular")
	other := f.person("regular")

	t.Run("workspace admin without a share cannot read a shared Space or its items", func(t *testing.T) {
		space := f.spaceWithoutAdmins("Closed")
		f.share(space, "user", owner.id, "full")
		item := customFieldItem(f, owner, space, "Private doc")
		f.want(f.call(admin, "GET", "/spaces/"+space, nil), http.StatusNotFound, "admin without share: space hidden")
		f.want(f.call(admin, "GET", "/items/"+item, nil), http.StatusNotFound, "admin without share: item hidden")
	})

	t.Run("workspace admin with an explicit read share gets exactly the effective rights", func(t *testing.T) {
		space := f.spaceWithoutAdmins("Read for admin")
		f.share(space, "user", owner.id, "full")
		item := customFieldItem(f, owner, space, "Shared doc")
		f.share(space, "user", admin.id, "read")
		f.want(f.call(admin, "GET", "/spaces/"+space, nil), http.StatusOK, "admin with read share sees the space")
		if got := itemCaps(f, admin, item); len(got) != 1 || got[0] != "view" {
			t.Fatalf("admin with read share must hold only view on the item, got %v", got)
		}
		f.want(f.call(admin, "PATCH", "/items/"+item, map[string]any{"title": "x"}, "If-Match", `"1"`), http.StatusForbidden, "read share cannot edit")
	})

	t.Run("workspace admin cannot open another user's personal space because of the user type", func(t *testing.T) {
		personal := f.queryID("SELECT id::text FROM spaces WHERE kind='personal' AND owner_id=$1", other.id)
		f.want(f.call(admin, "GET", "/spaces/"+personal, nil), http.StatusNotFound, "admin, another user's personal space")
		f.want(f.call(admin, "GET", "/spaces/"+personal+"/items", nil), http.StatusNotFound, "admin, another user's personal items")
	})

	t.Run("membership revoke takes effect on the next request", func(t *testing.T) {
		space := f.spaceWithoutAdmins("Revoke")
		f.share(space, "user", owner.id, "full")
		f.share(space, "user", other.id, "read")
		f.want(f.call(other, "GET", "/spaces/"+space, nil), http.StatusOK, "member reads before revoke")
		version := f.call(owner, "GET", "/spaces/"+space, nil).header.Get("ETag")
		f.want(f.call(owner, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": []map[string]string{
			{"principal_type": "user", "principal_id": owner.id, "role": "full"},
		}}, "If-Match", version), http.StatusOK, "owner removes the member")
		f.want(f.call(other, "GET", "/spaces/"+space, nil), http.StatusNotFound, "removed member loses access at once")
	})

	t.Run("move and share stay bounded across spaces and parent and child", func(t *testing.T) {
		from := f.spaceWithoutAdmins("From")
		to := f.spaceWithoutAdmins("To")
		f.share(from, "user", owner.id, "full")
		f.share(to, "user", owner.id, "full")
		mover := f.person("regular")
		f.share(from, "user", mover.id, "editor") // edit and create in the source only
		f.share(to, "user", mover.id, "read")     // can see the destination but not create there
		item := customFieldItem(f, mover, from, "Movable")
		f.want(f.call(mover, "PUT", "/items/"+item+"/locations", map[string]any{
			"locations": []any{map[string]any{"space_id": to, "parent_item_id": nil}},
		}), http.StatusForbidden, "move into a space where the mover cannot create")
		if n := f.queryID("SELECT count(*)::text FROM item_locations WHERE item_id=$1 AND space_id=$2 AND deleted_root_id IS NULL", item, to); n != "0" {
			t.Fatalf("a refused cross-space move must write nothing: %s", n)
		}
		f.want(f.call(mover, "PUT", "/items/"+item+"/shares", map[string]any{"shares": []map[string]string{
			{"principal_type": "user", "principal_id": other.id, "role": "full"},
		}}, "If-Match", `"1"`), http.StatusForbidden, "editor cannot share the item")
	})

	t.Run("direct API calls cannot bypass the permission check", func(t *testing.T) {
		space := f.spaceWithoutAdmins("Direct API")
		f.share(space, "user", owner.id, "full")
		item := customFieldItem(f, owner, space, "Guarded")
		f.share(space, "user", other.id, "read")
		etag := f.call(other, "GET", "/items/"+item, nil).header.Get("ETag")
		f.want(f.call(other, "PATCH", "/items/"+item, map[string]any{"title": "hacked"}, "If-Match", etag), http.StatusForbidden, "read member cannot patch")
		f.want(f.call(other, "DELETE", "/items/"+item, nil, "If-Match", etag), http.StatusForbidden, "read member cannot delete")
		f.want(f.call(other, "PUT", "/items/"+item+"/shares", map[string]any{"shares": []map[string]string{}}, "If-Match", etag), http.StatusForbidden, "read member cannot change shares")
		f.want(f.call(admin, "PATCH", "/items/"+item, map[string]any{"title": "hacked"}, "If-Match", etag), http.StatusNotFound, "admin without share cannot patch")
		f.want(f.call(nil, "GET", "/items/"+item, nil), http.StatusUnauthorized, "anonymous request is refused")
	})
}
