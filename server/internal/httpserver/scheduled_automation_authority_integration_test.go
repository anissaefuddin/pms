package httpserver_test

import (
	"fmt"
	"testing"
	"time"
)

// PROMPT #026 Workstream B: a scheduled rule (item.due_reached) runs as its owner at execution time. Revoking the owner's Customer
// membership after the rule was created stops later scheduled runs, and a scheduled run never assigns a non-member.
func TestScheduledAutomationUsesCurrentOwnerAccess(t *testing.T) {
	e, member, admin, client := newClientAutoEnv(t)
	f := e.f
	outsider := f.person("regular")
	yesterday := time.Now().UTC().AddDate(0, 0, -3).Format("2006-01-02")
	assign := func(user string) string {
		return fmt.Sprintf(`{"when":{"type":"item.due_reached","days_before":-3},"then":[{"type":"assign","user_ids":["%s"]}]}`, user)
	}
	memberRule := e.rule(t, e.editor, "Scheduled member", assign(member.id))
	outsiderRule := e.rule(t, e.editor, "Scheduled outsider", assign(outsider.id))
	runDue := func() {
		t.Helper()
		if _, err := e.engine.EmitDue(f.ctx); err != nil {
			t.Fatal(err)
		}
		e.drain(t)
	}

	// SCH-01: positive control. The owner is an active member, so the scheduled run assigns the eligible member.
	due := e.item(t, admin, "Due while owner active", map[string]any{"due_date": yesterday})
	runDue()
	if n := f.queryID("SELECT count(*)::text FROM item_assignees WHERE item_id=$1 AND user_id=$2", due["id"], member.id); n != "1" {
		t.Fatalf("SCH-01: the scheduled run must assign the member (runs %v)", e.runs(memberRule))
	}

	// SCH-03: a scheduled run never assigns a non-member of the Customer.
	outsiderDue := e.item(t, admin, "Due for outsider rule", map[string]any{"due_date": yesterday})
	runDue()
	if n := f.queryID("SELECT count(*)::text FROM item_assignees WHERE item_id=$1 AND user_id=$2", outsiderDue["id"], outsider.id); n != "0" {
		t.Fatalf("SCH-03: a scheduled run assigned a non-member (runs %v)", e.runs(outsiderRule))
	}

	// SCH-02: the owner loses Customer access after the rule was created. A later scheduled run must not act for them.
	doneBefore := e.runs(memberRule)["done"]
	f.revokeMember(admin, client, e.editor)
	later := e.item(t, admin, "Due after owner revoked", map[string]any{"due_date": yesterday})
	runDue()
	if n := f.queryID("SELECT count(*)::text FROM item_assignees WHERE item_id=$1", later["id"]); n != "0" {
		t.Fatalf("SCH-02: a scheduled run assigned after the owner's access ended")
	}
	if e.runs(memberRule)["done"] != doneBefore {
		t.Fatalf("SCH-02: a scheduled run completed for a revoked owner: %v", e.runs(memberRule))
	}
}
