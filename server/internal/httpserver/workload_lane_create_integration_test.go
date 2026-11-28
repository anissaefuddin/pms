package httpserver_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// Batch 4 (W1, W3): assigning someone else on a direct create needs Space Admin or Full access; the Task's allocations are saved with it.
func TestDirectCreateAssignmentRulesAndAllocations(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	member, other, planner, manager := f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.internalSpace("Lane")
	f.share(space, "user", member.id, "editor")
	f.share(space, "user", planner.id, "editor")
	f.share(space, "user", other.id, "read")
	f.share(space, "user", manager.id, "full")
	f.exec("INSERT INTO space_admins(space_id,user_id) VALUES ($1,$2)", space, planner.id)

	body := func(title string, assignee string) map[string]any {
		b := map[string]any{"type": "task", "title": title, "locations": []map[string]string{{"space_id": space}}}
		if assignee != "" {
			b["assignee_ids"] = []string{assignee}
		}
		return b
	}

	f.want(f.call(member, http.MethodPost, "/items", body("Other lane", other.id)), http.StatusForbidden, "member cannot assign someone else")
	f.want(f.call(member, http.MethodPost, "/items", body("Own lane", member.id)), http.StatusCreated, "member can assign self")
	f.want(f.call(planner, http.MethodPost, "/items", body("Planned lane", other.id)), http.StatusCreated, "space admin can assign someone else")
	f.want(f.call(manager, http.MethodPost, "/items", body("Managed lane", other.id)), http.StatusCreated, "Full access can assign someone else (DEF-006)")
	personal := f.queryID("SELECT id::text FROM spaces WHERE kind='personal' AND owner_id=$1", manager.id)
	inPersonal := body("Private lane", other.id)
	inPersonal["locations"] = []map[string]string{{"space_id": personal}}
	f.want(f.call(manager, http.MethodPost, "/items", inPersonal), http.StatusForbidden, "nobody else is assigned in a personal Space")
	f.want(f.call(admin, http.MethodPost, "/items", body("Admin lane", other.id)), http.StatusCreated, "workspace admin can assign someone else")

	// W1: allocations are created with the Task, atomically; no resource booking is created.
	alloc := map[string]any{
		"type": "task", "title": "Allocated lane", "locations": []map[string]string{{"space_id": space}}, "assignee_ids": []string{member.id},
		"assignee_allocations": []map[string]any{{"assignee_id": member.id, "allocation": map[string]any{"unit": "minutes_per_day", "value": 60}}},
	}
	made := f.call(planner, http.MethodPost, "/items", alloc)
	f.want(made, http.StatusCreated, "allocation create by space admin")
	id := made.body["id"].(string)
	var unit string
	var value int
	if err := f.pool.QueryRow(f.ctx, "SELECT allocation_unit, allocation_value FROM item_assignees WHERE item_id=$1 AND user_id=$2", id, member.id).Scan(&unit, &value); err != nil {
		t.Fatal(err)
	}
	if unit != "minutes_per_day" || value != 60 {
		t.Fatalf("saved allocation = %q %d, want minutes_per_day 60", unit, value)
	}
	if n := f.queryID("SELECT count(*)::text FROM resource_bookings WHERE project_id=$1", id); n != "0" {
		t.Fatalf("resource bookings created = %s, want 0", n)
	}
}

// Owner decision (2026-11-26): an Editor cannot assign someone else on an edit either, in any path; taking a task or leaving it
// stays possible, and people already assigned are untouched by the Editor's other edits.
func TestEditorCannotAssignOthersOnEdit(t *testing.T) {
	f := newFixture(t)
	manager, editor, other := f.person("regular"), f.person("regular"), f.person("regular")
	space := f.internalSpace("Reassign")
	f.share(space, "user", manager.id, "full")
	f.share(space, "user", editor.id, "editor")
	f.share(space, "user", other.id, "editor")
	made := f.call(manager, "POST", "/items", map[string]any{"type": "task", "title": "T", "assignee_ids": []string{manager.id}, "locations": []map[string]string{{"space_id": space}}})
	f.want(made, http.StatusCreated, "task")
	path, version := "/items/"+made.body["id"].(string), 1
	patch := func(p *person, want int, what string, fields map[string]any) {
		t.Helper()
		f.want(f.call(p, "PATCH", path, fields, "If-Match", fmt.Sprintf(`"%d"`, version)), want, what)
		if want == http.StatusOK {
			version++
		}
	}
	patch(editor, http.StatusForbidden, "editor adds someone else", map[string]any{"assignee_ids": []string{manager.id, other.id}})
	patch(editor, http.StatusOK, "editor takes the task, the manager stays", map[string]any{"assignee_ids": []string{manager.id, editor.id}})
	patch(editor, http.StatusOK, "editor edits another field with the same people", map[string]any{"title": "T2", "assignee_ids": []string{manager.id, editor.id}})
	patch(editor, http.StatusOK, "editor leaves the task", map[string]any{"assignee_ids": []string{manager.id}})
	patch(manager, http.StatusOK, "Full access assigns someone else", map[string]any{"assignee_ids": []string{other.id}})
}

