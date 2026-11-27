package httpserver_test

import (
	"net/http"
	"testing"
)

// Audit probe S-4 (LENS RBAC audit): a custom role with Share but without Delete must not be able to revoke a
// Full share that another person holds on the item. Revoking above your own capabilities is refused in a Space
// (spaces/members.go guardSpaceShareChange); this checks the same rule for an item.
func TestItemShareRevokeAboveOwnCapabilitiesIsRefused(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	actor := f.person("regular")
	victim := f.person("regular")
	space := f.space("Shared")
	f.share(space, "user", admin.id, "full") // the admin creates this Space's content as an explicit member
	f.share(space, "user", actor.id, "read") // the actor can see the Space and reach the item
	item := f.queryID("INSERT INTO items(type,title,created_by,scope_state) VALUES ('task','Plan',$1,'internal') RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	path := "/items/" + item + "/shares"

	role := newRole(f, admin, "sharer without delete", "view", "share")
	first := f.call(admin, "GET", path, nil)
	f.want(first, http.StatusOK, "read item shares")
	f.want(f.call(admin, "PUT", path, map[string]any{"shares": []map[string]string{
		{"principal_type": "user", "principal_id": actor.id, "role": "custom:" + role},
		{"principal_type": "user", "principal_id": victim.id, "role": "full"},
	}}, "If-Match", first.header.Get("ETag")), http.StatusOK, "admin grants the sharer and a Full holder")

	// The sharer tries to remove the Full holder.
	cur := f.call(admin, "GET", path, nil)
	res := f.call(actor, "PUT", path, map[string]any{"shares": []map[string]string{
		{"principal_type": "user", "principal_id": actor.id, "role": "custom:" + role},
	}}, "If-Match", cur.header.Get("ETag"))
	t.Logf("sharer revoking a Full holder: status %d body %v", res.status, res.body)
	f.want(res, http.StatusForbidden, "revoking a Full share above your own capabilities")

	// Nothing changed: the Full holder keeps the share.
	if n := f.queryID("SELECT count(*)::text FROM shares WHERE target_type='item' AND target_id=$1 AND principal_id=$2 AND role='full'", item, victim.id); n != "1" {
		t.Fatalf("a refused revoke must keep the Full share: %s", n)
	}
}
