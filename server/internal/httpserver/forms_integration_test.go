package httpserver_test

import (
	"net/http"
	"testing"
)

func formBody(space string, mod func(map[string]any)) map[string]any {
	b := map[string]any{
		"name": "IT request", "description": "Ask IT",
		"definition": map[string]any{"pages": []map[string]any{{"title": "Details", "elements": []map[string]any{
			{"key": "subject", "kind": "single_line", "label": "Subject", "required": true},
			{"key": "area", "kind": "single_choice", "label": "Area", "options": []string{"Network", "Laptop"}},
			{"key": "extra", "kind": "section", "label": "More"},
		}}}},
		"response": map[string]any{"item_type": "task", "location": map[string]any{"space_id": space}, "title_question": "subject"},
	}
	if mod != nil {
		mod(b)
	}
	return b
}

func TestFormsDefinitionAndSettings(t *testing.T) {
	f := newFixture(t)
	admin, editor, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	space, readOnly := f.space("Intake"), f.space("Archive")
	f.share(space, "user", editor.id, "editor")
	f.share(space, "user", reader.id, "read")
	f.share(readOnly, "user", editor.id, "read")
	path := "/spaces/" + space + "/forms"

	f.want(f.call(reader, "POST", path, formBody(space, nil)), 403, "read-only member cannot build forms")
	f.want(f.call(reader, "GET", path, nil), 403, "read-only member cannot list forms")
	f.want(f.call(outsider, "POST", path, formBody(space, nil)), 404, "no access is 404")

	// Validation: each case must be refused with a field error, and nothing may be stored.
	personal := f.queryID("SELECT id::text FROM spaces WHERE kind='personal' AND owner_id=$1", editor.id)
	missingParent := "00000000-0000-0000-0000-000000000001"
	for name, mod := range map[string]func(map[string]any){
		"blank name": func(b map[string]any) { b["name"] = " " },
		"no questions": func(b map[string]any) {
			b["definition"] = map[string]any{"pages": []map[string]any{{"title": "x", "elements": []map[string]any{}}}}
		},
		"unknown title key": func(b map[string]any) { b["response"].(map[string]any)["title_question"] = "nope" },
		"destination read-only": func(b map[string]any) {
			b["response"].(map[string]any)["location"] = map[string]any{"space_id": readOnly}
		},
		"destination personal": func(b map[string]any) {
			b["response"].(map[string]any)["location"] = map[string]any{"space_id": personal}
		},
		"destination missing parent": func(b map[string]any) {
			b["response"].(map[string]any)["location"] = map[string]any{"space_id": space, "parent_item_id": missingParent}
		},
		"unknown status":   func(b map[string]any) { b["response"].(map[string]any)["status_id"] = missingParent },
		"unknown assignee": func(b map[string]any) { b["response"].(map[string]any)["assignee_ids"] = []string{missingParent} },
		"unknown approver": func(b map[string]any) {
			b["response"].(map[string]any)["approval"] = map[string]any{"approver_ids": []string{missingParent}}
		},
	} {
		r := f.call(editor, "POST", path, formBody(space, mod))
		if r.status != 422 || r.body["errors"] == nil {
			t.Fatalf("%s: want 422 with a field error, got %d %v", name, r.status, r.body)
		}
	}
	if r := f.call(editor, "POST", path, formBody(space, func(b map[string]any) { b["response"].(map[string]any)["bogus"] = 1 })); r.status != 400 {
		t.Fatalf("unknown fields are rejected: %d", r.status)
	}
	if n := f.queryID("SELECT count(*)::text FROM request_forms"); n != "0" {
		t.Fatalf("refused forms must not be stored, got %s", n)
	}

	// A status from another workflow is not valid for this destination.
	other := f.call(admin, "POST", "/workflows", map[string]any{"name": "Other"})
	foreign := other.body["statuses"].([]any)[0].(map[string]any)["id"].(string)
	f.want(f.call(editor, "POST", path, formBody(space, func(b map[string]any) { b["response"].(map[string]any)["status_id"] = foreign })), 422, "status of another workflow")

	created := f.call(editor, "POST", path, formBody(space, func(b map[string]any) {
		b["response"].(map[string]any)["status_id"] = f.queryID("SELECT id::text FROM statuses WHERE name='In Progress' AND workflow_id=(SELECT id FROM workflows WHERE is_default)")
		b["response"].(map[string]any)["assignee_ids"] = []string{editor.id}
		b["response"].(map[string]any)["approval"] = map[string]any{"approver_ids": []string{admin.id}, "description": "Check"}
	}))
	if created.status != 201 || created.header.Get("ETag") != `"1"` || created.body["version"] != float64(1) {
		t.Fatalf("create: %d %v", created.status, created.body)
	}
	id := created.body["id"].(string)
	resp := created.body["response"].(map[string]any)
	if resp["title_question"] != "subject" || len(resp["assignee_ids"].([]any)) != 1 || resp["approval"] == nil {
		t.Fatalf("response settings must round-trip: %v", resp)
	}
	f.want(f.call(editor, "POST", path, formBody(space, func(b map[string]any) { b["name"] = "it REQUEST" })), 409, "duplicate name ignores case")

	list := f.call(editor, "GET", path, nil)
	if list.status != 200 || len(list.items()) != 1 || list.items()[0]["question_count"] != float64(2) {
		t.Fatalf("list counts questions, not sections: %v", list.body)
	}
	f.want(f.call(reader, "GET", "/forms/"+id, nil), 403, "reader get")
	f.want(f.call(outsider, "GET", "/forms/"+id, nil), 404, "outsider get")
	if got := f.call(admin, "GET", "/forms/"+id, nil); got.status != 200 || got.body["name"] != "IT request" {
		t.Fatalf("admin reads any form: %d %v", got.status, got.body)
	}

	// Replace needs the current version.
	update := formBody(space, func(b map[string]any) { b["name"] = "IT request v2" })
	f.want(f.call(editor, "PUT", "/forms/"+id, update), 428, "missing If-Match")
	f.want(f.call(editor, "PUT", "/forms/"+id, update, "If-Match", `"7"`), 409, "stale version")
	f.want(f.call(reader, "PUT", "/forms/"+id, update, "If-Match", `"1"`), 403, "reader replace")
	saved := f.call(editor, "PUT", "/forms/"+id, update, "If-Match", `"1"`)
	if saved.status != 200 || saved.header.Get("ETag") != `"2"` || saved.body["name"] != "IT request v2" {
		t.Fatalf("replace: %d %v", saved.status, saved.body)
	}

	f.want(f.call(reader, "DELETE", "/forms/"+id, nil), 403, "reader delete")
	f.want(f.call(editor, "DELETE", "/forms/"+id, nil), http.StatusNoContent, "delete")
	f.want(f.call(editor, "GET", "/forms/"+id, nil), 404, "deleted form")
}

