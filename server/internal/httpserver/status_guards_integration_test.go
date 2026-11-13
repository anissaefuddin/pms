package httpserver_test

import (
	"strconv"
	"testing"
)

// WF-04: entering a guarded status needs the required fields and the right actor, the refusal lists every problem,
// and a refused change writes nothing. The same evaluator guards every PATCH path (UI and REST share it).
func TestStatusGuardsRequireFieldsAndActorAndWriteNothingOnRefusal(t *testing.T) {
	f := newFixture(t)
	admin, editor, other := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Guards")
	f.share(space, "user", editor.id, "full")
	f.share(space, "user", other.id, "full")
	wf := f.call(admin, "GET", "/workflows", nil).items()[0]
	var review, done string
	for _, st := range wf["statuses"].([]any) {
		m := st.(map[string]any)
		if m["group"] == "active" && review == "" && m["name"] != wf["statuses"].([]any)[0].(map[string]any)["name"] {
			review = m["id"].(string)
		}
		if m["group"] == "completed" && done == "" {
			done = m["id"].(string)
		}
	}
	if review == "" {
		review = f.queryID("SELECT id::text FROM statuses WHERE grp='deferred' LIMIT 1")
	}
	field := f.call(admin, "POST", "/spaces/"+space+"/custom-fields", map[string]any{"name": "Ticket", "type": "text"})
	f.want(field, 201, "field")
	fid := field.body["id"].(string)
	item := f.call(editor, "POST", "/items", map[string]any{"type": "task", "title": "Guarded", "locations": []map[string]any{{"space_id": space}}})
	f.want(item, 201, "item")
	id, version := item.body["id"].(string), 1
	patch := func(p *person, body map[string]any) reply {
		got := f.call(p, "PATCH", "/items/"+id, body, "If-Match", `"`+strconv.Itoa(version)+`"`)
		if got.status == 200 {
			version++
		}
		return got
	}
	// Configure the guard: due date, assignee and the Ticket field are required, and only an assignee (or admin) may do it.
	f.want(f.call(editor, "PATCH", "/statuses/"+done, map[string]any{"required_fields": []string{"due_date"}}), 403, "only admins configure guards")
	for name, bad := range map[string]map[string]any{
		"unknown field": {"required_fields": []string{"colour"}}, "bad rule": {"actor_rule": "nobody"},
	} {
		if got := f.call(admin, "PATCH", "/statuses/"+done, bad); got.status != 422 {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	configured := f.call(admin, "PATCH", "/statuses/"+done, map[string]any{"required_fields": []string{"due_date", "assignees", fid, "due_date"}, "actor_rule": "assignee"})
	f.want(configured, 200, "configure guard")
	var guard map[string]any
	for _, st := range configured.body["statuses"].([]any) {
		if st.(map[string]any)["id"] == done {
			guard = st.(map[string]any)
		}
	}
	if len(guard["required_fields"].([]any)) != 3 || guard["actor_rule"] != "assignee" {
		t.Fatalf("guard not stored or not deduplicated: %+v", guard)
	}
	// Refused: every unmet requirement is named, and nothing about the item changed.
	refused := patch(editor, map[string]any{"status_id": done})
	if refused.status != 422 || refused.body["code"] != "transition_blocked" || len(refused.body["errors"].([]any)) != 4 {
		t.Fatalf("expected 4 problems (date, assignee, field, actor): %+v", refused.body)
	}
	if got := f.call(editor, "GET", "/items/"+id, nil); got.header.Get("ETag") != `"`+strconv.Itoa(version)+`"` || got.body["status"].(map[string]any)["id"] == done {
		t.Fatalf("refused change was written: %+v", got.body)
	}
	// One PATCH that supplies the fields and the status together passes the guard, for an assignee.
	f.want(patch(editor, map[string]any{"assignee_ids": []string{editor.id}}), 200, "assign")
	other2 := patch(other, map[string]any{"status_id": done, "due_date": "2026-12-01", "fields": map[string]any{fid: "T-1"}})
	if other2.status != 422 || len(other2.body["errors"].([]any)) != 1 {
		t.Fatalf("a non-assignee must be refused on the actor rule alone: %+v", other2.body)
	}
	f.want(patch(editor, map[string]any{"status_id": done, "due_date": "2026-12-01", "fields": map[string]any{fid: "T-1"}}), 200, "assignee completes with everything supplied")
	// Admin passes the actor rule; leaving the status and staying in it are not guarded.
	f.want(patch(admin, map[string]any{"status_id": review}), 200, "move back")
	f.want(patch(admin, map[string]any{"title": "Renamed"}), 200, "other edits are not guarded")
	f.want(patch(admin, map[string]any{"status_id": done}), 200, "admin may move in; fields are already there")
	f.want(patch(admin, map[string]any{"status_id": done}), 200, "same status again is not a transition")

	// WF-05: a status can require every subitem to be finished first; a required relation field is a field requirement too.
	f.want(f.call(admin, "PATCH", "/statuses/"+review, map[string]any{"require_children_done": true}), 200, "children guard")
	sub := f.call(editor, "POST", "/items", map[string]any{"type": "task", "title": "Sub", "locations": []map[string]any{{"space_id": space, "parent_item_id": id}}})
	f.want(sub, 201, "subitem")
	// the parent is a task here, so build the rule on a project instead
	proj := f.call(editor, "POST", "/items", map[string]any{"type": "project", "title": "Parent", "locations": []map[string]any{{"space_id": space}}})
	f.want(proj, 201, "project")
	pid := proj.body["id"].(string)
	child := f.call(editor, "POST", "/items", map[string]any{"type": "task", "title": "Child", "locations": []map[string]any{{"space_id": space, "parent_item_id": pid}}})
	f.want(child, 201, "child")
	blocked := f.call(editor, "PATCH", "/items/"+pid, map[string]any{"status_id": review}, "If-Match", `"1"`)
	if blocked.status != 422 || blocked.body["errors"].([]any)[0].(map[string]any)["field"] != "subitems" {
		t.Fatalf("open child must block: %+v", blocked.body)
	}
	f.want(f.call(editor, "PATCH", "/items/"+child.body["id"].(string), map[string]any{"status_id": f.queryID("SELECT id::text FROM statuses WHERE grp='cancelled' LIMIT 1")}, "If-Match", `"1"`), 200, "cancel the child (needs nothing)")
	f.want(f.call(editor, "PATCH", "/items/"+pid, map[string]any{"status_id": review}, "If-Match", `"1"`), 200, "parent may move now")
}
