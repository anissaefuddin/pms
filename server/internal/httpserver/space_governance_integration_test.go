package httpserver_test

import (
	"fmt"
	"testing"
)

// D-01 visibility and join, D-03 archive, D-04 icon, D-SA1 explicit admins (see docs/SPACE_MANAGEMENT_ANALISA.md).
func TestSpaceGovernance(t *testing.T) {
	f := newFixture(t)
	owner := f.person("regular")
	member := f.person("regular")
	outsider := f.person("regular")
	external := f.person("external")
	space := f.space("Ops")
	f.share(space, "user", owner.id, "full")
	f.share(space, "user", member.id, "full") // a Full member is not an admin (D-SA1)
	f.exec("INSERT INTO space_admins(space_id,user_id) VALUES ($1,$2)", space, owner.id)

	// D-01: new Spaces are private; only eligible internal users can find a public one, and Join gives Read only.
	f.want(f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"visibility": "public"}, "If-Match", `"1"`), 200, "a Space admin makes it public")
	if list := f.call(outsider, "GET", "/spaces/discover", nil); len(list.items()) != 1 {
		t.Fatalf("an eligible user finds the public Space: %v", list.body)
	}
	if list := f.call(external, "GET", "/spaces/discover", nil); len(list.items()) != 0 {
		t.Fatalf("an external user does not browse public Spaces: %v", list.body)
	}
	f.want(f.call(external, "POST", "/spaces/"+space+"/join", nil), 403, "external cannot join")
	f.want(f.call(outsider, "GET", "/spaces/"+space, nil), 404, "not a member before joining")
	joined := f.call(outsider, "POST", "/spaces/"+space+"/join", nil)
	f.want(joined, 200, "join")
	if got := joined.caps(); fmt.Sprint(got) != "[view]" {
		t.Fatalf("join grants Read only, got %v", got)
	}
	if list := f.call(outsider, "GET", "/spaces/discover", nil); len(list.items()) != 0 {
		t.Fatalf("a joined Space is no longer listed for discovery")
	}
	f.want(f.call(member, "PATCH", "/spaces/"+space, map[string]any{"visibility": "private"}, "If-Match", fmt.Sprintf(`"%d"`, versionOf(f, space))), 403, "a Full member is not a Space admin")
	f.want(f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"visibility": "sideways"}, "If-Match", fmt.Sprintf(`"%d"`, versionOf(f, space))), 422, "invalid visibility")

	// D-04: icons come from the catalogue only.
	f.want(f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"icon_key": "rocket"}, "If-Match", fmt.Sprintf(`"%d"`, versionOf(f, space))), 200, "catalogue icon")
	f.want(f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"icon_key": "not-an-icon"}, "If-Match", fmt.Sprintf(`"%d"`, versionOf(f, space))), 422, "icon outside the catalogue")

	// D-SA1: admins are explicit. Only admins manage admins; the member must already have access; the last admin stays.
	f.want(f.call(member, "POST", "/spaces/"+space+"/admins", map[string]any{"user_id": member.id}), 403, "a Full member cannot grant admin")
	stranger := f.person("regular")
	f.want(f.call(owner, "POST", "/spaces/"+space+"/admins", map[string]any{"user_id": stranger.id}), 422, "admin must be a member")
	f.want(f.call(owner, "POST", "/spaces/"+space+"/admins", map[string]any{"user_id": member.id}), 204, "admin grant to a member")
	f.want(f.call(owner, "DELETE", "/spaces/"+space+"/admins/"+owner.id, nil), 204, "remove one of two admins")
	f.want(f.call(member, "DELETE", "/spaces/"+space+"/admins/"+member.id, nil), 422, "the last admin cannot be removed")
	f.want(f.call(member, "POST", "/spaces/"+space+"/transfer-ownership", map[string]any{"user_id": owner.id}), 204, "transfer to an existing member")
	if n := f.queryID("SELECT count(*)::text FROM space_admins WHERE space_id=$1 AND user_id=$2", space, member.id); n != "0" {
		t.Fatalf("the transferring admin loses the role, rows=%s", n)
	}

	// D-03: archive keeps everything, blocks changes, and restore needs an admin.
	item := f.call(owner, "POST", "/items", map[string]any{"type": "project", "title": "Kept", "locations": []map[string]any{{"space_id": space}}})
	f.want(item, 201, "item before archive")
	f.want(f.call(member, "POST", "/spaces/"+space+"/archive", nil, "If-Match", fmt.Sprintf(`"%d"`, versionOf(f, space))), 403, "a former admin cannot archive")
	f.want(f.call(owner, "POST", "/spaces/"+space+"/archive", nil, "If-Match", fmt.Sprintf(`"%d"`, versionOf(f, space))), 200, "archive by the current admin")
	f.want(f.call(member, "PATCH", "/spaces/"+space, map[string]any{"name": "Changed"}, "If-Match", fmt.Sprintf(`"%d"`, versionOf(f, space))), 403, "archived space refuses rename")
	f.want(f.call(member, "POST", "/items", map[string]any{"type": "project", "title": "Late", "locations": []map[string]any{{"space_id": space}}}), 403, "archived space refuses new items")
	f.want(f.call(owner, "GET", "/spaces/"+space+"/shares", nil), 200, "archived space still readable by members")
	if n := f.queryID("SELECT count(*)::text FROM items WHERE deleted_at IS NULL AND id=$1", item.body["id"].(string)); n != "1" {
		t.Fatalf("archive keeps items, rows=%s", n)
	}
	f.want(f.call(member, "POST", "/spaces/"+space+"/restore", nil, "If-Match", fmt.Sprintf(`"%d"`, versionOf(f, space))), 403, "a former admin cannot restore")
	f.want(f.call(owner, "POST", "/spaces/"+space+"/restore", nil, "If-Match", fmt.Sprintf(`"%d"`, versionOf(f, space))), 200, "restore by admin")
	f.want(f.call(owner, "POST", "/spaces/"+space+"/restore", nil, "If-Match", fmt.Sprintf(`"%d"`, versionOf(f, space))), 422, "restore of a live space")
}

