package httpserver_test

import (
	"net/http"
	"strings"
	"testing"
)

// Blueprint edit (manage_blueprint, optimistic version) and preview (use_blueprint, writes nothing).

type editFixture struct {
	editor, useOnly, manageOnly, reader, outsider *person
	source, target, bp, root                      string
}

func setupEditable(t *testing.T, f *fixture) editFixture {
	t.Helper()
	e := editFixture{editor: f.person("regular"), useOnly: f.person("regular"), manageOnly: f.person("regular"), reader: f.person("regular"), outsider: f.person("regular")}
	e.source, e.target = f.space("Edit source"), f.space("Edit target")
	f.share(e.source, "user", e.editor.id, "editor")
	f.share(e.target, "user", e.editor.id, "editor")
	f.share(e.target, "user", e.useOnly.id, "editor")
	f.share(e.source, "user", e.reader.id, "read")
	customRoleWith(f, "edit use only", bitView|bitUseBP, e.useOnly, e.source)
	customRoleWith(f, "edit manage only", bitView|bitManageBP, e.manageOnly, e.source)

	root := f.call(e.editor, "POST", "/items", map[string]any{"type": "project", "title": "Launch", "locations": []map[string]any{{"space_id": e.source}}})
	f.want(root, 201, "root")
	e.root = root.body["id"].(string)
	plan := f.call(e.editor, "POST", "/items", map[string]any{"type": "task", "title": "Plan", "start_date": "2026-03-02", "due_date": "2026-03-04",
		"locations": []map[string]any{{"space_id": e.source, "parent_item_id": e.root}}})
	f.want(plan, 201, "plan")
	ship := f.call(e.editor, "POST", "/items", map[string]any{"type": "task", "title": "Ship", "start_date": "2026-03-09", "due_date": "2026-03-10",
		"locations": []map[string]any{{"space_id": e.source, "parent_item_id": e.root}}})
	f.want(ship, 201, "ship")
	f.want(f.call(e.editor, "POST", "/items/"+ship.body["id"].(string)+"/dependencies", map[string]any{"predecessor_id": plan.body["id"], "type": "FS"}), 201, "dependency")
	saved := f.call(e.editor, "POST", "/spaces/"+e.source+"/blueprints", map[string]any{"name": "Editable", "source_item_id": e.root})
	f.want(saved, 201, "save")
	e.bp = saved.body["id"].(string)
	return e
}

func TestBlueprintEditNeedsManageAndTheCurrentVersion(t *testing.T) {
	f := newFixture(t)
	e := setupEditable(t, f)
	path := "/blueprints/" + e.bp

	f.want(f.call(e.reader, "PATCH", path, map[string]any{"version": 1, "name": "Reader"}), 403, "reader cannot edit")
	f.want(f.call(e.useOnly, "PATCH", path, map[string]any{"version": 1, "name": "Use only"}), 403, "use right cannot edit")
	f.want(f.call(e.outsider, "PATCH", path, map[string]any{"version": 1, "name": "Outsider"}), 404, "outsider is 404")
	f.want(f.call(e.editor, "PATCH", path, map[string]any{"name": "No version"}), 422, "version is required")

	edited := f.call(e.manageOnly, "PATCH", path, map[string]any{"version": 1, "name": "Renamed"})
	f.want(edited, 200, "manage right renames")
	if edited.body["version"] != float64(2) || edited.body["name"] != "Renamed" {
		t.Fatalf("after edit: %v", edited.body)
	}
	f.want(f.call(e.editor, "PATCH", path, map[string]any{"version": 1, "name": "Stale"}), 409, "stale version is a conflict")
	if n := f.queryID("SELECT name FROM blueprints WHERE id=$1", e.bp); n != "Renamed" {
		t.Fatalf("a conflicting edit changed the name: %s", n)
	}
	if n := f.queryID("SELECT count(*)::text FROM audit_events WHERE action='blueprint.edit' AND target_id=$1", e.bp); n != "1" {
		t.Fatalf("edit audit rows = %s", n)
	}
	f.want(f.call(e.editor, "PATCH", path, map[string]any{"version": 2}), 422, "nothing to change")
	f.want(f.call(e.editor, "PATCH", path, map[string]any{"version": 2, "name": "Renamed"}), 200, "rename to the same name is fine")
}

// An invalid tree is refused and leaves the stored tree exactly as it was.
func TestBlueprintEditRefusesInvalidTrees(t *testing.T) {
	f := newFixture(t)
	e := setupEditable(t, f)
	path := "/blueprints/" + e.bp
	before := f.queryID("SELECT tree::text FROM blueprints WHERE id=$1", e.bp)
	bad := []map[string]any{
		{"nodes": []map[string]any{{"parent": -1, "type": "epic", "title": "X", "importance": "high", "assignee_ids": []string{}}}, "dependencies": []any{}},
		{"nodes": []map[string]any{{"parent": 3, "type": "task", "title": "X", "importance": "high", "assignee_ids": []string{}}}, "dependencies": []any{}},
		{"nodes": []map[string]any{{"parent": -1, "type": "project", "title": "X", "importance": "urgent", "assignee_ids": []string{}}}, "dependencies": []any{}},
		{"nodes": []map[string]any{{"parent": -1, "type": "task", "title": "", "importance": "high", "assignee_ids": []string{}}}, "dependencies": []any{}},
		{"nodes": []map[string]any{{"parent": -1, "type": "task", "title": "X", "importance": "high", "assignee_ids": []string{}, "start_offset": 5, "due_offset": 2}}, "dependencies": []any{}},
		{"nodes": []map[string]any{{"parent": -1, "type": "task", "title": "X", "importance": "high", "assignee_ids": []string{}}}, "dependencies": []map[string]any{{"predecessor": 0, "successor": 0, "type": "FS", "lag": 0}}},
		{"nodes": []map[string]any{{"parent": -1, "type": "task", "title": "X", "importance": "high", "assignee_ids": []string{}}}, "dependencies": []map[string]any{{"predecessor": 0, "successor": 0, "type": "XX", "lag": 0}}},
	}
	for i, tree := range bad {
		f.want(f.call(e.editor, "PATCH", path, map[string]any{"version": 1, "tree": tree}), 422, "invalid tree "+string(rune('A'+i)))
	}
	if after := f.queryID("SELECT tree::text FROM blueprints WHERE id=$1", e.bp); after != before {
		t.Fatalf("a refused edit changed the tree")
	}
}

