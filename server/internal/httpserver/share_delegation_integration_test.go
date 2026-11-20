package httpserver_test

import (
	"net/http"
	"testing"
)

// S-4 (LENS RBAC closure): a share change on an item is bounded by the actor's own capabilities, both ways.
// The actor holds a custom role with view and share but without delete, so it can manage Read shares only.
// Revoke of a Full share is covered by TestItemShareRevokeAboveOwnCapabilitiesIsRefused.
func TestItemShareDowngradeAndUpgradeAgainstOwnRights(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	actor := f.person("regular")
	holder := f.person("regular") // Full share on the item
	reader := f.person("regular") // Read share on the item
	space := f.space("Shared")
	f.share(space, "user", admin.id, "full")
	f.share(space, "user", actor.id, "read")
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Plan',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	path := "/items/" + item + "/shares"
	role := newRole(f, admin, "sharer without delete (downgrade)", "view", "share")

	set := func(who *person, shares []map[string]string) reply {
		cur := f.call(who, "GET", path, nil)
		f.want(cur, http.StatusOK, "read shares")
		return f.call(who, "PUT", path, map[string]any{"shares": shares}, "If-Match", cur.header.Get("ETag"))
	}
	adminSet := func(shares []map[string]string) {
		f.want(set(admin, shares), http.StatusOK, "admin sets shares")
	}
	grant := func(p *person, r string) map[string]string {
		return map[string]string{"principal_type": "user", "principal_id": p.id, "role": r}
	}
	sharer := grant(actor, "custom:"+role)
	roleOf := func(p *person) string {
		return f.queryID("SELECT role::text FROM shares WHERE target_type='item' AND target_id=$1 AND principal_id=$2", item, p.id)
	}

	// Setup: sharer, a Full holder and a Read holder.
	adminSet([]map[string]string{sharer, grant(holder, "full"), grant(reader, "read")})

	// Downgrade a Full share to Limited: Limited is above the sharer's own rights, so 403 and nothing changes.
	res := set(actor, []map[string]string{sharer, grant(holder, "limited"), grant(reader, "read")})
	f.want(res, http.StatusForbidden, "sharer cannot downgrade a Full holder")
	if got := roleOf(holder); got != "full" {
		t.Fatalf("a refused downgrade must keep Full, got %s", got)
	}

	// Upgrade a Read share to Limited: Limited is above the sharer's own rights, so 403.
	res = set(actor, []map[string]string{sharer, grant(holder, "full"), grant(reader, "limited")})
	f.want(res, http.StatusForbidden, "sharer cannot upgrade a Read holder to Limited")
	if got := roleOf(reader); got != "read" {
		t.Fatalf("a refused upgrade must keep Read, got %s", got)
	}

	// Positive control: the sharer may change a Read share, because Read is within its rights.
	res = set(actor, []map[string]string{sharer, grant(holder, "full"), grant(reader, "read")})
	f.want(res, http.StatusOK, "sharer keeps an unchanged Read share")
	res = set(actor, []map[string]string{sharer, grant(holder, "full")})
	f.want(res, http.StatusOK, "sharer removes a Read share that is within its rights")
	if n := f.queryID("SELECT count(*)::text FROM shares WHERE target_type='item' AND target_id=$1 AND principal_id=$2", item, reader.id); n != "0" {
		t.Fatalf("the Read share should be gone: %s", n)
	}

	// The admin (Full, with delete) can still downgrade the Full holder: nothing above the admin's rights.
	adminSet([]map[string]string{sharer, grant(holder, "read")})
	if got := roleOf(holder); got != "read" {
		t.Fatalf("admin downgrade must apply, got %s", got)
	}
}
