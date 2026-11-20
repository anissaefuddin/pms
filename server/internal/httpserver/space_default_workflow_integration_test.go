package httpserver_test

import (
	"fmt"
	"net/http"
	"testing"
)

// B: Space default workflows for root Tasks and Projects. Precedence: the request, the Item Type, the Space default, then the
// Workspace default. Existing items keep their workflow and status.
func TestSpaceDefaultWorkflowsPrecedenceAndPermissions(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	reader := f.person("regular")
	space := f.space("Defaults")
	f.share(space, "user", admin.id, "full") // A1.2: the admin manages the Space's defaults as its explicit member
	f.share(space, "user", reader.id, "read")

	alpha := f.call(admin, "POST", "/workflows", map[string]any{"name": "Alpha flow"})
	f.want(alpha, 201, "alpha workflow")
	beta := f.call(admin, "POST", "/workflows", map[string]any{"name": "Beta flow"})
	f.want(beta, 201, "beta workflow")
	alphaID, betaID := alpha.body["id"].(string), beta.body["id"].(string)
	path := "/spaces/" + space + "/default-workflows"
	workflowOf := func(item string) string {
		return f.queryID("SELECT s.workflow_id::text FROM items i JOIN statuses s ON s.id=i.status_id WHERE i.id=$1", item)
	}
	makeItem := func(who *person, typ string, extra map[string]any) reply {
		body := map[string]any{"type": typ, "title": fmt.Sprintf("%s root", typ), "locations": []map[string]any{{"space_id": space}}}
		for k, v := range extra {
			body[k] = v
		}
		return f.call(who, http.MethodPost, "/items", body)
	}

	// An item created before any default exists keeps its workflow when defaults change later.
	before := makeItem(admin, "task", nil)
	f.want(before, 201, "task before defaults")
	beforeID := before.body["id"].(string)
	beforeWorkflow := workflowOf(beforeID)

	// Permissions: a reader can see the defaults, not change them.
	f.want(f.call(reader, "GET", path, nil), 200, "reader sees defaults")
	f.want(f.call(reader, "PUT", path, map[string]any{"task": alphaID}), 403, "reader cannot change defaults")

	// Validation: an unknown workflow is refused and nothing changes.
	f.want(f.call(admin, "PUT", path, map[string]any{"task": "00000000-0000-4000-8000-000000000999"}), 422, "unknown workflow refused")

	set := f.call(admin, "PUT", path, map[string]any{"task": alphaID})
	f.want(set, 200, "set task default")
	if set.body["task"] != alphaID || set.body["project"] != nil {
		t.Fatalf("defaults after set: %v", set.body)
	}

	// Space default applies to a root task with no other choice.
	task := makeItem(admin, "task", nil)
	f.want(task, 201, "root task")
	if got := workflowOf(task.body["id"].(string)); got != alphaID {
		t.Fatalf("a root task takes the Space default: got %s want %s", got, alphaID)
	}

	// The request names a workflow: it wins over the Space default.
	explicit := makeItem(admin, "task", map[string]any{"workflow_id": betaID})
	f.want(explicit, 201, "explicit workflow")
	if got := workflowOf(explicit.body["id"].(string)); got != betaID {
		t.Fatalf("an explicit workflow wins: got %s", got)
	}

	// A child takes its parent's workflow; naming one for a child is refused.
	parentID := task.body["id"].(string)
	child := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "subtask", "title": "child", "workflow_id": betaID,
		"locations": []map[string]any{{"space_id": space, "parent_item_id": parentID}}})
	f.want(child, 422, "a child cannot name a workflow")

	// Projects use their own default, independent of tasks.
	f.want(f.call(admin, "PUT", path, map[string]any{"task": alphaID, "project": betaID}), 200, "set project default")
	project := makeItem(admin, "project", nil)
	f.want(project, 201, "root project")
	if got := workflowOf(project.body["id"].(string)); got != betaID {
		t.Fatalf("a root project takes the Space project default: got %s", got)
	}

	// Existing items are unchanged by a later default.
	f.want(f.call(admin, "PUT", path, map[string]any{"task": betaID}), 200, "change task default")
	if got := workflowOf(beforeID); got != beforeWorkflow {
		t.Fatalf("an existing item keeps its workflow: got %s want %s", got, beforeWorkflow)
	}
}
