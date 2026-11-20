package httpserver_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/httpserver"
)

func TestApprovals(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	editor, viewer, approver, outsider := f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Work")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
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
	// QA-D-lane2-011: the approval is final, so a voter cannot flip their vote afterwards.
	if flipped := decide(viewer, "rejected"); flipped.status != 409 || flipped.body["code"] != "approval_final" {
		t.Fatalf("final approval must refuse a changed vote: %d %v", flipped.status, flipped.body)
	}
	flipped := two

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

func TestApprovalHistoryIsAppendOnlyAndPinnedToItemVersion(t *testing.T) {
	f := newFixture(t)
	admin, approver, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Work")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	f.share(space, "user", approver.id, "read")
	item := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": "Ship", "locations": []map[string]any{{"space_id": space}}})
	f.want(item, 201, "create item")
	itemID := item.body["id"].(string)
	made := f.call(admin, "POST", "/items/"+itemID+"/approvals", map[string]any{"approver_ids": []string{approver.id}})
	f.want(made, 201, "request")
	id := made.body["id"].(string)
	f.want(f.call(approver, "PUT", "/approvals/"+id+"/decision", map[string]any{"decision": "approved", "comment": "fine"}), 200, "decide")

	// QA-D-lane2-011: a finalized approval is immutable; a second decision is refused and the history keeps one decision.
	f.exec("UPDATE items SET version=version+5 WHERE id=$1", itemID)
	f.want(f.call(approver, "PUT", "/approvals/"+id+"/decision", map[string]any{"decision": "rejected", "comment": "changed my mind"}), 409, "final approval cannot be re-decided")
	hist := f.call(admin, "GET", "/items/"+itemID+"/approval-events", nil)
	f.want(hist, 200, "history")
	ev := hist.items()
	if len(ev) != 2 || ev[0]["kind"] != "decided" || ev[0]["decision"] != "approved" || ev[1]["kind"] != "approver_requested" {
		t.Fatalf("history newest first, one decision: %v", ev)
	}
	f.want(f.call(outsider, "GET", "/items/"+itemID+"/approval-events", nil), 404, "no access is 404")

	// Deleting the approval is recorded, and the rows can never be changed or removed, not even by SQL.
	f.want(f.call(admin, "DELETE", "/approvals/"+id, nil), 204, "withdraw")
	if n := f.queryID("SELECT count(*)::text FROM approval_events WHERE item_id=$1", itemID); n != "3" {
		t.Fatalf("history survives the approval: %s", n)
	}
	for _, q := range []string{"UPDATE approval_events SET comment='x'", "DELETE FROM approval_events", "TRUNCATE approval_events"} {
		if _, err := f.pool.Exec(context.Background(), q); err == nil {
			t.Fatalf("%s must be rejected", q)
		}
	}
}

func TestApprovalPoliciesReassignAndDeadlines(t *testing.T) {
	f := newFixture(t)
	admin, first, second, replacement := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Approval deadlines")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	for _, reviewer := range []*person{first, second, replacement} {
		f.share(space, "user", reviewer.id, "read")
	}
	item := f.call(admin, "POST", "/items", map[string]any{"type": "task", "title": "Release", "locations": []map[string]any{{"space_id": space}}})
	f.want(item, 201, "create item")
	path := "/items/" + item.body["id"].(string) + "/approvals"
	makeApproval := func(policy string) reply {
		got := f.call(admin, "POST", path, map[string]any{"policy": policy, "due_date": "2026-11-13", "approver_ids": []string{first.id, second.id}})
		f.want(got, 201, "request approval")
		if got.body["policy"] != policy {
			t.Fatalf("policy: %v", got.body)
		}
		return got
	}
	decide := func(r reply, actor *person, vote, want string) {
		got := f.call(actor, "PUT", "/approvals/"+r.body["id"].(string)+"/decision", map[string]string{"decision": vote})
		f.want(got, 200, "decide")
		if got.body["status"] != want {
			t.Fatalf("wanted %s: %v", want, got.body)
		}
	}
	anyApproval := makeApproval("any")
	decide(anyApproval, first, "rejected", "pending")
	decide(anyApproval, second, "rejected", "rejected")
	// QA-D-lane2-011: the rejected outcome is final; changing a vote afterwards is refused.
	f.want(f.call(second, "PUT", "/approvals/"+anyApproval.body["id"].(string)+"/decision", map[string]string{"decision": "approved"}), 409, "final vote is immutable")
	all := makeApproval("all")
	decide(all, first, "approved", "pending")
	id := all.body["id"].(string)
	body := map[string]any{"approver_ids": []string{first.id, replacement.id}}
	f.want(f.call(admin, "PATCH", "/approvals/"+id, body, "If-Match", all.header.Get("ETag")), 409, "stale reassign")
	f.want(f.call(second, "PATCH", "/approvals/"+id, body, "If-Match", `"2"`), 403, "viewer cannot reassign")
	got := f.call(admin, "PATCH", "/approvals/"+id, body, "If-Match", `"2"`)
	f.want(got, 200, "reassign")
	if got.body["approved_count"] != float64(1) {
		t.Fatalf("reassign erased a retained vote: %v", got.body)
	}
	f.want(f.call(second, "PUT", "/approvals/"+id+"/decision", map[string]string{"decision": "approved"}), 403, "old reviewer loses decision rights")
	if n := f.queryID("SELECT count(*)::text FROM approval_events WHERE approval_id=$1 AND ((kind='approver_removed' AND subject_id=$2) OR (kind='approver_requested' AND subject_id=$3))", id, second.id, replacement.id); n != "2" {
		t.Fatalf("reassignment history count: %s", n)
	}
	for _, day := range []int{12, 13, 13, 14, 14} {
		err := httpserver.DeliverApprovalDeadlines(t.Context(), f.pool, time.Date(2026, 11, day, 12, 0, 0, 0, time.UTC), f.queue.NotifyTx)
		if err != nil {
			t.Fatal(err)
		}
		want := "0"
		if day >= 13 {
			want = "1"
		}
		if n := f.queryID("SELECT count(*)::text FROM notifications WHERE kind='approval_reminder' AND recipient_id=$1", replacement.id); n != want {
			t.Fatalf("reminder count on day %d: %s", day, n)
		}
	}
	if n := f.queryID("SELECT count(*)::text FROM notifications WHERE kind='approval_escalated' AND recipient_id=$1", admin.id); n != "1" {
		t.Fatalf("escalation count: %s", n)
	}
	if n := f.queryID("SELECT count(*)::text FROM notifications WHERE kind IN ('approval_reminder','approval_escalated')"); n != "2" {
		t.Fatalf("unexpected deadline notifications: %s", n)
	}
	decide(all, replacement, "approved", "approved")
}
