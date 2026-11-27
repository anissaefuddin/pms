package httpserver_test

import (
	"fmt"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/automation"
)

// B3-13: an automation rule runs as its owner, and that authority is checked when it runs, not when it was saved. On a
// client-owned Space the owner must still be an active member of the Customer, and an assignee must be a member too.
func newClientAutoEnv(t *testing.T) (*autoEnv, *person, *person, string) {
	t.Helper()
	f := newFixture(t)
	admin, editor, member := f.person("admin"), f.person("regular"), f.person("regular")
	client := f.createClient(admin, "AU-A")["id"].(string)
	f.addMember(admin, client, admin)
	f.addMember(admin, client, editor)
	f.addMember(admin, client, member)
	space := f.scopedSpace("AU client", "client", client)
	f.share(space, "user", editor.id, "full")
	f.share(space, "user", admin.id, "full")
	e := &autoEnv{f: f, engine: automation.New(f.pool), admin: admin, editor: editor, space: space}
	return e, member, admin, client
}

func assigneesOf(f *fixture, item string) string {
	return f.queryID("SELECT count(*)::text FROM item_assignees WHERE item_id=$1", item)
}

// AU-01..AU-03 ride the same rule: on item creation it assigns the given people.
func TestAutomationAssignmentIsCheckedAtRunTime(t *testing.T) {
	e, member, admin, client := newClientAutoEnv(t)
	f := e.f
	outsider := f.person("regular")
	assign := func(user string) string {
		return fmt.Sprintf(`{"when":{"type":"item.created"},"then":[{"type":"assign","user_ids":["%s"]}]}`, user)
	}
	memberRule := e.rule(t, e.editor, "Assign member", assign(member.id))
	outsiderRule := e.rule(t, e.editor, "Assign outsider", assign(outsider.id))

	t.Run("AU-01 a rule assigns an active member of the Customer", func(t *testing.T) {
		item := e.item(t, admin, "Member assigned", nil)
		e.drain(t)
		if n := assigneesOf(f, item["id"].(string)); n == "0" {
			t.Fatalf("the member rule did not assign, runs %v", e.runs(memberRule))
		}
	})
	t.Run("AU-02 a rule cannot assign a non-member of the Customer", func(t *testing.T) {
		f := e.f
		item := e.item(t, admin, "Outsider refused", nil)
		e.drain(t)
		if n := f.queryID("SELECT count(*)::text FROM item_assignees WHERE item_id=$1 AND user_id=$2", item["id"], outsider.id); n != "0" {
			t.Fatalf("a non-member was assigned by automation")
		}
		if e.runs(outsiderRule)["done"] != 0 {
			t.Fatalf("the refused rule must not report a done run: %v", e.runs(outsiderRule))
		}
	})
	t.Run("AU-03 revoking the owner's membership stops the rule from acting", func(t *testing.T) {
		f := e.f
		doneBefore := e.runs(memberRule)["done"]
		f.revokeMember(admin, client, e.editor)
		item := e.item(t, admin, "After revoke", nil)
		e.drain(t)
		if n := assigneesOf(f, item["id"].(string)); n != "0" {
			t.Fatalf("a revoked owner's rule still assigned: %s", n)
		}
		if e.runs(memberRule)["done"] != doneBefore {
			t.Fatalf("a run completed after the owner was revoked: %v", e.runs(memberRule))
		}
	})
}

// AU-04: a revoked owner's rule is paused with a reason code, so it stays off until someone resumes it.
func TestRevokedOwnerPausesRule(t *testing.T) {
	e, member, admin, client := newClientAutoEnv(t)
	f := e.f
	rule := e.rule(t, e.editor, "Owner revoked", fmt.Sprintf(`{"when":{"type":"item.created"},"then":[{"type":"assign","user_ids":["%s"]}]}`, member.id))
	f.revokeMember(admin, client, e.editor)
	e.item(t, admin, "Trigger after revoke", nil)
	e.drain(t)
	if health := f.queryID("SELECT health FROM automation_rules WHERE id=$1", rule); health == "ok" {
		t.Fatalf("a rule whose owner lost Customer access must not stay ok")
	}
}