// "Project Manager" is Full access to a Space. It never reaches the Workspace configuration.
func TestFullSpaceAccessIsNotWorkspaceConfiguration(t *testing.T) {
	f := newFixture(t)
	manager := f.person("regular")
	space := f.space("Managed")
	f.share(space, "user", manager.id, "full")
	f.exec("INSERT INTO space_admins(space_id,user_id) VALUES ($1,$2)", space, manager.id)
	for _, c := range []struct {
		method, path string
		body         map[string]any
	}{
		{"POST", "/custom-fields", map[string]any{"name": "W", "type": "text"}},
		{"POST", "/item-types", map[string]any{"key": "ws_type", "name": "W", "base": "task"}},
		{"POST", "/workflows", map[string]any{"name": "W"}},
		{"POST", "/groups", map[string]any{"name": "W"}},
		{"POST", "/rates", map[string]any{}},
		{"PUT", "/settings/work-schedule", map[string]any{"working_days": []int{1}}},
		{"PUT", "/automation/limits", map[string]any{"rule_per_hour": 1, "workspace_per_hour": 1, "space_per_hour": 1}},
		{"POST", "/automation/workspace-rules", map[string]any{"name": "W"}},
		{"POST", "/custom-roles", map[string]any{"name": "W"}},
		{"GET", "/audit-events", nil},
		{"POST", "/connectors", map[string]any{}},
	} {
		got := f.call(manager, c.method, c.path, c.body)
		if got.status != http.StatusForbidden && got.status != http.StatusNotFound && got.status != http.StatusMethodNotAllowed {
			t.Errorf("%s %s as Full + Space Admin: %d %v", c.method, c.path, got.status, got.body)
		}
	}
	f.want(f.call(manager, "POST", "/spaces/"+space+"/custom-fields", map[string]any{"name": "S", "type": "text"}), http.StatusCreated, "the Space's own configuration stays available")
}

// Owner decision (2026-11-26): a blueprint or a request form is not a way around it. An Editor gets the structure and routing
// without other people on it; Full access and people who do not work in the Space keep the preset team.
func TestEditorCannotAssignThroughBlueprintOrForm(t *testing.T) {
	f := newFixture(t)
	manager, editor, teammate, requester := f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.internalSpace("Intake team")
	f.share(space, "user", manager.id, "full")
	f.share(space, "user", editor.id, "editor")
	f.share(space, "user", teammate.id, "editor")
	here := []map[string]string{{"space_id": space}}
	assignees := func(item any) string {
		return f.queryID("SELECT COALESCE(string_agg(user_id::text, ',' ORDER BY user_id), '') FROM item_assignees WHERE item_id=$1::uuid", item)
	}

	source := f.call(manager, "POST", "/items", map[string]any{"type": "project", "title": "Kit", "locations": here})
	f.want(source, 201, "source")
	child := f.call(manager, "POST", "/items", map[string]any{"type": "task", "title": "Step", "assignee_ids": []string{teammate.id, editor.id},
		"locations": []map[string]any{{"space_id": space, "parent_item_id": source.body["id"]}}})
	f.want(child, 201, "assigned step")
	saved := f.call(manager, "POST", "/spaces/"+space+"/blueprints", map[string]any{"name": "Kit", "source_item_id": source.body["id"]})
	f.want(saved, 201, "blueprint")
	stepOf := func(p *person) string {
		made := f.call(p, "POST", "/blueprints/"+saved.body["id"].(string)+"/items", map[string]any{"locations": here})
		f.want(made, 201, "use blueprint")
		return assignees(f.queryID("SELECT item_id::text FROM item_locations WHERE parent_item_id=$1::uuid", made.body["id"]))
	}
	if got := stepOf(editor); got != editor.id {
		t.Fatalf("an Editor keeps only themself: %q", got)
	}
	if got := stepOf(manager); !strings.Contains(got, teammate.id) || !strings.Contains(got, editor.id) {
		t.Fatalf("Full access keeps the blueprint's team: %q", got)
	}

	form := f.call(editor, "POST", "/spaces/"+space+"/forms", map[string]any{"name": "Ask", "definition": map[string]any{"pages": []map[string]any{{"title": "One", "elements": []map[string]any{
		{"key": "who", "kind": "assignee", "label": "Who"}}}}}, "response": map[string]any{"item_type": "task", "location": map[string]any{"space_id": space}, "assignee_ids": []string{teammate.id}}})
	f.want(form, 201, "an Editor builds a form")
	id := form.body["id"].(string)
	f.publishForm(editor, id)
	f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", id)
	submit := func(p *person) string {
		made := f.submit(p, id, map[string]any{"who": editor.id})
		f.want(made, 201, "submit")
		return assignees(made.body["item_id"])
	}
	if got := submit(editor); got != editor.id {
		t.Fatalf("an Editor cannot assign someone else with a form: %q", got)
	}
	if got := submit(teammate); got != teammate.id {
		t.Fatalf("another Editor keeps only their own preset, not the person they answered: %q", got)
	}
	for name, p := range map[string]*person{"Full access": manager, "a requester outside the Space": requester} {
		if got := submit(p); !strings.Contains(got, teammate.id) || !strings.Contains(got, editor.id) {
			t.Fatalf("%s keeps the form's routing: %q", name, got)
		}
	}
}
