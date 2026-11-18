package httpserver_test

import (
	"fmt"
	"testing"
)

// A1 verification (P0): an automation acts with its owner's rights and cannot skip approvals, workflow transitions, required fields,
// or role-only statuses. Each refusal is recorded with a reason code and stops the rule.
func TestAutomationKeepsApprovalsWorkflowsAndRoles(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	admin, approver := e.admin, f.person("regular")
	f.share(e.space, "user", approver.id, "read")
	stat := func(group string) string {
		return f.queryID("SELECT id::text FROM statuses WHERE grp=$1::status_group ORDER BY position LIMIT 1", group)
	}
	review, approved, rejected := stat("deferred"), stat("completed"), stat("cancelled")
	f.want(f.call(admin, "PATCH", "/statuses/"+review, map[string]any{"approval_approver_ids": []string{approver.id}, "approved_status_id": approved, "rejected_status_id": rejected}), 200, "review needs an approval")
	f.exec("UPDATE statuses SET lock_transitions=true WHERE id=$1", review) // no way out of review except the approval

	// reason returns the outcome and reason code of a rule's action 1.
	reason := func(rule string) (string, string) {
		var status, why string
		f.pool.QueryRow(f.ctx, "SELECT status, reason FROM automation_runs WHERE rule_id=$1 AND action_index=0 ORDER BY created_at DESC LIMIT 1", rule).Scan(&status, &why)
		return status, why
	}

	// 1. Entering review starts the approval as the owner. The rule then tries to jump to Approved without it: refused.
	enter := e.rule(t, e.editor, "Send to review", fmt.Sprintf(`{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"status_id","value":"%s"}]}`, review))
	jump := e.rule(t, e.editor, "Skip approval", fmt.Sprintf(`{"when":{"type":"item.status_changed","to_status_id":"%s"},"then":[{"type":"set_field","field":"status_id","value":"%s"}]}`, review, approved))
	item := e.item(t, admin, "Needs sign-off", nil)
	e.drain(t)
	list := f.call(admin, "GET", "/items/"+item["id"].(string)+"/approvals", nil).items()
	if len(list) != 1 || list[0]["status"] != "pending" {
		t.Fatalf("entering review must start the approval: %+v", list)
	}
	if got := f.call(admin, "GET", "/items/"+item["id"].(string), nil); got.body["status"].(map[string]any)["id"] != review {
		t.Fatalf("the item waits in review for its approval: %+v", got.body["status"])
	}
	if st, why := reason(jump); st != "denied" || why != "field_invalid" {
		t.Fatalf("an automation cannot leave a locked status for the approved one: %s %s", st, why)
	}
	f.want(f.call(e.editor, "PATCH", "/automation-rules/"+enter, map[string]any{"enabled": false}), 200, "switch off step 1")
	f.want(f.call(e.editor, "PATCH", "/automation-rules/"+jump, map[string]any{"enabled": false}), 200, "switch off step 1b")

	// 2. A status only admins may enter: the owner is not an admin, so the rule is refused by the guard.
	adminOnly := rejected // the default workflow has one cancelled status; any status the owner may not enter will do
	f.exec("UPDATE statuses SET actor_rule='admin' WHERE id=$1", adminOnly)
	roles := e.rule(t, e.editor, "Admin-only status", fmt.Sprintf(`{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"status_id","value":"%s"}]}`, adminOnly))
	e.item(t, admin, "Role check", nil)
	e.drain(t)
	if st, why := reason(roles); st != "denied" || why != "guard" {
		t.Fatalf("an admin-only status is not entered on an owner's rights: %s %s", st, why)
	}
	f.want(f.call(e.editor, "PATCH", "/automation-rules/"+roles, map[string]any{"enabled": false}), 200, "switch off step 2")

	// 3. A status that needs a due date: the item has none, so the guard refuses the move.
	start := stat("active") // new items start here, so the rule must move the item to a different active status
	active := f.queryID("SELECT id::text FROM statuses WHERE grp='active' AND id<>$1 ORDER BY position LIMIT 1", start)
	f.exec("UPDATE statuses SET required_fields=ARRAY['due_date'] WHERE id=$1", active)
	needs := e.rule(t, e.editor, "Needs due date", fmt.Sprintf(`{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"status_id","value":"%s"}]}`, active))
	e.item(t, admin, "No due date", nil)
	e.drain(t)
	if st, why := reason(needs); st != "denied" || why != "guard" {
		t.Fatalf("required fields apply to automation too: %s %s", st, why)
	}
}
