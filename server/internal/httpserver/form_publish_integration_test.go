package httpserver_test

import "testing"

// RF-06: a new form is a draft. Only people who manage it can see or preview it; requesters get it after publishing.
func TestFormDraftPublishLifecycle(t *testing.T) {
	f := newFixture(t)
	editor, requester := f.person("regular"), f.person("regular")
	intake := f.space("Intake")
	f.share(intake, "user", editor.id, "editor")
	path := "/spaces/" + intake + "/forms"

	made := f.call(editor, "POST", path, formBody(intake, nil))
	f.want(made, 201, "editor builds a form")
	if made.body["status"] != "draft" {
		t.Fatalf("a new form starts as a draft: %v", made.body["status"])
	}
	id := made.body["id"].(string)
	f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", id)

	// Draft: the manager previews it; a requester sees nothing and cannot submit.
	f.want(f.call(editor, "GET", "/forms/"+id+"/definition", nil), 200, "manager previews the draft")
	f.want(f.call(requester, "GET", "/forms/"+id+"/definition", nil), 404, "requester cannot open a draft")
	f.want(f.submit(requester, id, map[string]any{"subject": "Too early"}), 404, "requester cannot submit a draft")
	f.want(f.submit(editor, id, map[string]any{"subject": "Preview"}), 409, "nobody submits a draft")
	f.want(f.call(editor, "GET", "/forms/available", nil), 200, "available list loads")
	if offered := f.call(requester, "GET", "/forms/available", nil); len(offered.body["items"].([]any)) != 0 {
		t.Fatalf("a draft is not offered: %v", offered.body)
	}

	// Only managers publish; a requester is refused.
	f.want(f.call(requester, "POST", "/forms/"+id+"/publish", nil), 404, "requester cannot publish")

	published := f.call(editor, "POST", "/forms/"+id+"/publish", nil)
	f.want(published, 200, "manager publishes")
	if published.body["status"] != "published" {
		t.Fatalf("published form reports its status: %v", published.body["status"])
	}
	f.want(f.call(requester, "GET", "/forms/"+id+"/definition", nil), 200, "requester opens the published form")
	f.want(f.submit(requester, id, map[string]any{"subject": "Laptop"}), 201, "requester submits the published form")

	// Unpublishing takes it back from requesters; existing submissions stay.
	f.want(f.call(editor, "POST", "/forms/"+id+"/unpublish", nil), 200, "manager unpublishes")
	f.want(f.call(requester, "GET", "/forms/"+id+"/definition", nil), 404, "requester cannot open it again")

	// A form needs at least one question to be saved at all, so an empty form can never reach publish.
	empty := f.call(editor, "POST", path, formBody(intake, func(b map[string]any) {
		b["name"] = "Only a heading"
		b["definition"] = map[string]any{"pages": []map[string]any{{"title": "Intro", "elements": []map[string]any{{"key": "intro", "kind": "section", "label": "Intro"}}}}}
	}))
	f.want(empty, 422, "a form with only a heading is refused")
}
