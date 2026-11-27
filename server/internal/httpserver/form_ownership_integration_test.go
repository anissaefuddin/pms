package httpserver_test

import (
	"fmt"
	"net/http"
	"testing"
)

// B3-12: a Request Form routes only to a destination with the form's own verified owner. Assignees and approvers on a client
// destination must be active members of its Customer. An archived Customer takes no new form destination. Positive controls use
// the same form shape, so a refusal is about ownership, not the body.
func TestRequestFormDestinationRespectsOwner(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	client := f.createClient(admin, "FR-A")["id"].(string)
	editor, requester, outsider := f.person("regular"), f.person("regular"), f.person("regular")
	f.addMember(admin, client, admin)
	f.addMember(admin, client, editor)
	f.addMember(admin, client, requester)
	intake := f.scopedSpace("FR intake", "client", client)
	work := f.scopedSpace("FR work", "client", client)
	internal := f.internalSpace("FR internal")
	for _, s := range []string{intake, work, internal} {
		f.share(s, "user", admin.id, "full")
		f.share(s, "user", editor.id, "full")
		f.share(s, "user", requester.id, "editor")
	}
	progress := f.queryID("SELECT id::text FROM statuses WHERE name='In Progress' AND workflow_id=(SELECT id FROM workflows WHERE is_default)")
	definition := map[string]any{"pages": []map[string]any{{"title": "One", "elements": []map[string]any{
		{"key": "subject", "kind": "single_line", "label": "Subject", "required": true},
	}}}}
	seq := 0
	body := func(space string, assignees []string) map[string]any {
		seq++
		response := map[string]any{"item_type": "task", "status_id": progress, "location": map[string]any{"space_id": space}, "title_question": "subject"}
		if assignees != nil {
			response["assignee_ids"] = assignees
		}
		return map[string]any{"name": fmt.Sprintf("Help %d", seq), "definition": definition, "response": response}
	}
	build := func(space string, assignees []string) reply {
		return f.call(editor, "POST", "/spaces/"+intake+"/forms", body(space, assignees))
	}

	t.Run("FR-01 a same-owner destination is accepted and the submission lands there", func(t *testing.T) {
		f := f.on(t)
		created := build(work, nil)
		f.want(created, http.StatusCreated, "same-owner destination")
		formID := created.body["id"].(string)
		f.publishForm(editor, formID)
		f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", formID)
		sub := f.call(requester, "POST", "/forms/"+formID+"/submissions", map[string]any{"answers": map[string]any{"subject": "Need a desk"}})
		f.want(sub, http.StatusCreated, "member submits")
		if scope := f.queryID("SELECT i.scope_state FROM items i JOIN item_locations l ON l.item_id=i.id WHERE i.title='Need a desk' AND l.space_id=$1", work); scope != "client" {
			t.Fatalf("the submitted item must take the destination owner, got %q", scope)
		}
	})
	t.Run("FR-02 an internal destination is refused for a client-owned form", func(t *testing.T) {
		f := f.on(t)
		f.want(build(internal, nil), http.StatusUnprocessableEntity, "cross-owner destination")
	})
	t.Run("FR-03 an assignee outside the Customer is refused", func(t *testing.T) {
		f := f.on(t)
		f.want(build(work, []string{outsider.id}), http.StatusUnprocessableEntity, "non-member assignee")
	})
	t.Run("FR-04 a member assignee is accepted", func(t *testing.T) {
		f := f.on(t)
		f.want(build(work, []string{requester.id}), http.StatusCreated, "member assignee")
	})
	t.Run("FR-05 an archived Customer takes no new form destination", func(t *testing.T) {
		f := f.on(t)
		f.want(f.call(admin, "POST", "/clients/"+client+"/archive", nil), http.StatusOK, "archive Customer")
		f.want(build(work, nil), http.StatusUnprocessableEntity, "archived Customer destination")
	})
}

// B3-12: a form built before its Customer was archived must not create a resource there once archived.
func TestRequestFormSubmissionAfterArchiveWritesNothing(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	client := f.createClient(admin, "FR-ARCH")["id"].(string)
	editor, requester := f.person("regular"), f.person("regular")
	f.addMember(admin, client, admin)
	f.addMember(admin, client, editor)
	f.addMember(admin, client, requester)
	intake := f.scopedSpace("FRA intake", "client", client)
	work := f.scopedSpace("FRA work", "client", client)
	for _, s := range []string{intake, work} {
		f.share(s, "user", admin.id, "full")
		f.share(s, "user", editor.id, "full")
		f.share(s, "user", requester.id, "editor")
	}
	progress := f.queryID("SELECT id::text FROM statuses WHERE name='In Progress' AND workflow_id=(SELECT id FROM workflows WHERE is_default)")
	body := map[string]any{"name": "Pre-archive help", "definition": map[string]any{"pages": []map[string]any{{"title": "One", "elements": []map[string]any{
		{"key": "subject", "kind": "single_line", "label": "Subject", "required": true},
	}}}}, "response": map[string]any{"item_type": "task", "status_id": progress, "location": map[string]any{"space_id": work}, "title_question": "subject"}}
	created := f.call(editor, "POST", "/spaces/"+intake+"/forms", body)
	f.want(created, http.StatusCreated, "build before archive")
	formID := created.body["id"].(string)
	f.publishForm(editor, formID)
	f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", formID)
	f.want(f.call(admin, "POST", "/clients/"+client+"/archive", nil), http.StatusOK, "archive Customer")

	res := f.call(requester, "POST", "/forms/"+formID+"/submissions", map[string]any{"answers": map[string]any{"subject": "Too late"}})
	f.want(res, http.StatusConflict, "submission to an archived Customer (customer_archived)")
	if n := f.queryID("SELECT count(*)::text FROM items WHERE title='Too late'"); n != "0" {
		t.Fatalf("a refused submission created an item: %s", n)
	}
}
