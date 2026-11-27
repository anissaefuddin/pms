package httpserver_test

import (
	"net/http"
	"reflect"
	"testing"
)

func TestGetItemDetail(t *testing.T) {
	f := newFixture(t)
	creator := f.person("regular")
	reader := f.person("viewer")
	first := f.space("First")
	second := f.space("Second")
	group := f.group("Readers", reader)
	f.share(second, "group", group, "full")
	id := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Plan',$1) RETURNING id::text", creator.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'a'),($1,$3,'a')", id, first, second)

	reply := f.call(reader, http.MethodGet, "/items/"+id, nil)
	if reply.status != http.StatusOK || reply.header.Get("ETag") != `"1"` || reply.body["title"] != "Plan" {
		t.Fatalf("item detail: status=%d etag=%q body=%v", reply.status, reply.header.Get("ETag"), reply.body)
	}
	if got, want := reply.caps(), expectedCaps("viewer", "full"); !reflect.DeepEqual(got, want) {
		t.Fatalf("capabilities = %v, want %v", got, want)
	}
	wantLocations := []any{map[string]any{"space_id": second, "parent_item_id": nil}}
	if !reflect.DeepEqual(reply.body["locations"], wantLocations) {
		t.Fatalf("locations = %v, want %v", reply.body["locations"], wantLocations)
	}
}

func TestItemVisibilityBoundaries(t *testing.T) {
	f := newFixture(t)
	owner := f.person("regular")
	member := f.person("regular")
	admin := f.person("admin")
	privateSpace := f.queryID("SELECT id::text FROM spaces WHERE owner_id=$1 AND kind='personal'", owner.id)
	sharedSpace := f.internalSpace("Team")
	create := func(space string) string {
		t.Helper()
		r := f.call(owner, http.MethodPost, "/items", map[string]any{
			"type": "task", "title": "Plan",
			"locations": []map[string]any{{"space_id": space}},
		})
		if r.status != http.StatusCreated {
			t.Fatalf("fixture create: %d %v", r.status, r.body)
		}
		return r.body["id"].(string)
	}
	f.share(sharedSpace, "user", owner.id, "editor")
	shared, personal := create(sharedSpace), create(privateSpace)
	for _, tc := range []struct {
		name  string
		actor *person
		item  string
		want  int
	}{
		{"owner can read Personal Item", owner, personal, http.StatusOK},
		{"admin cannot read another Personal Item", admin, personal, http.StatusNotFound},
		{"member without a grant cannot read shared Item", member, shared, http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := f.call(tc.actor, http.MethodGet, "/items/"+tc.item, nil)
			if r.status != tc.want {
				t.Fatalf("status=%d want=%d body=%v", r.status, tc.want, r.body)
			}
		})
	}
}

