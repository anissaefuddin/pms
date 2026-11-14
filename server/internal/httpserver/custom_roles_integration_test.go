package httpserver_test

import (
	"fmt"
	"slices"
	"testing"
)

func itemCaps(f *fixture, p *person, id string) []string {
	f.t.Helper()
	r := f.call(p, "GET", "/items/"+id, nil)
	f.want(r, 200, "read item")
	body := r.body
	if inner, ok := body["item"].(map[string]any); ok {
		body = inner
	}
	var out []string
	for _, c := range body["my_capabilities"].([]any) {
		out = append(out, c.(string))
	}
	return out
}

func spaceCaps(f *fixture, p *person, id string) []string {
	f.t.Helper()
	r := f.call(p, "GET", "/spaces/"+id, nil)
	f.want(r, 200, "read space")
	return r.caps()
}

func newRole(f *fixture, admin *person, name string, caps ...string) string {
	f.t.Helper()
	r := f.call(admin, "POST", "/custom-roles", map[string]any{"name": name, "capabilities": caps})
	f.want(r, 201, "create role "+name)
	return r.body["id"].(string)
}

func TestCustomRolesLeastPrivilegeAndLifecycle(t *testing.T) {
	f := newFixture(t)
	admin, alice, bob, ext, viewer := f.person("admin"), f.person("regular"), f.person("regular"), f.person("external"), f.person("viewer")
	space := f.space("Custom role space")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Roles project',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, space)
	task := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Roles task',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'U')", task, space, project)
	shareSpace := func(who *person, role string, others ...map[string]any) reply {
		cur := f.call(admin, "GET", "/spaces/"+space+"/shares", nil)
		list := append([]map[string]any{{"principal_type": "user", "principal_id": who.id, "role": role}}, others...)
		return f.call(admin, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": list}, "If-Match", cur.header.Get("ETag"))
	}

	// Definition rules: admin only to change, everyone to read; view is mandatory; names are unique ignoring case.
	f.want(f.call(alice, "POST", "/custom-roles", map[string]any{"name": "x", "capabilities": []string{"view"}}), 403, "regular user cannot create")
	f.want(f.call(nil, "GET", "/custom-roles", nil), 401, "anonymous")
	for name, body := range map[string]map[string]any{
		"no view":       {"name": "No view", "capabilities": []string{"comment"}},
		"empty":         {"name": "Empty", "capabilities": []string{}},
		"unknown":       {"name": "Unknown", "capabilities": []string{"view", "fly"}},
		"repeated":      {"name": "Repeated", "capabilities": []string{"view", "view"}},
		"blank name":    {"name": "   ", "capabilities": []string{"view"}},
		"name too long": {"name": string(make([]byte, 61)), "capabilities": []string{"view"}},
	} {
		if r := f.call(admin, "POST", "/custom-roles", body); r.status != 422 && r.status != 400 {
			t.Errorf("%s: status %d, want 422", name, r.status)
		}
	}
	commenter := newRole(f, admin, "Commenter", "view", "comment")
	f.want(f.call(admin, "POST", "/custom-roles", map[string]any{"name": " commenter ", "capabilities": []string{"view"}}), 422, "duplicate name")
	if list := f.call(alice, "GET", "/custom-roles", nil); list.status != 200 || len(list.items()) != 1 || list.items()[0]["in_use"] != 0.0 {
		t.Fatalf("a regular user must read the list: %+v", list)
	}

	// A Space share with a custom role gives exactly the role's capabilities, on the Space and on everything inside it.
	f.want(shareSpace(alice, "custom:"+commenter), 200, "share")
	if got := spaceCaps(f, alice, space); !slices.Equal(got, []string{"view", "comment"}) {
		t.Fatalf("space caps = %v", got)
	}
	if got := itemCaps(f, alice, task); !slices.Equal(got, []string{"view", "comment"}) {
		t.Fatalf("item caps = %v", got)
	}
	shares := f.call(admin, "GET", "/spaces/"+space+"/shares", nil)
	if row := shares.items()[0]; row["role"] != "custom:"+commenter || row["role_name"] != "Commenter" {
		t.Fatalf("share row = %v", row)
	}
	f.want(f.call(alice, "PATCH", "/items/"+task, map[string]any{"title": "Nope"}, "If-Match", `"1"`), 403, "no edit")
	f.want(f.call(alice, "DELETE", "/items/"+task, nil), 403, "no delete")
	f.want(f.call(alice, "POST", "/items", map[string]any{"type": "task", "title": "x", "locations": []map[string]any{{"space_id": space}}}), 403, "no create")
	comment := map[string]any{"body": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Hi"}}}}}}
	f.want(f.call(alice, "POST", "/items/"+task+"/comments", comment), 201, "comment allowed")
	f.want(f.call(alice, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": []any{}}, "If-Match", f.call(alice, "GET", "/spaces/"+space+"/shares", nil).header.Get("ETag")), 403, "no share")

	// The role is live: removing a capability takes effect on the next request, and the update is audited with before and after.
	role := f.call(admin, "GET", "/custom-roles", nil).items()[0]
	f.want(f.call(admin, "PUT", "/custom-roles/"+commenter, map[string]any{"name": "Commenter", "capabilities": []string{"view"}}), 428, "If-Match required")
	f.want(f.call(admin, "PUT", "/custom-roles/"+commenter, map[string]any{"name": "Commenter", "capabilities": []string{"view"}}, "If-Match", `"99"`), 409, "stale")
	f.want(f.call(admin, "PUT", "/custom-roles/"+commenter, map[string]any{"name": "Commenter", "capabilities": []string{"comment"}}, "If-Match", `"1"`), 422, "update must keep view")
	updated := f.call(admin, "PUT", "/custom-roles/"+commenter, map[string]any{"name": "Reader+", "capabilities": []string{"view"}}, "If-Match", `"`+fmt.Sprint(role["version"])+`"`)
	f.want(updated, 200, "update role")
	f.want(f.call(alice, "POST", "/items/"+task+"/comments", comment), 403, "comment revoked at once")
	if got := spaceCaps(f, alice, space); !slices.Equal(got, []string{"view"}) {
		t.Fatalf("space caps after the update = %v", got)
	}
	if e := lastAudit(t, f, admin, "role.updated"); e["target_id"] != commenter || e["details"].(map[string]any)["shares"] != 1.0 {
		t.Fatalf("role.updated entry = %v", e)
	}
	if e := lastAudit(t, f, admin, "role.created"); e["details"].(map[string]any)["name"] != "Commenter" {
		t.Fatalf("role.created entry = %v", e)
	}

	// In use: delete is refused; the share list may not name unknown or malformed roles.
	f.want(f.call(admin, "DELETE", "/custom-roles/"+commenter, nil, "If-Match", `"2"`), 409, "role in use")
	for _, bad := range []string{"custom:00000000-0000-4000-8000-000000000000", "custom:nope", "custom:", "mask:255", "owner"} {
		f.want(shareSpace(alice, bad), 422, "bad role "+bad)
	}

	// Item-level share: rights flow down to children but not sideways.
	editRole := newRole(f, admin, "Editor without create", "view", "comment", "edit")
	cur := f.call(admin, "GET", "/items/"+project+"/shares", nil)
	f.want(f.call(admin, "PUT", "/items/"+project+"/shares", map[string]any{"shares": []map[string]any{{"principal_type": "user", "principal_id": bob.id, "role": "custom:" + editRole}}}, "If-Match", cur.header.Get("ETag")), 200, "item share")
	if got := itemCaps(f, bob, task); !slices.Equal(got, []string{"view", "comment", "edit"}) {
		t.Fatalf("child caps through an item share = %v", got)
	}
	f.want(f.call(bob, "PATCH", "/items/"+task, map[string]any{"title": "Edited by bob"}, "If-Match", `"1"`), 200, "edit allowed")
	f.want(f.call(bob, "POST", "/items", map[string]any{"type": "task", "title": "x", "locations": []map[string]any{{"space_id": space, "parent_item_id": project}}}), 403, "no create")
	other := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Other project',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", other, space)
	f.want(f.call(bob, "GET", "/items/"+other, nil), 404, "no access to the sibling project")

	// Creating items with a custom role in the initial shares is not supported and fails closed.
	f.want(f.call(admin, "POST", "/items", map[string]any{"type": "task", "title": "x", "locations": []map[string]any{{"space_id": space}},
		"shares": []map[string]any{{"principal_type": "user", "principal_id": bob.id, "role": "custom:" + editRole}}}), 422, "custom role in create shares")

	// Granting is limited to what the granter holds: a sharer without delete cannot hand out a role that has it.
	sharer := newRole(f, admin, "Sharer", "view", "share", "edit")
	everything := newRole(f, admin, "Everything", "view", "comment", "change_status", "attach", "edit", "create", "delete", "share")
	f.want(shareSpace(bob, "custom:"+sharer), 200, "make bob a sharer")
	grant := func(role string) reply {
		cur := f.call(bob, "GET", "/spaces/"+space+"/shares", nil)
		return f.call(bob, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": []map[string]any{
			{"principal_type": "user", "principal_id": bob.id, "role": "custom:" + sharer},
			{"principal_type": "user", "principal_id": alice.id, "role": role}}}, "If-Match", cur.header.Get("ETag"))
	}
	f.want(grant("custom:"+everything), 403, "cannot grant above own capabilities")
	f.want(grant("full"), 403, "built-in full is also above")
	f.want(grant("custom:"+editRole), 403, "comment is not among bob's capabilities either")
	f.want(grant("custom:"+newRole(f, admin, "Edit only", "view", "edit")), 200, "a role within bob's capabilities")

	// A user type caps every role: an external user never gets delete or share, a viewer never gets edit.
	f.want(shareSpace(ext, "custom:"+everything, map[string]any{"principal_type": "user", "principal_id": viewer.id, "role": "custom:" + everything}), 200, "share with all")
	if got := spaceCaps(f, ext, space); !slices.Equal(got, []string{"view", "comment", "change_status", "attach", "edit", "create"}) {
		t.Fatalf("external caps = %v", got)
	}
	if got := spaceCaps(f, viewer, space); !slices.Equal(got, []string{"view", "comment"}) {
		t.Fatalf("viewer caps = %v", got)
	}

	// Groups: custom and built-in roles from different rows combine as a union.
	team := f.group("Review team", alice)
	cur = f.call(admin, "GET", "/spaces/"+space+"/shares", nil)
	f.want(f.call(admin, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": []map[string]any{
		{"principal_type": "group", "principal_id": team, "role": "custom:" + commenter},
		{"principal_type": "user", "principal_id": alice.id, "role": "limited"}}}, "If-Match", cur.header.Get("ETag")), 200, "group and user shares")
	if got := spaceCaps(f, alice, space); !slices.Equal(got, []string{"view", "comment", "change_status", "attach"}) {
		t.Fatalf("union caps = %v", got)
	}

	// Fail-safe in the database: a custom share is stored with role read, and the two columns cannot disagree.
	f.exec("DELETE FROM shares")
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,custom_role_id) VALUES ('space',$1,'user',$2,'read',$3)", space, alice.id, everything)
	if _, err := f.pool.Exec(f.ctx, "INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,custom_role_id) VALUES ('space',$1,'user',$2,'editor',$3)", space, bob.id, everything); err == nil {
		t.Fatal("a custom share with a role other than read must be refused by the database")
	}
	if got := f.queryID("SELECT role::text FROM shares WHERE principal_id=$1", alice.id); got != "read" {
		t.Fatalf("stored role = %s", got)
	}
	// Deleting works once nothing uses the role, and is audited.
	f.exec("DELETE FROM shares")
	v := f.call(admin, "GET", "/custom-roles", nil)
	var version float64
	for _, r := range v.items() {
		if r["id"] == everything {
			version = r["version"].(float64)
		}
	}
	f.want(f.call(admin, "DELETE", "/custom-roles/"+everything, nil, "If-Match", fmt.Sprintf(`"%d"`, int(version))), 204, "delete unused role")
	f.want(f.call(admin, "DELETE", "/custom-roles/"+everything, nil, "If-Match", `"1"`), 404, "already gone")
	f.want(f.call(alice, "DELETE", "/custom-roles/"+sharer, nil, "If-Match", `"1"`), 403, "regular user cannot delete")
	if e := lastAudit(t, f, admin, "role.deleted"); e["target_id"] != everything {
		t.Fatalf("role.deleted entry = %v", e)
	}
}
