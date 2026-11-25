package httpserver_test

import (
	"fmt"
	"net/http"
	"testing"
)

// Batch 4 (W1, W3): assigning someone else on a direct create needs Space Admin or Full access; the Task's allocations are saved with it.
func TestDirectCreateAssignmentRulesAndAllocations(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	member, other, planner, manager := f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Lane")
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
	space := f.space("Reassign")
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
