package httpserver_test

import (
	"net/http"
	"testing"
)

// B1.2: the status a Task is created with must be a status of the workflow it lands in.
func TestCreateTaskStatusFollowsWorkflowRules(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Board")
	f.share(space, "user", admin.id, "full")

	workflows := f.call(admin, http.MethodGet, "/workflows", nil)
	if workflows.status != 200 || len(workflows.items()) == 0 {
		t.Fatalf("workflows: %d %v", workflows.status, workflows.body)
	}
	statuses := workflows.items()[0]["statuses"].([]any)
	column := statuses[1].(map[string]any)["id"].(string)

	// A status of the same workflow is persisted as given, not replaced by the workflow's first status.
	created := f.call(admin, http.MethodPost, "/items", map[string]any{
		"type": "task", "title": "Board column", "status_id": column,
		"locations": []map[string]any{{"space_id": space}},
	})
	if created.status != 201 {
		t.Fatalf("create with a column status: %d %v", created.status, created.body)
	}
	id := created.body["id"].(string)
	if got := f.call(admin, http.MethodGet, "/items/"+id, nil); got.status != 200 || got.body["status"].(map[string]any)["id"] != column {
		t.Fatalf("status must persist after create: %d %v", got.status, got.body["status"])
	}

	// A status from another workflow is refused, and nothing is stored.
	other := f.call(admin, http.MethodPost, "/workflows", map[string]any{"name": "Other"})
	foreign := other.body["statuses"].([]any)[0].(map[string]any)["id"].(string)
	before := f.queryID("SELECT count(*)::text FROM items WHERE title='Foreign status'")
	rejected := f.call(admin, http.MethodPost, "/items", map[string]any{
		"type": "task", "title": "Foreign status", "status_id": foreign,
		"locations": []map[string]any{{"space_id": space}},
	})
	if rejected.status < 400 || rejected.status >= 500 {
		t.Fatalf("status of another workflow must be a client error, got %d %v", rejected.status, rejected.body)
	}
	if after := f.queryID("SELECT count(*)::text FROM items WHERE title='Foreign status'"); after != before {
		t.Fatalf("a refused create must not store an item: before %s after %s", before, after)
	}

	// A status that does not exist is refused too.
	missing := f.call(admin, http.MethodPost, "/items", map[string]any{
		"type": "task", "title": "Missing status", "status_id": "00000000-0000-0000-0000-000000000000",
		"locations": []map[string]any{{"space_id": space}},
	})
	if missing.status < 400 || missing.status >= 500 {
		t.Fatalf("unknown status must be a client error, got %d %v", missing.status, missing.body)
	}
}

// B1.2: a child Task inherits its parent's workflow, so a status of any other workflow is refused there.
func TestCreateChildTaskRespectsParentWorkflow(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Children")
	f.share(space, "user", admin.id, "full")

	parent := f.call(admin, http.MethodPost, "/items", map[string]any{
		"type": "project", "title": "Parent", "locations": []map[string]any{{"space_id": space}},
	})
	if parent.status != 201 {
		t.Fatalf("parent: %d %v", parent.status, parent.body)
	}
	other := f.call(admin, http.MethodPost, "/workflows", map[string]any{"name": "Child other"})
	foreign := other.body["statuses"].([]any)[0].(map[string]any)["id"].(string)
	rejected := f.call(admin, http.MethodPost, "/items", map[string]any{
		"type": "task", "title": "Child foreign", "status_id": foreign,
		"locations": []map[string]any{{"space_id": space, "parent_item_id": parent.body["id"]}},
	})
	if rejected.status < 400 || rejected.status >= 500 {
		t.Fatalf("child with a status of another workflow must be refused, got %d %v", rejected.status, rejected.body)
	}
}

// B1.4 / cross-Space: no access to the destination gives 404; view-only gives 403.
func TestCreateTaskPermissionByLocation(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	regular := f.person("regular")
	open, closed := f.space("Open"), f.space("Closed")
	f.share(open, "user", admin.id, "full")
	f.share(closed, "user", admin.id, "full")
	f.share(open, "user", regular.id, "read")

	body := func(space string) map[string]any {
		return map[string]any{"type": "task", "title": "Probe", "locations": []map[string]any{{"space_id": space}}}
	}
	if r := f.call(regular, http.MethodPost, "/items", body(open)); r.status != 403 {
		t.Fatalf("view-only create must be 403, got %d %v", r.status, r.body)
	}
	if r := f.call(regular, http.MethodPost, "/items", body(closed)); r.status != 404 {
		t.Fatalf("create in a Space without access must be 404, got %d %v", r.status, r.body)
	}
	if n := f.queryID("SELECT count(*)::text FROM items WHERE title='Probe'"); n != "0" {
		t.Fatalf("refused creates must not store items, got %s", n)
	}
}

