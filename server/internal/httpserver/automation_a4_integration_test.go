package httpserver_test

import (
	"fmt"
	"testing"
	"time"
)

// A4: a rule can react to an approval's outcome, to a comment, to an item that is overdue, and can open a subitem under the item.
func TestApprovalOutcomeTriggersOnlyTheMatchingRule(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	admin, approver := e.admin, f.person("regular")
	f.share(e.space, "user", approver.id, "read")
	stat := func(group string) string {
		return f.queryID("SELECT id::text FROM statuses WHERE grp=$1::status_group ORDER BY position LIMIT 1", group)
	}
	review, approved, rejected := stat("deferred"), stat("completed"), stat("cancelled")
	f.want(f.call(admin, "PATCH", "/statuses/"+review, map[string]any{"approval_approver_ids": []string{approver.id}, "approved_status_id": approved, "rejected_status_id": rejected}), 200, "review needs an approval")
	e.rule(t, e.editor, "Enter review", fmt.Sprintf(`{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"status_id","value":"%s"}]}`, review))
	onApproved := e.rule(t, e.editor, "On approved", `{"when":{"type":"approval.decided","decision":"approved"},"then":[{"type":"comment","text":"approved"}]}`)
	onRejected := e.rule(t, e.editor, "On rejected", `{"when":{"type":"approval.decided","decision":"rejected"},"then":[{"type":"comment","text":"rejected"}]}`)

	item := e.item(t, admin, "Sign me off", nil)
	e.drain(t)
	aid := f.call(admin, "GET", "/items/"+item["id"].(string)+"/approvals", nil).items()[0]["id"].(string)
	f.want(f.call(approver, "PUT", "/approvals/"+aid+"/decision", map[string]any{"decision": "approved", "comment": "ok"}), 200, "approve")
	e.drain(t)
	if e.runs(onApproved)["done"] != 1 || e.runs(onRejected)["done"] != 0 {
		t.Fatalf("an approval runs only the rule for its outcome: approved %v rejected %v", e.runs(onApproved), e.runs(onRejected))
	}
	if f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1 AND body_text='approved' AND via='automation_bot'", item["id"]) != "1" {
		t.Fatal("the approved rule commented once, as the bot")
	}
}

// A4: a comment triggers a rule once; the rule's own comment does not trigger it again.
func TestCommentTriggersARuleOnceWithoutLooping(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	rule := e.rule(t, e.editor, "Reply", `{"when":{"type":"comment.created"},"then":[{"type":"comment","text":"Thanks, noted"}]}`)
	item := e.item(t, e.admin, "Discuss", nil)
	f.want(f.call(e.admin, "POST", "/items/"+item["id"].(string)+"/comments", map[string]any{"body": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Question?"}}}}}}), 201, "a person comments")
	e.drain(t)
	if n := f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1 AND via='automation_bot'", item["id"]); n != "1" {
		t.Fatalf("one reply, not a loop: %s replies, runs %v", n, e.runs(rule))
	}
}

// A4 / AUT-T18 (part): an item that is overdue triggers the overdue rule once per day, and never twice for the same item.
func TestOverdueRuleFiresOncePerItem(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	yesterday := time.Now().UTC().AddDate(0, 0, -3).Format("2006-01-02")
	rule := e.rule(t, e.editor, "Overdue", `{"when":{"type":"item.due_reached","days_before":-3},"then":[{"type":"comment","text":"Overdue!"}]}`)
	f.want(f.call(e.editor, "PUT", "/automation-rules/"+rule+"/draft", map[string]any{"definition": map[string]any{"when": map[string]any{"type": "item.due_reached", "days_before": -31}, "if": []any{}, "then": []any{map[string]any{"type": "comment", "text": "x"}}}}), 422, "a window beyond 30 days is refused")
	e.item(t, e.admin, "Late", map[string]any{"due_date": yesterday})
	for range 2 {
		if _, err := e.engine.EmitDue(f.ctx); err != nil {
			t.Fatal(err)
		}
		e.drain(t)
	}
	if e.runs(rule)["done"] != 1 || f.queryID("SELECT count(*)::text FROM comments WHERE body_text='Overdue!'") != "1" {
		t.Fatalf("the overdue rule acts once: %v", e.runs(rule))
	}
}

// A4: a create-item action with under=item makes the new item a subitem of the item that triggered the rule.
func TestCreateItemUnderTheTriggeringItem(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	bad := f.call(e.editor, "POST", "/spaces/"+e.space+"/automation-rules", map[string]any{"name": "Bad under", "definition": map[string]any{"when": map[string]any{"type": "item.created"}, "if": []any{}, "then": []any{map[string]any{"type": "comment", "text": "x", "under": "item"}}}})
	f.want(bad, 422, "under belongs to create_item only")
	rule := e.rule(t, e.editor, "Subtask", `{"when":{"type":"item.created"},"if":[{"field":"item_type","op":"eq","value":"task"}],"then":[{"type":"create_item","title":"Sub of {item.title}","kind":"task","under":"item"}]}`)
	parent := e.item(t, e.admin, "Parent", nil)
	e.drain(t)
	child := f.queryID("SELECT l.item_id::text FROM item_locations l JOIN items i ON i.id=l.item_id WHERE l.parent_item_id=$1 AND i.title='Sub of Parent' AND l.deleted_root_id IS NULL", parent["id"])
	if child == "" || e.runs(rule)["done"] == 0 {
		t.Fatalf("the subitem is created under the parent: child %q runs %v", child, e.runs(rule))
	}
}
