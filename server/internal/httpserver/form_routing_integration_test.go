package httpserver_test

import (
	"testing"

	"github.com/google/uuid"
)

// FORM-07 and FORM-09 end to end: a branch of the form decides which questions count, which item type is created,
// and which custom fields the answers fill; the server enforces it whatever the client sends.
func TestFormBranchesDecideQuestionsItemTypeAndFields(t *testing.T) {
	f := newFixture(t)
	admin, editor, requester := f.person("admin"), f.person("regular"), f.person("regular")
	intake, work, other := f.space("Intake"), f.space("Work"), f.space("Other")
	f.share(intake, "user", editor.id, "editor")
	f.share(work, "user", editor.id, "editor")
	bug := f.call(admin, "POST", "/item-types", map[string]any{"key": "bug", "name": "Bug", "base": "task"})
	idea := f.call(admin, "POST", "/item-types", map[string]any{"key": "idea", "name": "Idea", "base": "task"})
	epic := f.call(admin, "POST", "/item-types", map[string]any{"key": "epic", "name": "Epic", "base": "project"})
	f.want(bug, 201, "bug")
	f.want(idea, 201, "idea")
	f.want(epic, 201, "epic")
	sev := f.call(admin, "POST", "/spaces/"+work+"/custom-fields", map[string]any{"name": "Severity", "type": "single_select", "options": []map[string]any{
		{"id": uuid.NewString(), "label": "Low", "color": "#16A34A"}, {"id": uuid.NewString(), "label": "High", "color": "#DC2626"}}})
	ref := f.call(admin, "POST", "/spaces/"+work+"/custom-fields", map[string]any{"name": "Ref", "type": "text"})
	foreign := f.call(admin, "POST", "/spaces/"+other+"/custom-fields", map[string]any{"name": "Elsewhere", "type": "text"})
	f.want(sev, 201, "severity")
	f.want(ref, 201, "ref")
	f.want(foreign, 201, "foreign field")

	definition := map[string]any{"pages": []map[string]any{{"title": "Report", "elements": []map[string]any{
		{"key": "kind", "kind": "single_choice", "label": "Kind", "required": true, "options": []string{"Bug", "Idea"}},
		{"key": "steps", "kind": "paragraph", "label": "Steps", "required": true, "show_if": map[string]any{"key": "kind", "op": "equals", "value": "Bug"}},
		{"key": "sev", "kind": "single_choice", "label": "Severity", "options": []string{"Low", "High"}, "show_if": map[string]any{"key": "kind", "op": "equals", "value": "Bug"}},
		{"key": "ref", "kind": "single_line", "label": "Reference"},
	}}}}
	response := func(mod func(map[string]any)) map[string]any {
		r := map[string]any{"item_type": "task", "location": map[string]any{"space_id": work}, "title_question": "ref", "type_key": "idea",
			"routes":    []map[string]any{{"when": map[string]any{"key": "kind", "op": "equals", "value": "Bug"}, "type_key": "bug"}},
			"field_map": map[string]any{"sev": sev.body["id"], "ref": ref.body["id"]}}
		if mod != nil {
			mod(r)
		}
		return map[string]any{"name": "Report", "definition": definition, "response": r}
	}
	for name, mod := range map[string]func(map[string]any){
		"unknown type": func(r map[string]any) { r["type_key"] = "nope" },
		"wrong class":  func(r map[string]any) { r["type_key"] = "epic" },
		"route wrong class": func(r map[string]any) {
			r["routes"] = []map[string]any{{"when": map[string]any{"key": "kind", "op": "equals", "value": "Bug"}, "type_key": "epic"}}
		},
		"route bad answer": func(r map[string]any) {
			r["routes"] = []map[string]any{{"when": map[string]any{"key": "kind", "op": "equals", "value": "Other"}, "type_key": "bug"}}
		},
		"foreign field":    func(r map[string]any) { r["field_map"] = map[string]any{"ref": foreign.body["id"]} },
		"unknown question": func(r map[string]any) { r["field_map"] = map[string]any{"ghost": ref.body["id"]} },
	} {
		if got := f.call(editor, "POST", "/spaces/"+intake+"/forms", response(mod)); got.status != 422 {
			t.Fatalf("%s accepted: %+v", name, got)
		}
	}
	form := f.call(editor, "POST", "/spaces/"+intake+"/forms", response(nil))
	f.want(form, 201, "form")
	id := form.body["id"].(string)
	f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", id)

	// The Bug branch: its questions are required, the route picks Bug, and the answers fill the mapped fields.
	f.want(f.submit(requester, id, map[string]any{"kind": "Bug", "ref": "T-1"}), 422, "steps are required on the Bug branch")
	bugItem := f.submit(requester, id, map[string]any{"kind": "Bug", "steps": "click save", "sev": "High", "ref": "T-1"})
	f.want(bugItem, 201, "bug report")
	got := f.call(editor, "GET", "/items/"+bugItem.body["item_id"].(string), nil)
	fields := got.body["fields"].(map[string]any)
	if got.body["item_type"].(map[string]any)["key"] != "bug" || fields[ref.body["id"].(string)] != "T-1" || fields[sev.body["id"].(string)] == nil {
		t.Fatalf("bug branch: %+v", got.body)
	}
	// The Idea branch: Bug-only questions neither count nor survive, even if the client sends them anyway.
	ideaItem := f.submit(requester, id, map[string]any{"kind": "Idea", "steps": "leftover", "sev": "Low", "ref": "T-2"})
	f.want(ideaItem, 201, "idea")
	got = f.call(editor, "GET", "/items/"+ideaItem.body["item_id"].(string), nil)
	if got.body["item_type"].(map[string]any)["key"] != "idea" || got.body["fields"].(map[string]any)[sev.body["id"].(string)] != nil {
		t.Fatalf("idea branch: %+v", got.body)
	}
	if desc := string(mustJSON(got.body["description"])); contains(desc, "leftover") {
		t.Fatalf("hidden answer leaked into the item: %s", desc)
	}
	// A type deactivated later stops the form instead of creating an item of the wrong type.
	f.exec("UPDATE item_type_defs SET active=false WHERE key='bug'")
	f.want(f.submit(requester, id, map[string]any{"kind": "Bug", "steps": "x", "ref": "T-3"}), 422, "inactive type")
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
