package httpserver_test

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Decision: Try it uses the same validation pipeline as a run (guards, approvals, permissions) and writes nothing.
func TestTryItRefusesWhatARunWouldRefuseAndWritesNothing(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	admin, approver := e.admin, f.person("regular")
	f.share(e.space, "user", approver.id, "read")
	stat := func(group string) string {
		return f.queryID("SELECT id::text FROM statuses WHERE grp=$1::status_group ORDER BY position LIMIT 1", group)
	}
	review, approved, rejected := stat("deferred"), stat("completed"), stat("cancelled")
	f.want(f.call(admin, "PATCH", "/statuses/"+review, map[string]any{"approval_approver_ids": []string{approver.id}, "approved_status_id": approved, "rejected_status_id": rejected}), 200, "review needs an approval")
	adminOnly := rejected
	f.exec("UPDATE statuses SET actor_rule='admin' WHERE id=$1", adminOnly)

	item := e.item(t, admin, "Try me", nil)
	id := item["id"].(string)
	start := f.queryID("SELECT status_id::text FROM items WHERE id=$1", id)

	// A definition that would enter an approval-gated status, and one that only an admin may enter.
	toReview := fmt.Sprintf(`{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"status_id","value":"%s"},{"type":"comment","text":"after"}]}`, review)
	toAdminOnly := fmt.Sprintf(`{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"status_id","value":"%s"}]}`, adminOnly)

	before := map[string]string{
		"comments":      f.queryID("SELECT count(*)::text FROM comments"),
		"approvals":     f.queryID("SELECT count(*)::text FROM approvals"),
		"notifications": f.queryID("SELECT count(*)::text FROM notifications"),
		"events":        f.queryID("SELECT count(*)::text FROM domain_events"),
		"activity":      f.queryID("SELECT count(*)::text FROM activity"),
		"runs":          f.queryID("SELECT count(*)::text FROM automation_runs"),
	}
	dry := func(def string) map[string]any {
		got := f.call(e.editor, "POST", "/spaces/"+e.space+"/automation-rules/dry-run", map[string]any{"definition": json.RawMessage(def), "item_id": id})
		f.want(got, 200, "dry run")
		return got.body
	}

	ok := dry(toReview)
	if ok["would_run"] != true {
		t.Fatalf("entering review is allowed for the owner: %+v", ok)
	}
	trace, _ := ok["trace"].([]any)
	if len(trace) < 3 || !containsText(fmt.Sprint(trace[1]), "would be done") {
		t.Fatalf("the trace says each action would be done: %+v", trace)
	}

	refused := dry(toAdminOnly)
	if refused["would_run"] != false {
		t.Fatalf("an owner who is not an admin cannot enter an admin-only status: %+v", refused)
	}
	last := refused["trace"].([]any)
	if got := last[len(last)-1]; got == nil || !containsText(fmt.Sprint(got), "refused (guard)") {
		t.Fatalf("the trace says why it is refused: %+v", last)
	}

	// Nothing changed: the item is where it was, and no approval, comment, notification, event, activity, or run was written.
	after := map[string]string{
		"comments":      f.queryID("SELECT count(*)::text FROM comments"),
		"approvals":     f.queryID("SELECT count(*)::text FROM approvals"),
		"notifications": f.queryID("SELECT count(*)::text FROM notifications"),
		"events":        f.queryID("SELECT count(*)::text FROM domain_events"),
		"activity":      f.queryID("SELECT count(*)::text FROM activity"),
		"runs":          f.queryID("SELECT count(*)::text FROM automation_runs"),
	}
	for k := range before {
		if before[k] != after[k] {
			t.Fatalf("Try it must write nothing, but %s changed: %s -> %s", k, before[k], after[k])
		}
	}
	if f.queryID("SELECT status_id::text FROM items WHERE id=$1", id) != start {
		t.Fatal("Try it does not move the item")
	}
}

func containsText(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Try it on an existing rule runs as the rule's owner, as a run does: an admin who tests it sees the owner's refusal, not the admin's.
func TestTryItOnAnExistingRuleRunsAsItsOwner(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	admin := e.admin
	rejected := f.queryID("SELECT id::text FROM statuses WHERE grp='cancelled' ORDER BY position LIMIT 1")
	f.exec("UPDATE statuses SET actor_rule='admin' WHERE id=$1", rejected)
	rule := e.rule(t, e.editor, "Owner only", fmt.Sprintf(`{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"status_id","value":"%s"}]}`, rejected))
	item := e.item(t, admin, "Owner check", nil)
	body := func(withRule bool) map[string]any {
		m := map[string]any{"definition": json.RawMessage(fmt.Sprintf(`{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"status_id","value":"%s"}]}`, rejected)), "item_id": item["id"]}
		if withRule {
			m["rule_id"] = rule
		}
		return m
	}
	asAdmin := f.call(admin, "POST", "/spaces/"+e.space+"/automation-rules/dry-run", body(false))
	f.want(asAdmin, 200, "dry run as admin")
	if asAdmin.body["would_run"] != true {
		t.Fatalf("an admin may enter the status: %+v", asAdmin.body)
	}
	asOwner := f.call(admin, "POST", "/spaces/"+e.space+"/automation-rules/dry-run", body(true))
	f.want(asOwner, 200, "dry run of the existing rule")
	if asOwner.body["would_run"] != false {
		t.Fatalf("the existing rule runs as its owner, who may not enter the status: %+v", asOwner.body)
	}
}
