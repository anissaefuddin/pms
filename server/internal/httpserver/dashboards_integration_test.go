package httpserver_test

import (
	"net/http"
	"testing"
)

func TestDashboardLifecycle(t *testing.T) {
	f := newFixture(t)
	admin, editor, reader, stranger := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	source, target := f.space("Dashboard source"), f.space("Dashboard destination")
	f.share(target, "user", admin.id, "full") // A1.2: admin creates this Space's content as an explicit member
	f.share(source, "user", admin.id, "full") // A1.2: the admin moves a dashboard out of the source Space
	f.share(source, "user", editor.id, "editor")
	f.share(source, "user", reader.id, "read")
	f.share(target, "user", reader.id, "read")
	f.share(target, "user", editor.id, "editor")
	task := customFieldItem(f, admin, source, "Task independent of dashboards")

	create := func(actor *person, scope any, name string) reply {
		return f.call(actor, http.MethodPost, "/dashboards", map[string]any{"space_id": scope, "name": name})
	}
	f.want(create(reader, source, "Denied"), 403, "viewer cannot create shared dashboard")
	f.want(create(stranger, source, "Hidden"), 404, "hidden Space")
	personalSpace := f.queryID("SELECT id::text FROM spaces WHERE kind='personal' AND owner_id=$1", editor.id)
	f.want(create(editor, personalSpace, "Duplicate personal scope"), 422, "personal dashboard has one canonical scope")
	created := create(editor, source, "  Delivery  ")
	f.want(created, 201, "create Space dashboard")
	if created.body["name"] != "Delivery" || created.header.Get("ETag") != `"1"` || len(created.body["widgets"].([]any)) != 0 {
		t.Fatalf("unexpected dashboard: %+v", created)
	}
	id := created.body["id"].(string)
	path := "/dashboards/" + id
	f.want(f.call(stranger, "GET", path, nil), 404, "non-member cannot read")
	f.want(f.call(reader, "GET", path, nil), 200, "reader can read")
	f.want(f.call(reader, "PATCH", path, map[string]string{"name": "Denied"}, "If-Match", `"1"`), 403, "reader cannot rename")
	f.want(f.call(editor, "PATCH", path, map[string]string{"name": "Missing If-Match"}), 428, "rename requires version")
	renamed := f.call(editor, "PATCH", path, map[string]string{"name": "Delivery overview"}, "If-Match", `"1"`)
	f.want(renamed, 200, "rename")
	if renamed.body["name"] != "Delivery overview" || renamed.header.Get("ETag") != `"2"` {
		t.Fatalf("unexpected rename: %+v", renamed)
	}
	f.want(f.call(editor, "PATCH", path, map[string]string{"name": "Stale"}, "If-Match", `"1"`), 409, "stale version")
	f.want(f.call(editor, "POST", path+"/move", map[string]string{"space_id": target}, "If-Match", `"2"`), 403, "move requires Delete at source")
	f.want(f.call(reader, "POST", path+"/duplicate", map[string]string{"space_id": target, "name": "No Create"}), 403, "copy requires destination Create")
	f.want(f.call(stranger, "POST", path+"/duplicate", map[string]any{"name": "Hidden copy"}), 404, "copy requires source View")

	widgets := path + "/widgets"
	for _, config := range []any{[]string{"bad"}, nil, map[string]any{"show_labels": "true"}, map[string]any{"source_id": task}} {
		f.want(f.call(editor, "POST", widgets, map[string]any{"name": "Bad", "type": "pie", "config": config}, "If-Match", `"2"`),
			422, "reject invalid or out-of-scope configuration")
	}
	f.want(f.call(editor, "POST", widgets, map[string]any{"name": "Bad kind", "type": "unknown"}, "If-Match", `"2"`), 422, "reject unsupported widget")
	f.want(f.call(editor, "POST", widgets, map[string]any{"name": "Bad style", "type": "table", "config": map[string]string{"pie_style": "ring"}}, "If-Match", `"2"`),
		422, "chart style only applies to pie")
	widget := f.call(editor, "POST", widgets, map[string]any{
		"name": "Status", "type": "pie", "position": 4, "config": map[string]any{"pie_style": "donut", "show_labels": true},
	}, "If-Match", `"2"`)
	f.want(widget, 201, "create widget")
	original := widget.body["widgets"].([]any)[0].(map[string]any)
	if original["position"] != float64(4) || widget.header.Get("ETag") != `"3"` {
		t.Fatalf("unexpected widget: %+v", widget)
	}
	copied := f.call(reader, "POST", path+"/duplicate", map[string]any{"name": "My copy", "space_id": nil})
	f.want(copied, 201, "reader copies visible dashboard to their personal scope")
	copyPath := "/dashboards/" + copied.body["id"].(string)
	copyWidget := copied.body["widgets"].([]any)[0].(map[string]any)
	if copied.body["space_id"] != nil || copied.body["owner_id"] != reader.id || copyWidget["id"] == original["id"] ||
		copyWidget["config"].(map[string]any)["pie_style"] != "donut" {
		t.Fatalf("unexpected copied dashboard: %+v", copied)
	}
	f.want(f.call(admin, "GET", copyPath, nil), 404, "another user's personal dashboard is private")
	own := f.call(reader, "GET", "/dashboards", nil)
	f.want(own, 200, "personal listing")
	if len(own.items()) != 1 || own.items()[0]["id"] != copied.body["id"] {
		t.Fatalf("unexpected personal list: %+v", own)
	}
	f.want(f.call(reader, "DELETE", copyPath+"/widgets/"+original["id"].(string), nil, "If-Match", `"1"`), 404, "widget UUID scoped to parent")

	widgetPath := widgets + "/" + original["id"].(string)
	f.want(f.call(editor, "PUT", widgetPath, map[string]any{"name": "Stale", "type": "table"}, "If-Match", `"2"`), 409, "widget writes use dashboard version")
	updated := f.call(editor, "PUT", widgetPath, map[string]any{"name": "Task status", "type": "table"}, "If-Match", `"3"`)
	f.want(updated, 200, "replace widget")
	if entry := updated.body["widgets"].([]any)[0].(map[string]any); entry["type"] != "table" || len(entry["config"].(map[string]any)) != 0 {
		t.Fatalf("unexpected replacement: %+v", updated)
	}
	sharedCopy := f.call(editor, "POST", path+"/duplicate", map[string]any{"name": "Shared copy", "space_id": target})
	f.want(sharedCopy, 201, "duplicate across visible Spaces")
	withoutWidget := f.call(editor, "DELETE", widgetPath, nil, "If-Match", `"4"`)
	f.want(withoutWidget, 200, "delete individual widget")
	if len(withoutWidget.body["widgets"].([]any)) != 0 {
		t.Fatalf("widget still present: %+v", withoutWidget)
	}
	moved := f.call(admin, "POST", path+"/move", map[string]any{"space_id": target}, "If-Match", `"5"`)
	f.want(moved, 200, "move dashboard between Spaces")
	if moved.body["space_id"] != target || moved.body["id"] != id {
		t.Fatalf("unexpected move: %+v", moved)
	}
	sourceList := f.call(reader, "GET", "/dashboards?space_id="+source, nil)
	f.want(sourceList, 200, "source listing")
	if len(sourceList.items()) != 0 {
		t.Fatalf("moved dashboard still listed: %+v", sourceList)
	}
	private := f.call(admin, "POST", path+"/move", map[string]any{"space_id": nil}, "If-Match", `"6"`)
	f.want(private, 200, "move into personal transfers ownership to mover")
	f.want(f.call(editor, "GET", path, nil), 404, "previous creator cannot access personal move")
	f.want(f.call(admin, "DELETE", path, nil, "If-Match", `"7"`), 204, "delete personal dashboard")
	f.want(f.call(admin, "DELETE", "/dashboards/"+sharedCopy.body["id"].(string), nil, "If-Match", `"1"`), 204, "delete dashboard with widgets")
	if count := f.queryID("SELECT count(*)::text FROM widgets WHERE dashboard_id=$1", sharedCopy.body["id"]); count != "0" {
		t.Fatal("widgets were not deleted")
	}
	f.want(f.call(admin, "GET", "/items/"+task, nil), 200, "dashboard deletion preserves task")
}

func TestDashboardSpaceLifecycle(t *testing.T) {
	f := newFixture(t)
	admin, outsider := f.person("admin"), f.person("viewer")
	space := f.space("Dashboard archive")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	created := f.call(admin, "POST", "/dashboards", map[string]any{"space_id": space, "name": "Retained"})
	f.want(created, 201, "create")
	path := "/dashboards/" + created.body["id"].(string)
	f.want(f.call(outsider, "POST", "/dashboards", map[string]string{"name": "Denied"}), 403, "personal creation obeys user type ceiling")
	f.want(f.call(admin, "GET", "/dashboards?space_id=invalid", nil), 422, "bad scope")
	f.exec("UPDATE spaces SET deleted_at=now() WHERE id=$1", space)
	f.want(f.call(admin, "GET", path, nil), 404, "deleted Space hides dashboard")
	f.exec("UPDATE spaces SET deleted_at=NULL WHERE id=$1", space)
	f.want(f.call(admin, "GET", path, nil), 200, "restored Space reveals dashboard")
}
