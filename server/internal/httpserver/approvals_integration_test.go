package httpserver_test

import (
	"net/http"
	"testing"
)

func TestApprovals(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	editor, viewer, approver, outsider := f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Work")
	f.share(space, "user", editor.id, "editor")
	f.share(space, "user", viewer.id, "read")
	f.share(space, "user", approver.id, "read") // viewing is enough to decide
	item := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": "Ship", "locations": []map[string]any{{"space_id": space}}})
	f.want(item, 201, "create item")
	path := "/items/" + item.body["id"].(string) + "/approvals"

	good := map[string]any{"description": "Please check", "due_date": "2026-12-01", "approver_ids": []string{approver.id, viewer.id}}
	f.want(f.call(viewer, "POST", path, good), 403, "viewer cannot request approvals")
	f.want(f.call(outsider, "POST", path, good), 404, "no access is 404")
	for name, body := range map[string]map[string]any{
		"no approvers":        {"approver_ids": []string{}},
		"missing approvers":   {"description": "x"},
		"approver w/o access": {"approver_ids": []string{outsider.id}},
		"bad date":            {"approver_ids": []string{approver.id}, "due_date": "2026-02-30"},
		"unknown field":       {"approver_ids": []string{approver.id}, "status": "approved"},
	} {
		if r := f.call(editor, "POST", path, body); r.status != 422 && r.status != 400 {
			t.Fatalf("%s: want 4xx validation, got %d %v", name, r.status, r.body)
		}
	}
	created := f.call(editor, "POST", path, good)
	f.want(created, 201, "editor requests approval")
	if created.body["status"] != "pending" || created.body["approved_count"] != float64(0) || created.body["total"] != float64(2) || created.header.Get("ETag") != `"1"` {
		t.Fatalf("new approval: %v etag=%q", created.body, created.header.Get("ETag"))
	}
	id := created.body["id"].(string)
	if n := f.queryID("SELECT count(*)::text FROM notifications WHERE kind='approval_requested' AND recipient_id=$1", approver.id); n != "1" {
		t.Fatalf("approver must be notified, got %s", n)
	}
	list := f.call(viewer, "GET", path, nil)
	if list.status != 200 || len(list.items()) != 1 {
		t.Fatalf("viewer lists approvals: %d %v", list.status, list.body)
	}
	f.want(f.call(outsider, "GET", path, nil), 404, "outsider list")

	// Decisions: only named approvers, either decision, changeable; the counter and status follow.
	decide := func(p *person, decision string) reply {
		return f.call(p, "PUT", "/approvals/"+id+"/decision", map[string]any{"decision": decision, "comment": "ok"})
	}
	f.want(decide(editor, "approved"), 403, "not an approver")
	f.want(decide(outsider, "approved"), 404, "no access")
	f.want(f.call(approver, "PUT", "/approvals/"+id+"/decision", map[string]any{"decision": "maybe"}), 422, "unknown decision")
	one := decide(approver, "approved")
	if one.status != 200 || one.body["approved_count"] != float64(1) || one.body["status"] != "pending" {
		t.Fatalf("first approval: %d %v", one.status, one.body)
	}
	if n := f.queryID("SELECT count(*)::text FROM notifications WHERE kind='approval_decided' AND recipient_id=$1", editor.id); n != "1" {
		t.Fatalf("creator must hear about the decision, got %s", n)
	}
	two := decide(viewer, "approved")
	if two.body["status"] != "approved" || two.body["approved_count"] != float64(2) {
		t.Fatalf("all approved: %v", two.body)
	}
	flipped := decide(viewer, "rejected")
	if flipped.body["status"] != "rejected" || flipped.body["approved_count"] != float64(1) {
		t.Fatalf("one rejection rejects the approval: %v", flipped.body)
	}

	// Editing: needs edit rights and the current version; removing an approver drops their decision.
	version := flipped.header.Get("ETag")
	patch := map[string]any{"approver_ids": []string{approver.id}}
	f.want(f.call(editor, "PATCH", "/approvals/"+id, patch), 428, "missing If-Match")
	f.want(f.call(editor, "PATCH", "/approvals/"+id, patch, "If-Match", `"1"`), 409, "stale version")
	f.want(f.call(viewer, "PATCH", "/approvals/"+id, patch, "If-Match", version), 403, "viewer cannot edit")
	f.want(f.call(editor, "PATCH", "/approvals/"+id, map[string]any{"approver_ids": []string{outsider.id}}, "If-Match", version), 422, "new approver needs access")
	f.want(f.call(editor, "PATCH", "/approvals/"+id, map[string]any{}, "If-Match", version), 422, "empty patch")
	edited := f.call(editor, "PATCH", "/approvals/"+id, map[string]any{"approver_ids": []string{approver.id}, "description": "Updated", "due_date": nil}, "If-Match", version)
	if edited.status != 200 || edited.body["total"] != float64(1) || edited.body["status"] != "approved" || edited.body["due_date"] != nil || edited.body["description"] != "Updated" {
		t.Fatalf("edit: %d %v", edited.status, edited.body)
	}

	// Delete.
	f.want(f.call(viewer, "DELETE", "/approvals/"+id, nil), 403, "viewer delete")
	f.want(f.call(outsider, "DELETE", "/approvals/"+id, nil), 404, "outsider delete")
	f.want(f.call(editor, "DELETE", "/approvals/"+id, nil), 204, "editor delete")
	if got := f.call(editor, "GET", path, nil); len(got.items()) != 0 {
		t.Fatalf("approval must be gone: %v", got.body)
	}
	f.want(f.call(editor, "DELETE", "/approvals/"+id, nil), 404, "delete twice")
}