func TestFormAccessAndManageList(t *testing.T) {
	f := newFixture(t)
	admin, editor, owner, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular")
	intake, other := f.space("Intake"), f.space("Other")
	f.share(intake, "user", editor.id, "editor")
	f.share(intake, "user", owner.id, "full")
	f.share(intake, "user", reader.id, "read")
	made := f.call(editor, "POST", "/spaces/"+intake+"/forms", formBody(intake, nil))
	f.want(made, 201, "editor builds a form")
	f.publishForm(editor, made.body["id"].(string))
	if made.body["access"] != "private" {
		t.Fatalf("new forms start private: %v", made.body["access"])
	}
	f.want(f.call(admin, "POST", "/spaces/"+other+"/forms", formBody(other, func(b map[string]any) { b["name"] = "Other form" })), 201, "form in another Space")
	id := made.body["id"].(string)

	// Publishing needs the share capability, not just edit.
	f.want(f.call(editor, "PUT", "/forms/"+id+"/access", map[string]any{"access": "internal"}), 403, "editor cannot publish")
	f.want(f.call(reader, "PUT", "/forms/"+id+"/access", map[string]any{"access": "internal"}), 403, "reader cannot publish")
	f.want(f.call(outsider, "PUT", "/forms/"+id+"/access", map[string]any{"access": "internal"}), 404, "outsider")
	f.want(f.call(owner, "PUT", "/forms/"+id+"/access", map[string]any{"access": "public"}), 422, "unsupported access")
	f.want(f.call(owner, "PUT", "/forms/"+id+"/access", map[string]any{"access": "internal", "x": 1}), 400, "unknown field")
	on := f.call(owner, "PUT", "/forms/"+id+"/access", map[string]any{"access": "internal"})
	if on.status != 200 || on.body["access"] != "internal" {
		t.Fatalf("publish: %d %v", on.status, on.body)
	}
	if got := f.call(outsider, "GET", "/forms/available", nil); len(got.items()) != 1 {
		t.Fatalf("a published form is offered to everyone: %v", got.body)
	}
	f.want(f.call(owner, "PUT", "/forms/"+id+"/access", map[string]any{"access": "private"}), 200, "unpublish")
	if got := f.call(outsider, "GET", "/forms/available", nil); len(got.items()) != 0 {
		t.Fatalf("an unpublished form disappears: %v", got.body)
	}
	f.want(f.call(owner, "PUT", "/forms/"+"00000000-0000-0000-0000-000000000000"+"/access", map[string]any{"access": "internal"}), 404, "missing form")

	// Manage forms lists what the actor can edit, across Spaces.
	mine := f.call(editor, "GET", "/forms", nil)
	if mine.status != 200 || len(mine.items()) != 1 || mine.items()[0]["space_name"] != "Intake" || mine.items()[0]["access"] != "private" {
		t.Fatalf("editor manages one form: %v", mine.body)
	}
	if all := f.call(admin, "GET", "/forms", nil); len(all.items()) != 2 || all.items()[0]["space_name"] != "Intake" {
		t.Fatalf("admin manages every form, ordered by Space then name: %v", all.body)
	}
	for name, p := range map[string]*person{"reader": reader, "outsider": outsider} {
		if got := f.call(p, "GET", "/forms", nil); got.status != 200 || len(got.items()) != 0 {
			t.Fatalf("%s manages nothing: %v", name, got.body)
		}
	}
}

// publishForm makes a draft live. New forms start as drafts, so tests that let requesters in publish first.
func (f *fixture) publishForm(p *person, id string) {
	f.t.Helper()
	f.want(f.call(p, "POST", "/forms/"+id+"/publish", nil), 200, "publish form")
}
