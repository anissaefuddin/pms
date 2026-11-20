package httpserver_test

import (
	"testing"

	"github.com/anissaefuddin/pms/server/internal/policy"
)

// Share-change rules that the plain grant check does not cover (see spaces/members.go).
func TestSpaceShareChangeRules(t *testing.T) {
	f := newFixture(t)
	owner := f.person("regular")
	space := f.space("Ops")
	f.share(space, "user", owner.id, "full")

	// Only the owner holds full access: removing it or emptying the list would leave no holder.
	shares := f.call(owner, "GET", "/spaces/"+space+"/shares", nil)
	f.want(f.call(owner, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": []any{}}, "If-Match", shares.header.Get("ETag")), 422, "last full holder")
	f.want(f.call(owner, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": []map[string]string{{"principal_type": "user", "principal_id": owner.id, "role": "editor"}}}, "If-Match", shares.header.Get("ETag")), 422, "lowering the last full holder")

	// A custom role with sharing but not everything of Full may not remove a Full share it cannot grant.
	mask, ok := policy.CapsFromNames([]string{"view", "share", "edit"})
	if !ok {
		t.Fatal("capability names must be known")
	}
	roleID := f.queryID("INSERT INTO custom_roles(name,caps_mask) VALUES ('sharer',$1) RETURNING id::text", mask.Mask())
	member := f.person("regular")
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,custom_role_id) VALUES ('space',$1,'user',$2,'read',$3)", space, member.id, roleID)
	shares = f.call(member, "GET", "/spaces/"+space+"/shares", nil)
	f.want(f.call(member, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": []any{}}, "If-Match", shares.header.Get("ETag")), 403, "remove a share above own capabilities")

	// Members list: direct and group reasons, and the group's member appears with the group as a source.
	group := f.group("Ops team", member)
	f.share(space, "group", group, "read")
	list := f.call(owner, "GET", "/spaces/"+space+"/members", nil)
	f.want(list, 200, "members list")
	if len(list.items()) < 2 {
		t.Fatalf("owner and the group member are listed: %v", list.body)
	}
}
