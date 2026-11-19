package httpserver_test

import (
	"net/http"
	"testing"
)

// Editor fields (assignee, status, custom field values, dependencies) are stored in the tree. A loop, a duplicate
// dependency, or an unknown status is refused before anything is written; a valid edit keeps every field.
func TestBlueprintEditKeepsEditorFieldsAndRefusesBadGraphs(t *testing.T) {
	f := newFixture(t)
	e := setupEditable(t, f)
	path := "/blueprints/" + e.bp
	before := f.queryID("SELECT tree::text FROM blueprints WHERE id=$1", e.bp)

	current := f.call(e.editor, "GET", path, nil)
	f.want(current, 200, "read")
	version := current.body["version"]
	tree := current.body["tree"].(map[string]any)
	nodes := tree["nodes"].([]any)
	plan := nodes[1].(map[string]any)
	ship := nodes[2].(map[string]any)
	dep := tree["dependencies"].([]any)[0].(map[string]any)
	if dep["predecessor"] != float64(1) || dep["successor"] != float64(2) {
		t.Fatalf("fixture dependency: %v", dep)
	}

	// A loop: Ship -> Plan as well as Plan -> Ship.
	loop := map[string]any{"nodes": nodes, "dependencies": []any{dep, map[string]any{"predecessor": 2, "successor": 1, "type": "FS", "lag": 0}}}
	f.want(f.call(e.editor, "PATCH", path, map[string]any{"version": version, "tree": loop}), 422, "loop refused")
	// The same dependency twice.
	twice := map[string]any{"nodes": nodes, "dependencies": []any{dep, dep}}
	f.want(f.call(e.editor, "PATCH", path, map[string]any{"version": version, "tree": twice}), 422, "duplicate refused")
	if after := f.queryID("SELECT tree::text FROM blueprints WHERE id=$1", e.bp); after != before {
		t.Fatalf("a refused graph changed the stored tree")
	}

	// A status that does not exist anywhere.
	bogus := map[string]any{"nodes": []any{nodes[0], plan, map[string]any{"parent": 0, "type": "task", "title": "Ship", "importance": "normal",
		"status_id": "99999999-9999-4999-8999-999999999999"}}, "dependencies": []any{}}
	f.want(f.call(e.editor, "PATCH", path, map[string]any{"version": version, "tree": bogus}), 422, "unknown status refused")

	// A valid edit: a real status, an assignee, a custom number field, and the dependency kept.
	progress := f.queryID("SELECT id::text FROM statuses WHERE name='In Progress' AND workflow_id=(SELECT id FROM workflows WHERE is_default)")
	owner := f.person("regular")
	f.share(e.source, "user", owner.id, "full")
	field := f.call(owner, http.MethodPost, "/spaces/"+e.source+"/custom-fields", map[string]any{"name": "Budget", "type": "number"})
	f.want(field, 201, "field")
	fid := field.body["id"].(string)
	ship["status_id"] = progress
	ship["assignee_ids"] = []string{owner.id}
	ship["fields"] = map[string]any{fid: 42}
	good := map[string]any{"nodes": []any{nodes[0], plan, ship}, "dependencies": []any{dep}}
	saved := f.call(e.editor, "PATCH", path, map[string]any{"version": version, "tree": good})
	f.want(saved, 200, "valid edit")

	read := f.call(e.editor, "GET", path, nil)
	f.want(read, 200, "read after edit")
	got := read.body["tree"].(map[string]any)["nodes"].([]any)[2].(map[string]any)
	if got["status_id"] != progress {
		t.Fatalf("status lost: %v", got["status_id"])
	}
	if assignees, _ := got["assignee_ids"].([]any); len(assignees) != 1 || assignees[0] != owner.id {
		t.Fatalf("assignee lost: %v", got["assignee_ids"])
	}
	if fields, _ := got["fields"].(map[string]any); fields[fid] != float64(42) {
		t.Fatalf("custom field value lost: %v", got["fields"])
	}
	if deps := read.body["tree"].(map[string]any)["dependencies"].([]any); len(deps) != 1 {
		t.Fatalf("dependency lost: %v", deps)
	}
}
