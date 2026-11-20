package httpserver_test

import (
	"net/http"
	"testing"
)

// A1.6: a decision needs an assignment. A Workspace admin, or any other person who can see the item, cannot approve
// unless they are one of the approvers the request names, and a finalized approval stays final.
func TestApprovalDecisionNeedsAnAssignment(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	owner := f.person("regular")
	approver := f.person("regular")
	reader := f.person("regular") // can see the item, is not an approver
	work := f.space("Approvals")
	f.share(work, "user", owner.id, "full")
	f.share(work, "user", approver.id, "read")
	f.share(work, "user", reader.id, "read")

	item := f.call(owner, http.MethodPost, "/items", map[string]any{"type": "task", "title": "Sign off", "locations": []map[string]any{{"space_id": work}}})
	f.want(item, 201, "create item")
	itemID := item.body["id"].(string)
	made := f.call(owner, "POST", "/items/"+itemID+"/approvals", map[string]any{"approver_ids": []string{approver.id}})
	f.want(made, 201, "request approval from one approver")
	id := made.body["id"].(string)
	decide := "/approvals/" + id + "/decision"
	body := map[string]any{"decision": "approved", "comment": "ok"}

	// Refused: a Workspace admin without a grant cannot see the item at all.
	if got := f.call(admin, "PUT", decide, body); got.status == http.StatusOK {
		t.Fatalf("an admin without access approved: %+v", got)
	}
	// Refused: an admin who holds a grant but is not an assigned approver cannot decide.
	f.share(work, "user", admin.id, "full")
	f.want(f.call(admin, "PUT", decide, body), 403, "an admin who is not an assigned approver cannot approve")
	// Refused: someone who can see the item but is not an approver.
	f.want(f.call(reader, "PUT", decide, body), 403, "a reader who is not an approver cannot approve")
	// Allowed: the assigned approver.
	f.want(f.call(approver, "PUT", decide, body), 200, "the assigned approver approves")
	// Finality: the decided approver cannot change the outcome; the approval is recorded once.
	f.exec("UPDATE items SET version=version+5 WHERE id=$1", itemID)
	f.want(f.call(approver, "PUT", decide, map[string]any{"decision": "rejected", "comment": "again"}), 409, "a final approval cannot be re-decided")
	if n := f.queryID("SELECT count(*)::text FROM approval_events WHERE approval_id=$1 AND kind='decided'", id); n != "1" {
		t.Fatalf("exactly one decision is recorded, got %s", n)
	}
}
