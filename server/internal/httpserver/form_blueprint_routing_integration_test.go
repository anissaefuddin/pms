package httpserver_test

import "testing"

// FORM-07 + blueprint routing (E2E-PAR-05): the branch the answers take decides the item type and the blueprint
// applied to the new item; questions outside the branch are not stored; blueprints of other Spaces are refused.
func TestFormBranchingBlueprintItem(t *testing.T) {
	f := newFixture(t)
	admin, editor, requester := f.person("admin"), f.person("regular"), f.person("regular")
	intake, work, other := f.space("Intake"), f.space("Work"), f.space("Other")
	f.share(intake, "user", editor.id, "editor")
	f.share(work, "user", editor.id, "editor")
	f.want(f.call(admin, "POST", "/item-types", map[string]any{"key": "bug", "name": "Bug", "base": "task"}), 201, "bug type")
	f.want(f.call(admin, "POST", "/item-types", map[string]any{"key": "idea", "name": "Idea", "base": "task"}), 201, "idea type")

	blueprint := func(space, name string, importance string, children ...string) string {
		root := f.call(admin, "POST", "/items", map[string]any{"type": "task", "title": name, "importance": importance,
			"assignee_ids": []string{editor.id}, "locations": []map[string]any{{"space_id": space}}})
		f.want(root, 201, "root "+name)
		for _, c := range children {
			f.want(f.call(admin, "POST", "/items", map[string]any{"type": "task", "title": c,
				"locations": []map[string]any{{"space_id": space, "parent_item_id": root.body["id"]}}}), 201, "child "+c)
		}
		saved := f.call(admin, "POST", "/spaces/"+space+"/blueprints", map[string]any{"name": name, "source_item_id": root.body["id"]})
		f.want(saved, 201, "save "+name)
		return saved.body["id"].(string)
	}
	bpBug := blueprint(work, "Bug triage", "low", "Reproduce", "Fix")
	bpIdea := blueprint(work, "Idea review", "low", "Evaluate")
	bpForeign := blueprint(other, "Foreign", "low", "Leak")

	definition := map[string]any{"pages": []map[string]any{{"title": "Report", "elements": []map[string]any{
		{"key": "kind", "kind": "single_choice", "label": "Kind", "required": true, "options": []string{"Bug", "Idea"}},
		{"key": "steps", "kind": "paragraph", "label": "Steps", "required": true, "show_if": map[string]any{"key": "kind", "op": "equals", "value": "Bug"}},
		{"key": "name", "kind": "single_line", "label": "Name"},
		{"key": "urgency", "kind": "importance", "label": "Urgency"},
	}}}}
	route := func(answer, typeKey string, bp any) map[string]any {
		return map[string]any{"when": map[string]any{"key": "kind", "op": "equals", "value": answer}, "type_key": typeKey, "blueprint_id": bp}
	}
	response := func(routes ...map[string]any) map[string]any {
		return map[string]any{"name": "Report", "definition": definition, "response": map[string]any{
			"item_type": "task", "location": map[string]any{"space_id": work}, "title_question": "name", "routes": routes}}
	}
	f.want(f.call(editor, "POST", "/spaces/"+intake+"/forms", response(route("Bug", "bug", bpForeign))), 422, "blueprint of another Space")
	f.want(f.call(editor, "POST", "/spaces/"+intake+"/forms", response(route("Bug", "bug", "00000000-0000-0000-0000-000000000001"))), 422, "unknown blueprint")
	form := f.call(editor, "POST", "/spaces/"+intake+"/forms", response(route("Bug", "bug", bpBug), route("Idea", "idea", bpIdea)))
	f.want(form, 201, "form")
	id := form.body["id"].(string)
	f.publishForm(editor, id)
	f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", id)

	children := func(item string) string {
		return f.queryID(`SELECT coalesce(string_agg(i.title, ',' ORDER BY i.title),'') FROM item_locations l JOIN items i ON i.id=l.item_id WHERE l.parent_item_id=$1`, item)
	}
	f.want(f.submit(requester, id, map[string]any{"kind": "Bug"}), 422, "required question of the branch")
	bug := f.submit(requester, id, map[string]any{"kind": "Bug", "steps": "click", "name": "B-1", "urgency": "high"})
	f.want(bug, 201, "bug branch")
	got := f.call(editor, "GET", "/items/"+bug.body["item_id"].(string), nil)
	if got.body["item_type"].(map[string]any)["key"] != "bug" || got.body["title"] != "B-1" {
		t.Fatalf("bug item: %+v", got.body)
	}
	if got.body["importance"] != "high" { // the answer wins over the blueprint's "low"
		t.Fatalf("form answer must win over the blueprint default: %v", got.body["importance"])
	}
	if c := children(bug.body["item_id"].(string)); c != "Fix,Reproduce" {
		t.Fatalf("bug children = %q", c)
	}
	idea := f.submit(requester, id, map[string]any{"kind": "Idea", "steps": "leftover", "name": "I-1"})
	f.want(idea, 201, "idea branch")
	got = f.call(editor, "GET", "/items/"+idea.body["item_id"].(string), nil)
	if got.body["item_type"].(map[string]any)["key"] != "idea" || got.body["importance"] != "low" {
		t.Fatalf("idea item (blueprint default applies when unanswered): %+v", got.body)
	}
	if c := children(idea.body["item_id"].(string)); c != "Evaluate" {
		t.Fatalf("idea children = %q", c)
	}
	if desc := string(mustJSON(got.body["description"])); contains(desc, "leftover") {
		t.Fatalf("hidden answer leaked: %s", desc)
	}
	// A deleted blueprint is skipped: the request still creates its item, without the structure, and no 5xx.
	f.want(f.call(admin, "DELETE", "/blueprints/"+bpIdea, nil), 204, "delete blueprint")
	again := f.submit(requester, id, map[string]any{"kind": "Idea", "name": "I-2"})
	f.want(again, 201, "deleted blueprint")
	if c := children(again.body["item_id"].(string)); c != "" {
		t.Fatalf("children after deleted blueprint = %q", c)
	}
}