// A tree edit drops restricted field values, like a save does, so an edit cannot store one.
func TestBlueprintEditDropsRestrictedFields(t *testing.T) {
	f := newFixture(t)
	e := setupEditable(t, f)
	owner := f.person("regular")
	f.share(e.source, "user", owner.id, "full")
	salary := f.call(owner, http.MethodPost, "/spaces/"+e.source+"/custom-fields", map[string]any{"name": "Salary", "type": "number"})
	f.want(salary, 201, "Salary")
	sid := salary.body["id"].(string)
	f.want(f.call(owner, http.MethodPut, "/spaces/"+e.source+"/custom-fields/"+sid+"/grants",
		map[string]any{"grants": []map[string]any{{"subject_kind": "user", "subject_id": owner.id, "level": "edit"}}}), 200, "restrict Salary")
	tree := map[string]any{"nodes": []map[string]any{{"parent": -1, "type": "project", "title": "Launch", "importance": "high", "assignee_ids": []string{},
		"fields": map[string]any{sid: 98765}}}, "dependencies": []any{}}
	f.want(f.call(e.editor, "PATCH", "/blueprints/"+e.bp, map[string]any{"version": 1, "tree": tree}), 200, "edit tree")
	if stored := f.queryID("SELECT tree::text FROM blueprints WHERE id=$1", e.bp); strings.Contains(stored, "98765") || strings.Contains(stored, sid) {
		t.Fatalf("a restricted value was stored by an edit")
	}
}

// Preview lists exactly what use creates, with the same dates, and writes nothing.
func TestBlueprintPreviewMatchesUseAndWritesNothing(t *testing.T) {
	f := newFixture(t)
	e := setupEditable(t, f)
	path := "/blueprints/" + e.bp + "/preview"
	f.want(f.call(e.reader, "POST", path, map[string]any{"start_date": "2026-06-01"}), 403, "reader cannot preview")
	f.want(f.call(e.manageOnly, "POST", path, map[string]any{"start_date": "2026-06-01"}), 403, "manage right alone cannot preview")
	f.want(f.call(e.outsider, "POST", path, map[string]any{"start_date": "2026-06-01"}), 404, "outsider is 404")
	f.want(f.call(e.useOnly, "POST", path, map[string]any{"start_date": "2026-06-01", "finish_date": "2026-06-30"}), 422, "both anchors")

	before := f.queryID("SELECT count(*)::text FROM items")
	preview := f.call(e.useOnly, "POST", path, map[string]any{"finish_date": "2026-06-30"})
	f.want(preview, 200, "preview by finish")
	if after := f.queryID("SELECT count(*)::text FROM items"); after != before {
		t.Fatalf("a preview wrote items: %s -> %s", before, after)
	}
	list, _ := preview.body["items"].([]any)
	if len(list) != 3 {
		t.Fatalf("preview lists %d items, want 3 (root, plan, ship): %v", len(list), preview.body)
	}

	// Use with the same finish date creates the same dates for the same titles.
	f.want(f.call(e.useOnly, "POST", "/blueprints/"+e.bp+"/items", map[string]any{"finish_date": "2026-06-30", "locations": []map[string]any{{"space_id": e.target}}}), 201, "use")
	for _, raw := range list {
		row := raw.(map[string]any)
		title := row["title"].(string)
		wantStart, _ := row["start_date"].(string)
		wantDue, _ := row["due_date"].(string)
		got := f.queryID("SELECT COALESCE(start_date::text,'')||'|'||COALESCE(due_date::text,'') FROM items WHERE title=$1 AND created_by=$2 AND id IN (SELECT item_id FROM item_locations WHERE space_id=$3)", title, e.useOnly.id, e.target)
		if got != wantStart+"|"+wantDue {
			t.Fatalf("%s: preview %q|%q but use created %s", title, wantStart, wantDue, got)
		}
	}
}

// The list reports the version and scope (new fields, additive).
func TestBlueprintListReportsVersion(t *testing.T) {
	f := newFixture(t)
	e := setupEditable(t, f)
	got := f.call(e.editor, "GET", "/spaces/"+e.source+"/blueprints", nil)
	f.want(got, 200, "list")
	items := got.items()
	if len(items) != 1 || items[0]["version"] != float64(1) || items[0]["scope"] != "space" {
		t.Fatalf("list row: %v", items)
	}
}
