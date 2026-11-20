package httpserver_test

import (
	"net/http"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/security"
)

// Residual risk (LENS RBAC closure): a Space Admin whose user type is lowered below the admin/regular rule (S-1)
// must lose the Space-admin authority at once. The explicit role row alone must not keep it, for example the right
// to move an item out of a locked Space.
func TestAuthorizationSpaceAdminDowngradeLosesSpaceAuthority(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	owner := f.person("regular")
	locked := f.space("Locked")
	dest := f.space("Destination")
	f.share(locked, "user", owner.id, "full")
	f.share(dest, "user", owner.id, "full")
	f.exec("INSERT INTO space_admins(space_id,user_id) VALUES ($1,$2)", locked, owner.id)
	item := customFieldItem(f, owner, locked, "Stays until admin moves it")
	f.lock(admin, locked, true)

	// Before the downgrade the explicit Space Admin may move the item out of the locked Space.
	before := f.call(owner, "GET", "/spaces/"+locked, nil)
	f.want(before, http.StatusOK, "explicit Space Admin reads the locked space")
	if before.body["is_space_admin"] != true {
		t.Fatalf("before the downgrade the owner must be a Space Admin: %v", before.body)
	}

	// The workspace admin lowers the owner to external (below the admin/regular rule).
	ver := f.call(admin, "GET", "/users/"+owner.id, nil)
	f.want(ver, http.StatusOK, "read user")
	f.want(f.call(admin, "PATCH", "/users/"+owner.id, map[string]any{"user_type": "external"}, "If-Match", ver.header.Get("ETag")), http.StatusOK, "downgrade to external")

	// The downgrade ends the old session at once (a security response, not the authority check under test).
	f.want(f.call(owner, "GET", "/spaces/"+locked, nil), http.StatusUnauthorized, "old session ends after the downgrade")

	// A fresh session for the same user, now external: the Space-admin authority must be gone on its own.
	token, hash, _ := security.Token()
	f.exec("INSERT INTO sessions(token_hash,user_id,expires_at,absolute_expires_at) VALUES ($1,$2,now()+interval '1 day',now()+interval '2 days')", hash, owner.id)
	fresh := &person{id: owner.id, userType: "external", cookie: &http.Cookie{Name: "pms_session", Value: token}}
	after := f.call(fresh, "GET", "/spaces/"+locked, nil)
	f.want(after, http.StatusOK, "external owner still reads the Space through the Full share")
	if after.body["is_space_admin"] == true {
		t.Fatalf("a downgraded user must not keep the Space Admin flag: %v", after.body)
	}
	f.want(f.call(fresh, "PUT", "/items/"+item+"/locations", map[string]any{
		"locations": []any{map[string]any{"space_id": dest, "parent_item_id": nil}},
	}), http.StatusForbidden, "downgraded owner cannot move an item out of the locked Space")
	if n := f.queryID("SELECT count(*)::text FROM item_locations WHERE item_id=$1 AND space_id=$2 AND deleted_root_id IS NULL", item, locked); n != "1" {
		t.Fatalf("a refused move must leave the item in the locked Space: %s", n)
	}
}
