package httpserver_test

import "testing"

func TestRecycleRestoresCascadeAndMissingParent(t *testing.T) {
	f := newFixture(t)
	admin, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	a, b := f.space("Primary"), f.space("Alternate")
	f.share(a, "user", reader.id, "read")
	makeItem := func(kind, title string) string {
		return f.queryID("INSERT INTO items(type,title,created_by) VALUES ($1,$2,$3) RETURNING id::text", kind, title, admin.id)
	}
	place := func(item, space string, parent *string, rank string) {
		f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,$4)", item, space, parent, rank)
	}
	parent := makeItem("project", "Pilot parent")
	branch := makeItem("folder", "Pilot branch")
	child := makeItem("task", "Pilot deleted child")
	multi := makeItem("task", "Pilot multi")
	place(parent, a, nil, "A")
	place(branch, a, &parent, "A")
	place(child, a, &branch, "A")
	place(multi, a, &branch, "B")
	place(multi, b, nil, "A")
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", child, reader.id)
	if got := f.call(admin, "DELETE", "/items/"+branch, nil); got.status != 204 {
		t.Fatalf("delete branch: %+v", got)
	}
	if got := f.call(admin, "GET", "/items/"+multi, nil); got.status != 200 {
		t.Fatalf("multi-location item hidden: %+v", got)
	}
	if got := f.call(admin, "GET", "/items/"+child, nil); got.status != 404 {
		t.Fatalf("deleted child visible: %+v", got)
	}
	if got := f.call(reader, "GET", "/me/todo?filter=all", nil); got.status != 200 || len(got.items()) != 0 {
		t.Fatalf("deleted child in to-do: %+v", got)
	}
	if got := f.call(reader, "GET", "/search?q=deleted+child", nil); got.status != 200 || len(got.items()) != 0 {
		t.Fatalf("deleted child in search: %+v", got)
	}
	if got := f.call(admin, "PATCH", "/items/"+child, map[string]any{"assignee_ids": []string{reader.id}}, "If-Match", `"1"`); got.status != 404 {
		t.Fatalf("deleted child mutation: %+v", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM notifications WHERE item_id=$1", child); got != "0" {
		t.Fatalf("notification for deleted child: %s", got)
	}
	if got := f.call(outsider, "GET", "/recycle-bin", nil); got.status != 200 || len(got.items()) != 0 {
		t.Fatalf("outsider bin: %+v", got)
	}
	path := "/recycle-bin/" + branch + "/restore"
	if got := f.call(reader, "POST", path, nil); got.status != 403 {
		t.Fatalf("read-only restore: %+v", got)
	}
	if got := f.call(admin, "DELETE", "/items/"+parent, nil); got.status != 204 {
		t.Fatalf("delete independent parent: %+v", got)
	}
	if got := f.call(admin, "GET", "/recycle-bin?type=folder&sort=title", nil); got.status != 200 || len(got.items()) != 1 || got.items()[0]["id"] != branch {
		t.Fatalf("bin filter: %+v", got)
	}
	if got := f.call(admin, "POST", path, nil); got.status != 200 {
		t.Fatalf("restore: %+v", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM item_locations WHERE item_id=$1 AND space_id=$2 AND parent_item_id IS NULL AND deleted_root_id IS NULL", branch, a); got != "1" {
		t.Fatalf("root fallback count: %s", got)
	}
	if got := f.call(admin, "GET", "/items/"+child, nil); got.status != 200 {
		t.Fatalf("child not restored: %+v", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM item_locations WHERE item_id=$1 AND deleted_root_id IS NULL", multi); got != "2" {
		t.Fatalf("multi-location restore count: %s", got)
	}
	if got := f.call(admin, "DELETE", "/spaces/"+a, nil); got.status != 204 {
		t.Fatalf("delete original Space: %+v", got)
	}
	if got := f.call(admin, "POST", path, nil); got.status != 409 || got.body["code"] != "restore_space_deleted" {
		t.Fatalf("restore deleted Space: %+v", got)
	}
}
