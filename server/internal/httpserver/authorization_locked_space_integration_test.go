package httpserver_test

import (
	"net/http"
	"testing"
)

// Locked-Space protection (D-95): a Full member who is not a Space Admin cannot move an item out of a locked Space.
// The check must read the item's locations before this request removes them.
func TestAuthorizationLockedSpaceBlocksNonAdminMoveOut(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	owner := f.person("regular")
	member := f.person("regular")
	locked := f.space("Locked")
	dest := f.space("Destination")
	f.share(locked, "user", owner.id, "full")
	f.share(locked, "user", member.id, "full") // Full, but not a Space Admin
	f.share(dest, "user", member.id, "full")
	item := customFieldItem(f, owner, locked, "Locked item")
	f.lock(admin, locked, true)

	f.want(f.call(member, "PUT", "/items/"+item+"/locations", map[string]any{
		"locations": []any{map[string]any{"space_id": dest, "parent_item_id": nil}},
	}), http.StatusForbidden, "a Full non-admin cannot move an item out of a locked Space")
	if n := f.queryID("SELECT count(*)::text FROM item_locations WHERE item_id=$1 AND space_id=$2 AND deleted_root_id IS NULL", item, locked); n != "1" {
		t.Fatalf("a refused move must leave the item in the locked Space: %s", n)
	}
}