// Characterization of B1.4 double submit at the API: the server has no idempotency key, so two identical
// requests create two items. The client prevents the double submit (busy guard); this test records the server
// contract until an Idempotency-Key is decided.
func TestCreateTaskWithoutIdempotencyKeyCreatesEachRequest(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Twice")
	f.share(space, "user", admin.id, "full")
	body := map[string]any{"type": "task", "title": "Same request", "locations": []map[string]any{{"space_id": space}}}
	first := f.call(admin, http.MethodPost, "/items", body)
	second := f.call(admin, http.MethodPost, "/items", body)
	if first.status != 201 || second.status != 201 {
		t.Fatalf("both requests: %d %d", first.status, second.status)
	}
	if n := f.queryID("SELECT count(*)::text FROM items WHERE title='Same request'"); n != "2" {
		t.Fatalf("without an idempotency key each request creates an item, got %s", n)
	}
}

// B1.3: a status that requires fields is guarded on every path that moves an item into it, create included
// (see items/guards.go). A Board column create must not skip that rule.
func TestCreateTaskIntoGuardedStatusRequiresItsFields(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Guarded")
	f.share(space, "user", admin.id, "full")

	workflows := f.call(admin, http.MethodGet, "/workflows", nil)
	column := workflows.items()[0]["statuses"].([]any)[1].(map[string]any)["id"].(string)
	f.exec("UPDATE statuses SET required_fields=ARRAY['due_date'] WHERE id=$1", column)

	without := f.call(admin, http.MethodPost, "/items", map[string]any{
		"type": "task", "title": "Guarded no date", "status_id": column,
		"locations": []map[string]any{{"space_id": space}},
	})
	if without.status < 400 || without.status >= 500 {
		t.Fatalf("create into a status that requires due_date must be refused without it, got %d %v", without.status, without.body)
	}
	if n := f.queryID("SELECT count(*)::text FROM items WHERE title='Guarded no date'"); n != "0" {
		t.Fatalf("a refused guarded create must not store an item, got %s", n)
	}

	with := f.call(admin, http.MethodPost, "/items", map[string]any{
		"type": "task", "title": "Guarded with date", "status_id": column, "due_date": "2026-12-01",
		"locations": []map[string]any{{"space_id": space}},
	})
	if with.status != 201 {
		t.Fatalf("create with the required field must succeed, got %d %v", with.status, with.body)
	}
}

// Domain rules for nested Tasks (decision: Task → Task allowed, Task → Project/Folder denied). Backend unchanged; this pins it.
func TestNestedTaskRules(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	regular := f.person("regular")
	space := f.space("Nested")
	f.share(space, "user", admin.id, "full")
	f.share(space, "user", regular.id, "read")

	create := func(p *person, kind, title, parent string) reply {
		loc := map[string]any{"space_id": space}
		if parent != "" {
			loc["parent_item_id"] = parent
		}
		return f.call(p, http.MethodPost, "/items", map[string]any{"type": kind, "title": title, "locations": []map[string]any{loc}})
	}
	project := create(admin, "project", "Project", "")
	if project.status != 201 {
		t.Fatalf("project: %d %v", project.status, project.body)
	}
	pid := project.body["id"].(string)
	task := create(admin, "task", "Task", pid)
	if task.status != 201 {
		t.Fatalf("project → task must be allowed: %d %v", task.status, task.body)
	}
	tid := task.body["id"].(string)
	nested := create(admin, "task", "Nested subtask", tid)
	if nested.status != 201 {
		t.Fatalf("task → task (nested subtask) must be allowed: %d %v", nested.status, nested.body)
	}
	if grand := create(admin, "task", "Grandchild", nested.body["id"].(string)); grand.status != 201 {
		t.Fatalf("nested subtask → task must be allowed: %d %v", grand.status, grand.body)
	}
	for _, kind := range []string{"project", "folder"} {
		if r := create(admin, kind, "Denied "+kind, tid); r.status < 400 || r.status >= 500 {
			t.Fatalf("task → %s must be refused, got %d %v", kind, r.status, r.body)
		}
	}
	if n := f.queryID("SELECT count(*)::text FROM items WHERE title LIKE 'Denied %'"); n != "0" {
		t.Fatalf("refused children must not be stored, got %s", n)
	}
	if r := create(regular, "task", "Read only nested", tid); r.status != 403 {
		t.Fatalf("view-only user must not create a nested subtask: %d %v", r.status, r.body)
	}
}
