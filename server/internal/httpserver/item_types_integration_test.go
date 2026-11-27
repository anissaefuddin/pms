package httpserver_test

import (
	"testing"

	"github.com/google/uuid"
)

func TestItemTypesCRUDDefaultsAndChildRules(t *testing.T) {
	f := newFixture(t)
	admin, regular, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Types")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	f.share(space, "user", regular.id, "full")

	list := f.call(regular, "GET", "/item-types", nil)
	if list.status != 200 || len(list.items()) != 3 {
		t.Fatalf("built-ins: %+v", list)
	}
	builtin := map[string]string{}
	for _, it := range list.items() {
		builtin[it["key"].(string)] = it["id"].(string)
	}
	// Only admins write; the matrix of bad input is refused before anything is stored.
	body := map[string]any{"key": "bug", "name": "Bug", "base": "task", "icon": "bug", "defaults": map[string]any{"importance": "high", "duration_days": 2}}
	f.want(f.call(regular, "POST", "/item-types", body), 403, "regular cannot create")
	f.want(f.call(nil, "POST", "/item-types", body), 401, "anonymous")
	for name, bad := range map[string]map[string]any{
		"bad key": {"key": "Bug!", "name": "X", "base": "task"}, "no name": {"key": "x1", "base": "task"}, "bad base": {"key": "x1", "name": "X", "base": "epic"},
		"bad default":          {"key": "x1", "name": "X", "base": "task", "defaults": map[string]any{"importance": "urgent"}},
		"unknown default":      {"key": "x1", "name": "X", "base": "task", "defaults": map[string]any{"color": "red"}},
		"task with workflow":   {"key": "x1", "name": "X", "base": "task", "default_workflow_id": f.queryID("SELECT id::text FROM workflows LIMIT 1")},
		"task holding project": {"key": "x1", "name": "X", "base": "task", "allowed_children": []string{builtin["project"]}},
		"unknown child":        {"key": "x1", "name": "X", "base": "project", "allowed_children": []string{uuid.NewString()}},
	} {
		if got := f.call(admin, "POST", "/item-types", bad); got.status != 422 {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	bug := f.call(admin, "POST", "/item-types", body)
	f.want(bug, 201, "create bug")
	f.want(f.call(admin, "POST", "/item-types", map[string]any{"key": "bug", "name": "Other", "base": "task"}), 422, "duplicate key")
	f.want(f.call(admin, "POST", "/item-types", map[string]any{"key": "bug2", "name": "BUG", "base": "task"}), 422, "duplicate name, any case")
	epic := f.call(admin, "POST", "/item-types", map[string]any{"key": "epic", "name": "Epic", "base": "project", "allowed_children": []string{bug.body["id"].(string)}})
	f.want(epic, 201, "epic that only holds bugs")

	// A custom type creates items of its class, with its defaults filling what the caller left out.
	create := func(p *person, fields map[string]any) reply {
		fields["locations"] = []map[string]any{{"space_id": space}}
		return f.call(p, "POST", "/items", fields)
	}
	proj := create(regular, map[string]any{"type_key": "epic", "title": "Release 2"})
	f.want(proj, 201, "epic project")
	if proj.body["type"] != "project" || proj.body["item_type"].(map[string]any)["key"] != "epic" {
		t.Fatalf("epic: %+v", proj.body)
	}
	parent := proj.body["id"].(string)
	child := func(fields map[string]any) reply {
		fields["locations"] = []map[string]any{{"space_id": space, "parent_item_id": parent}}
		return f.call(regular, "POST", "/items", fields)
	}
	first := child(map[string]any{"type_key": "bug", "title": "Crash on save"})
	f.want(first, 201, "bug inside epic")
	if first.body["importance"] != "high" || first.body["duration_days"] != float64(2) {
		t.Fatalf("defaults not applied: %+v", first.body)
	}
	over := child(map[string]any{"type_key": "bug", "title": "Explicit", "importance": "low"})
	f.want(over, 201, "explicit value wins")
	if over.body["importance"] != "low" {
		t.Fatalf("explicit importance overridden: %+v", over.body)
	}
	f.want(child(map[string]any{"type": "task", "title": "Plain task"}), 422, "epic only holds bugs")
	f.want(child(map[string]any{"type": "folder", "title": "Folder"}), 422, "epic does not hold folders")
	f.want(child(map[string]any{"type_key": "nope", "title": "x"}), 422, "unknown type")
	f.want(child(map[string]any{"type_key": "bug", "type": "project", "title": "x"}), 422, "type contradicts class")

	// Moving an existing plain task under the epic is refused too (the location path).
	plain := create(regular, map[string]any{"type": "task", "title": "Loose"})
	f.want(plain, 201, "plain")
	got := f.call(regular, "PUT", "/items/"+plain.body["id"].(string)+"/locations", map[string]any{"locations": []map[string]any{{"space_id": space, "parent_item_id": parent}}}, "If-Match", `"1"`)
	if got.status != 422 {
		t.Fatalf("task moved under epic: %+v", got)
	}

	// Inheritance: a type that inherits importance takes the parent's when the caller gives none.
	sub := f.call(admin, "POST", "/item-types", map[string]any{"key": "subtask", "name": "Subtask", "base": "task", "defaults": map[string]any{"inherit": []string{"importance"}}})
	f.want(sub, 201, "inheriting type")
	f.want(f.call(admin, "POST", "/item-types", map[string]any{"key": "bad_inh", "name": "Bad", "base": "task", "defaults": map[string]any{"inherit": []string{"title"}}}), 422, "only importance may be inherited")
	hot := create(regular, map[string]any{"type": "project", "title": "Hot", "importance": "high"})
	inherited := f.call(regular, "POST", "/items", map[string]any{"type_key": "subtask", "title": "Child", "locations": []map[string]any{{"space_id": space, "parent_item_id": hot.body["id"]}}})
	f.want(inherited, 201, "inheriting child")
	if inherited.body["importance"] != "high" {
		t.Fatalf("importance not inherited: %+v", inherited.body)
	}

	// A project type can carry its own workflow; items of that type start in it, and a task type cannot name one.
	wf := f.call(admin, "POST", "/workflows", map[string]any{"name": "Engineering flow"})
	f.want(wf, 201, "second workflow")
	eng := f.call(admin, "POST", "/item-types", map[string]any{"key": "eng_project", "name": "Engineering project", "base": "project", "default_workflow_id": wf.body["id"]})
	f.want(eng, 201, "type with a workflow")
	engItem := create(regular, map[string]any{"type_key": "eng_project", "title": "Platform"})
	f.want(engItem, 201, "item of that type")
	if engItem.body["default_workflow_id"] != wf.body["id"] {
		t.Fatalf("workflow of the type not applied: %+v", engItem.body)
	}
	other := create(regular, map[string]any{"type": "project", "title": "Plain project"})
	if other.body["default_workflow_id"] != nil {
		t.Fatalf("a built-in project must keep the default workflow: %+v", other.body)
	}

	// Layout: valid sections are stored, malformed ones are refused.
	good := map[string]any{"sections": []any{map[string]any{"title": "Triage", "fields": []any{"status", "importance"}}, map[string]any{"title": "More", "fields": []any{"description"}}}}
	laid := f.call(admin, "PATCH", "/item-types/"+bug.body["id"].(string), map[string]any{"layout": good}, "If-Match", `"1"`)
	f.want(laid, 200, "layout saved")
	if secs := laid.body["layout"].(map[string]any)["sections"].([]any); len(secs) != 2 {
		t.Fatalf("layout not stored: %+v", laid.body)
	}
	for name, bad := range map[string]any{
		"unknown block":   map[string]any{"sections": []any{map[string]any{"title": "A", "fields": []any{"nope"}}}},
		"duplicate field": map[string]any{"sections": []any{map[string]any{"title": "A", "fields": []any{"status", "status"}}}},
		"no title":        map[string]any{"sections": []any{map[string]any{"title": " ", "fields": []any{"status"}}}},
		"extra key":       map[string]any{"sections": []any{map[string]any{"title": "A", "fields": []any{}, "x": 1}}},
		"no sections":     map[string]any{"sections": []any{}},
	} {
		if got := f.call(admin, "PATCH", "/item-types/"+bug.body["id"].(string), map[string]any{"layout": bad}, "If-Match", `"2"`); got.status != 422 {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	// Deactivating keeps every existing item and only blocks new ones.
	f.want(f.call(admin, "PATCH", "/item-types/"+bug.body["id"].(string), map[string]any{"active": false}), 428, "needs If-Match")
	f.want(f.call(admin, "PATCH", "/item-types/"+bug.body["id"].(string), map[string]any{"active": false}, "If-Match", `"9"`), 409, "stale")
	off := f.call(admin, "PATCH", "/item-types/"+bug.body["id"].(string), map[string]any{"active": false}, "If-Match", `"2"`)
	f.want(off, 200, "deactivate")
	f.want(child(map[string]any{"type_key": "bug", "title": "After"}), 422, "inactive type refused")
	if got := f.call(regular, "GET", "/items/"+first.body["id"].(string), nil); got.status != 200 || got.body["item_type"].(map[string]any)["key"] != "bug" {
		t.Fatalf("existing item lost its type: %+v", got)
	}
	f.want(f.call(regular, "PATCH", "/item-types/"+bug.body["id"].(string), map[string]any{"name": "x"}, "If-Match", `"3"`), 403, "regular cannot edit")
	f.want(f.call(admin, "PATCH", "/item-types/"+builtin["task"], map[string]any{"active": false}, "If-Match", `"1"`), 422, "built-in stays active")
	f.want(f.call(admin, "PATCH", "/item-types/"+bug.body["id"].(string), map[string]any{"key": "other"}, "If-Match", `"3"`), 422, "key is fixed")
	f.want(f.call(admin, "PATCH", "/item-types/"+uuid.NewString(), map[string]any{"name": "x"}, "If-Match", `"1"`), 404, "unknown")
	// Items created the old way (no type at all in the row) are still typed after the migration.
	legacy := f.queryID("INSERT INTO items(type,title,created_by,scope_state) VALUES ('task','Legacy',$1,'internal') RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'Z')", legacy, space)
	if got := f.call(admin, "GET", "/items/"+legacy, nil); got.status != 200 || got.body["item_type"].(map[string]any)["key"] != "task" {
		t.Fatalf("legacy item: %+v", got)
	}
	_ = outsider
}