func TestItemLifecycle(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	assignee := f.person("regular")
	space, other := f.internalSpace("Work"), f.internalSpace("Other")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates the lifecycle items in both Spaces
	f.share(other, "user", admin.id, "full")
	create := func(title, kind string, locations ...map[string]any) string {
		t.Helper()
		r := f.call(admin, http.MethodPost, "/items", map[string]any{
			"type": kind, "title": title, "locations": locations, "assignee_ids": []string{assignee.id},
		})
		if r.status != 201 || r.header.Get("ETag") != `"1"` {
			t.Fatalf("create: %d %v", r.status, r.body)
		}
		return r.body["id"].(string)
	}
	parent := create("Project", "project", map[string]any{"space_id": space})
	child := create("Child", "task", map[string]any{"space_id": space, "parent_item_id": parent})
	retained := create("Kept", "task",
		map[string]any{"space_id": space, "parent_item_id": parent}, map[string]any{"space_id": other})
	path := "/items/" + child
	if got := f.call(admin, http.MethodPatch, path, map[string]any{"title": "Updated"}); got.status != 428 {
		t.Fatalf("patch without precondition: %d %v", got.status, got.body)
	}
	completed := f.queryID("SELECT id::text FROM statuses WHERE grp='completed'")
	workflows := f.call(admin, http.MethodGet, "/workflows", nil)
	if workflows.status != 200 || len(workflows.items()) != 1 || workflows.items()[0]["is_default"] != true {
		t.Fatalf("workflows: %d %v", workflows.status, workflows.body)
	}
	statuses, _ := workflows.items()[0]["statuses"].([]any)
	if len(statuses) != 5 || statuses[2].(map[string]any)["id"] != completed {
		t.Fatalf("workflow statuses: %v", statuses)
	}
	r := f.call(admin, http.MethodPatch, path, map[string]any{
		"title": "Updated", "status_id": completed, "start_date": "2026-11-12",
		"description": map[string]any{"type": "doc", "content": []any{
			map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Planning notes"}}},
		}},
	}, "If-Match", `"1"`)
	if r.status != 200 || r.header.Get("ETag") != `"2"` || r.body["completed_at"] == nil {
		t.Fatalf("patch: %d %v", r.status, r.body)
	}
	if got := f.call(admin, http.MethodPatch, path, map[string]any{"title": "Stale"}, "If-Match", `"1"`); got.status != 409 {
		t.Fatalf("stale patch: %d %v", got.status, got.body)
	}
	if got := f.call(admin, http.MethodPatch, path, map[string]any{"due_date": "2026-11-11"}, "If-Match", `"2"`); got.status != 422 {
		t.Fatalf("invalid date: %d %v", got.status, got.body)
	}
	if got := f.queryID("SELECT version::text FROM items WHERE id=$1", child); got != "2" {
		t.Fatalf("failed patch changed version to %s", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM item_followers WHERE item_id=$1", child); got != "2" {
		t.Fatalf("followers = %s, want creator and assignee", got)
	}
	if got := f.queryID("SELECT changes->'title'->>'to' FROM activity WHERE item_id=$1 AND action='update'", child); got != "Updated" {
		t.Fatalf("recorded title = %s", got)
	}
	// Force an activity write failure after the Item write in this test database.
	f.exec("ALTER TABLE activity ADD CONSTRAINT test_activity_write_fails CHECK (action <> 'update') NOT VALID")
	failed := f.call(admin, http.MethodPatch, path, map[string]any{"title": "Must roll back"}, "If-Match", `"2"`)
	f.exec("ALTER TABLE activity DROP CONSTRAINT test_activity_write_fails")
	if failed.status < 400 {
		t.Fatalf("patch succeeded without activity: %d %v", failed.status, failed.body)
	}
	if got := f.queryID("SELECT title || ':' || version::text FROM items WHERE id=$1", child); got != "Updated:2" {
		t.Fatalf("activity failure left a partial update: %s", got)
	}
	r = f.call(admin, http.MethodPost, path+"/reorder", map[string]any{
		"space_id": space, "parent_item_id": parent, "after_item_id": retained,
	})
	if r.status != 204 {
		t.Fatalf("reorder: %d %v", r.status, r.body)
	}
	if got := f.queryID(`SELECT (a.rank COLLATE "C" > b.rank COLLATE "C")::text FROM item_locations a,item_locations b
WHERE a.item_id=$1 AND b.item_id=$2 AND a.space_id=$3 AND b.space_id=$3`, child, retained, space); got != "true" {
		t.Fatalf("reorder did not place child after sibling: %s", got)
	}
	if got := f.call(admin, http.MethodDelete, "/items/"+parent, nil); got.status != 204 {
		t.Fatalf("delete: %d %v", got.status, got.body)
	}
	if got := f.queryID("SELECT deleted_root_id::text FROM items WHERE id=$1", child); got != parent {
		t.Fatalf("child deleted_root_id = %s", got)
	}
	if got := f.call(admin, http.MethodGet, "/items/"+retained, nil); got.status != 200 || !reflect.DeepEqual(got.body["locations"], []any{map[string]any{"space_id": other, "parent_item_id": nil}}) {
		t.Fatalf("retained item: %d %v", got.status, got.body)
	}
}

func TestItemAuthorizationMatrix(t *testing.T) {
	f := newFixture(t)
	creator := f.person("admin")
	completed := f.queryID("SELECT id::text FROM statuses WHERE grp='completed'")
	for _, userType := range []string{"admin", "regular", "external", "collaborator", "viewer"} {
		actor := f.person(userType)
		group := f.group("Item matrix "+userType, actor)
		for _, role := range []string{"read", "limited", "editor", "full"} {
			for _, source := range []string{"direct", "group", "ancestor", "second_location"} {
				t.Run(userType+"/"+role+"/"+source, func(t *testing.T) {
					space := f.scopedSpace(t.Name(), "internal", "") // A1: the actor's own share decides
					insert := func(kind string, parent *string) string {
						t.Helper()
						id := f.queryID("INSERT INTO items(type,title,created_by,scope_state) VALUES ($1::item_type,'Matrix',$2,'internal') RETURNING id::text", kind, creator.id)
						f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'U')", id, space, parent)
						return id
					}
					var parent *string
					if source == "ancestor" {
						p := insert("project", nil)
						parent = &p
					}
					item := insert("task", parent)
					createSpace := space
					switch source {
					case "direct":
						f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,'user',$2,$3::share_role)", item, actor.id, role)
					case "group":
						f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,'group',$2,$3::share_role)", item, group, role)
					case "ancestor":
						f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,'user',$2,$3::share_role)", *parent, actor.id, role)
					case "second_location":
						second := f.scopedSpace(t.Name()+" second", "internal", "")
						createSpace = second
						f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, second)
						f.share(second, "user", actor.id, role)
					}
					want := expectedCaps(userType, role) // A1: the actor's own share is the only grant, admins included
					can := map[string]bool{}
					for _, cap := range want {
						can[cap] = true
					}
					path := "/items/" + item
					detail := f.call(actor, http.MethodGet, path, nil)
					if detail.status != http.StatusOK || !reflect.DeepEqual(detail.caps(), want) {
						t.Fatalf("detail status=%d caps=%v want=%v", detail.status, detail.caps(), want)
					}
					tag := detail.header.Get("ETag")
					for _, tc := range []struct {
						cap  string
						body map[string]any
					}{
						{"edit", map[string]any{"title": "Edited"}},
						{"change_status", map[string]any{"status_id": completed}},
					} {
						r := f.call(actor, http.MethodPatch, path, tc.body, "If-Match", tag)
						expected := http.StatusForbidden
						if can[tc.cap] {
							expected = http.StatusOK
							tag = r.header.Get("ETag")
						}
						if r.status != expected {
							t.Fatalf("%s: status=%d want=%d body=%v", tc.cap, r.status, expected, r.body)
						}
					}
					r := f.call(actor, http.MethodPost, "/items", map[string]any{
						"type": "task", "title": "Subtask",
						"locations": []map[string]any{{"space_id": createSpace, "parent_item_id": item}},
					})
					expected := http.StatusForbidden
					if can["create"] {
						expected = http.StatusCreated
					}
					if r.status != expected {
						t.Fatalf("create status=%d want=%d body=%v", r.status, expected, r.body)
					}
					r = f.call(actor, http.MethodDelete, path, nil)
					expected = http.StatusForbidden
					if can["delete"] {
						expected = http.StatusNoContent
					}
					if r.status != expected {
						t.Fatalf("delete status=%d want=%d body=%v", r.status, expected, r.body)
					}
				})
			}
		}
	}
}
