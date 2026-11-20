package httpserver_test

import (
	"net/http"
	"testing"
)

// R1 (Batch A): a form submission runs as the requester inside the form's own Space only. Negative tests: the request
// cannot choose another destination, the requester gains no rights there, and the audit trail names the requester.

type scopeSetup struct {
	admin, editor, requester *person
	intake, work, other      string
	formID, token            string
}

func setupScopedForm(t *testing.T, f *fixture) scopeSetup {
	t.Helper()
	s := scopeSetup{admin: f.person("admin"), editor: f.person("regular"), requester: f.person("regular")}
	s.intake, s.work, s.other = f.space("Intake"), f.space("Work"), f.space("Other")
	f.share(s.intake, "user", s.editor.id, "editor")
	f.share(s.work, "user", s.editor.id, "editor")
	f.share(s.other, "user", s.editor.id, "full")
	f.share(s.intake, "user", s.admin.id, "full") // A1.2: the admin builds and links the form in intake, with destination work
	f.share(s.work, "user", s.admin.id, "full")
	progress := f.queryID("SELECT id::text FROM statuses WHERE name='In Progress' AND workflow_id=(SELECT id FROM workflows WHERE is_default)")
	body := map[string]any{"name": "Scoped help", "definition": map[string]any{"pages": []map[string]any{{"title": "One", "elements": []map[string]any{
		{"key": "subject", "kind": "single_line", "label": "Subject", "required": true},
	}}}}, "response": map[string]any{"item_type": "task", "status_id": progress, "location": map[string]any{"space_id": s.work}, "title_question": "subject"}}
	created := f.call(s.editor, "POST", "/spaces/"+s.intake+"/forms", body)
	f.want(created, 201, "build form")
	s.formID = created.body["id"].(string)
	f.publishForm(s.editor, s.formID)
	f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", s.formID)
	return s
}

// Tampering: extra keys name another destination, a type, and a parent. None of them may move the item.
func TestMemberSubmissionCannotChooseAnotherDestination(t *testing.T) {
	f := newFixture(t)
	s := setupScopedForm(t, f)
	tampered := map[string]any{"subject": "Tampered", "space_id": s.other, "locations": []map[string]any{{"space_id": s.other}},
		"type_key": "task", "parent_item_id": s.other, "owner_id": s.editor.id}
	got := f.call(s.requester, "POST", "/forms/"+s.formID+"/submissions", map[string]any{"answers": tampered, "locations": []map[string]any{{"space_id": s.other}}})
	if got.status != http.StatusCreated && got.status != http.StatusUnprocessableEntity && got.status != http.StatusBadRequest {
		t.Fatalf("tampered submission: status %d", got.status)
	}
	if n := f.queryID("SELECT count(*)::text FROM items i JOIN item_locations l ON l.item_id=i.id WHERE l.space_id=$1 AND i.title='Tampered'", s.other); n != "0" {
		t.Fatalf("a submission reached another Space: %s items", n)
	}
	if n := f.queryID("SELECT count(*)::text FROM item_locations WHERE space_id=$1 AND deleted_root_id IS NULL AND item_id IN (SELECT id FROM items WHERE title='Tampered')", s.work); n != "0" && n != "1" {
		t.Fatalf("unexpected placement count in the form Space: %s", n)
	}
}

// No escalation: the requester keeps no rights in the destination after a successful submission.
func TestMemberSubmissionGivesRequesterNoRightsInDestination(t *testing.T) {
	f := newFixture(t)
	s := setupScopedForm(t, f)
	f.want(f.submit(s.requester, s.formID, map[string]any{"subject": "Printer jammed"}), 201, "member submits")
	f.want(f.call(s.requester, "GET", "/spaces/"+s.work+"/items", nil), 404, "requester cannot list the destination")
	f.want(f.call(s.requester, "GET", "/spaces/"+s.other+"/items", nil), 404, "requester cannot see another Space")
	f.want(f.call(s.requester, "POST", "/items", map[string]any{"type": "task", "title": "Direct", "locations": []map[string]any{{"space_id": s.work}}}), 404, "requester cannot create directly")
	if n := f.queryID("SELECT count(*)::text FROM shares WHERE principal_id=$1 AND target_id=$2", s.requester.id, s.work); n != "0" {
		t.Fatalf("a submission granted the requester a share in the destination")
	}
}

// Audit: a member's submission names the member; a public one names the anonymous placeholder and the visitor's email.
func TestSubmissionAuditNamesTheActor(t *testing.T) {
	f := newFixture(t)
	s := setupScopedForm(t, f)
	f.want(f.submit(s.requester, s.formID, map[string]any{"subject": "Member audit"}), 201, "member submits")
	if n := f.queryID("SELECT count(*)::text FROM audit_events WHERE action='form.submit' AND actor_id=$1::uuid AND details->>'via'='member' AND details->>'form_id'=$2", s.requester.id, s.formID); n != "1" {
		t.Fatalf("member submission audit rows = %s", n)
	}
	link := f.call(s.admin, "POST", "/forms/"+s.formID+"/public-link", nil)
	f.want(link, 200, "create public link")
	token, _ := link.body["public_token"].(string)
	payload := map[string]any{"answers": map[string]any{"subject": "Public audit"}, "requester": map[string]string{"name": "Pat Visitor", "email": "pat@example.com"}}
	f.want(f.call(nil, "POST", "/public/forms/"+token+"/submissions", payload), 201, "public submits")
	if n := f.queryID("SELECT count(*)::text FROM audit_events WHERE action='form.submit' AND actor_id='00000000-0000-4000-8000-0000000000e1' AND details->>'via'='public' AND details->>'visitor_email'='pat@example.com'"); n != "1" {
		t.Fatalf("public submission audit rows = %s", n)
	}
}

// Anonymous submissions keep working, and a tampered anonymous request still lands only in the form's Space.
func TestPublicSubmissionCannotChooseAnotherDestination(t *testing.T) {
	f := newFixture(t)
	s := setupScopedForm(t, f)
	link := f.call(s.admin, "POST", "/forms/"+s.formID+"/public-link", nil)
	f.want(link, 200, "create public link")
	token, _ := link.body["public_token"].(string)
	payload := map[string]any{"answers": map[string]any{"subject": "Anon tampered", "space_id": s.other, "locations": []map[string]any{{"space_id": s.other}}},
		"requester": map[string]string{"name": "Pat", "email": "pat@example.com"}, "locations": []map[string]any{{"space_id": s.other}}}
	res := f.call(nil, "POST", "/public/forms/"+token+"/submissions", payload)
	if res.status != http.StatusCreated && res.status != http.StatusUnprocessableEntity && res.status != http.StatusBadRequest {
		t.Fatalf("anonymous tampered submission: status %d", res.status)
	}
	if n := f.queryID("SELECT count(*)::text FROM item_locations WHERE space_id=$1 AND deleted_root_id IS NULL AND item_id IN (SELECT id FROM items WHERE title='Anon tampered')", s.other); n != "0" {
		t.Fatalf("an anonymous submission reached another Space: %s", n)
	}
	f.want(f.call(nil, "POST", "/public/forms/"+token+"/submissions", map[string]any{"answers": map[string]any{"subject": "Anon ok"}, "requester": map[string]string{"name": "Pat", "email": "pat@example.com"}}), 201, "clean anonymous submission still works")
	if n := f.queryID("SELECT count(*)::text FROM items i JOIN item_locations l ON l.item_id=i.id WHERE i.title='Anon ok' AND l.space_id=$1", s.work); n != "1" {
		t.Fatalf("clean anonymous submission not in the form Space: %s", n)
	}
}
