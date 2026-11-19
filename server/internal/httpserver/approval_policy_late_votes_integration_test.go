package httpserver_test

import "testing"

// Lane-D (QA B2): Any/All outcomes, late votes after a final outcome, and immutability of every recorded vote.
func TestApprovalAnyAllLateVotesAndFinality(t *testing.T) {
	f := newFixture(t)
	admin, a, b, c := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Approval policies")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	for _, p := range []*person{a, b, c} {
		f.share(space, "user", p.id, "read")
	}
	item := f.call(admin, "POST", "/items", map[string]any{"type": "task", "title": "Gate", "locations": []map[string]any{{"space_id": space}}})
	f.want(item, 201, "create item")
	path := "/items/" + item.body["id"].(string) + "/approvals"
	make := func(policy string) string {
		r := f.call(admin, "POST", path, map[string]any{"policy": policy, "approver_ids": []string{a.id, b.id, c.id}})
		f.want(r, 201, "request "+policy)
		return r.body["id"].(string)
	}
	vote := func(id string, p *person, decision string) reply {
		return f.call(p, "PUT", "/approvals/"+id+"/decision", map[string]string{"decision": decision})
	}
	status := func(r reply) any { return r.body["status"] }

	// Any: one approval is enough; a vote by someone who has not voted yet may still arrive but cannot undo the outcome.
	anyID := make("any")
	if s := status(vote(anyID, a, "approved")); s != "approved" {
		t.Fatalf("any: first approval should finalize, got %v", s)
	}
	late := vote(anyID, b, "rejected")
	if late.status != 200 || status(late) != "approved" {
		t.Fatalf("any: late reject after approval must not flip the outcome: %d %v", late.status, late.body)
	}
	f.want(vote(anyID, a, "rejected"), 409, "any: approver of a final outcome cannot flip")
	f.want(vote(anyID, b, "approved"), 409, "any: late voter cannot change own recorded vote either")

	// All: a single rejection rejects at once; later approvals do not resurrect it.
	allID := make("all")
	if s := status(vote(allID, a, "approved")); s != "pending" {
		t.Fatalf("all: one of three approvals stays pending, got %v", s)
	}
	if s := status(vote(allID, b, "rejected")); s != "rejected" {
		t.Fatalf("all: one rejection rejects, got %v", s)
	}
	late = vote(allID, c, "approved")
	if late.status != 200 || status(late) != "rejected" {
		t.Fatalf("all: late approval must not resurrect a rejected approval: %d %v", late.status, late.body)
	}
	f.want(vote(allID, b, "approved"), 409, "all: rejecter cannot flip after final")

	// Every decision is in the append-only history exactly once.
	if n := f.queryID("SELECT count(*)::text FROM approval_events WHERE approval_id=$1 AND kind='decided'", anyID); n != "2" {
		t.Fatalf("any history decisions: %s", n)
	}
	if n := f.queryID("SELECT count(*)::text FROM approval_events WHERE approval_id=$1 AND kind='decided'", allID); n != "3" {
		t.Fatalf("all history decisions: %s", n)
	}
}