// Current version of a Space, read back for If-Match.
func versionOf(f *fixture, space string) int {
	f.t.Helper()
	var v int
	if err := f.pool.QueryRow(f.ctx, "SELECT version FROM spaces WHERE id=$1", space).Scan(&v); err != nil {
		f.t.Fatal(err)
	}
	return v
}

// Batch 3: the registry lists only Spaces the caller can already see, with counts and Space admins.
func TestSpaceRegistryShowsOnlyWhatTheCallerCanSee(t *testing.T) {
	f := newFixture(t)
	owner := f.person("regular")
	outsider := f.person("regular")
	secret := f.space("Secret Ops")
	f.share(secret, "user", owner.id, "full")
	f.exec("INSERT INTO space_admins(space_id,user_id) VALUES ($1,$2)", secret, owner.id)
	f.exec("INSERT INTO views(scope_space_id,type,name,position,is_default) VALUES ($1,'board','Board',0,false)", secret)

	list := f.call(owner, "GET", "/spaces/registry", nil)
	f.want(list, 200, "owner registry")
	var found map[string]any
	for _, row := range list.items() {
		if row["name"] == "Secret Ops" {
			found = row
		}
	}
	if found == nil {
		t.Fatalf("the owner sees the Space: %v", list.body)
	}
	if admins, _ := found["admins"].([]any); len(admins) != 1 {
		t.Fatalf("the Space admin is named: %v", found["admins"])
	}
	if found["shares"] != float64(1) {
		t.Fatalf("one share is counted: %v", found["shares"])
	}
	other := f.call(outsider, "GET", "/spaces/registry", nil)
	f.want(other, 200, "outsider registry")
	for _, row := range other.items() {
		if row["name"] == "Secret Ops" {
			t.Fatalf("a private Space must not appear in the registry of someone without access: %v", row)
		}
	}
}
