package httpserver_test

import (
	"fmt"
	"testing"
)

// A1: a rule that cannot run says why. Owner problems pause it, missing dependencies make it invalid, and only a resume that
// revalidates it brings it back.
func TestPausedRuleResumesOnlyAfterRevalidation(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	rule := e.rule(t, e.editor, "Flag hot", `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"importance","value":"high"}]}`)

	f.exec("DELETE FROM shares WHERE target_id=$1 AND principal_id=$2", e.space, e.editor.id)
	first := e.item(t, e.admin, "Lost access", nil)
	e.drain(t)
	if importanceOf(e, first["id"].(string)) != "normal" || e.runs(rule)["denied"] != 1 {
		t.Fatalf("a rule whose owner lost access must not act: runs %v", e.runs(rule))
	}
	got := f.call(e.admin, "GET", "/automation-rules/"+rule, nil)
	if got.body["health"] != "paused" || got.body["health_reason"] != "owner_no_access" {
		t.Fatalf("the rule says why it is paused: %+v", got.body)
	}
	if f.queryID("SELECT count(*)::text FROM audit_events WHERE action='automation_rule.paused' AND target_id=$1 AND actor_id IS NULL AND actor_name='Automation Bot'", rule) != "1" {
		t.Fatal("the pause is written to the trail as the Automation Bot")
	}

	// Resuming is refused while the owner still cannot run the rule; the admin is told why.
	f.want(f.call(e.admin, "PATCH", "/automation-rules/"+rule, map[string]any{"enabled": true}), 422, "resume refused without access")

	// After access is restored, resume revalidates and the rule runs again.
	f.share(e.space, "user", e.editor.id, "full")
	f.want(f.call(e.admin, "PATCH", "/automation-rules/"+rule, map[string]any{"enabled": true}), 200, "resume after access is back")
	second := e.item(t, e.admin, "Back", nil)
	e.drain(t)
	if importanceOf(e, second["id"].(string)) != "high" {
		t.Fatal("a resumed rule runs")
	}
}

// A1 / AUT-T16 and AUT-T20: a rule that points at a custom field that was archived, or at a user who was deactivated, becomes
// invalid, stops acting, and says which dependency is gone. Restoring the dependency and resuming makes it run again.
func TestInvalidRuleWhenADependencyIsGone(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	field := f.call(e.editor, "POST", "/spaces/"+e.space+"/custom-fields", map[string]any{"name": "Budget", "type": "number"})
	f.want(field, 201, "custom field")
	fid := field.body["id"].(string)
	rule := e.rule(t, e.editor, "Budget note", fmt.Sprintf(`{"when":{"type":"item.created"},"if":[{"field":"fields.%s","op":"set"}],"then":[{"type":"comment","text":"Has a budget"}]}`, fid))

	f.exec("UPDATE custom_fields SET archived_at=now() WHERE id=$1", fid)
	gone := e.item(t, e.admin, "Archived field", nil)
	e.drain(t)
	got := f.call(e.admin, "GET", "/automation-rules/"+rule, nil)
	if got.body["health"] != "invalid" || got.body["health_reason"] != "field_missing" {
		t.Fatalf("an archived field makes the rule invalid: %+v", got.body)
	}
	if f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1", gone["id"]) != "0" {
		t.Fatal("an invalid rule does not act")
	}
	f.want(f.call(e.admin, "PATCH", "/automation-rules/"+rule, map[string]any{"enabled": true}), 422, "resume refused while the field is archived")

	f.exec("UPDATE custom_fields SET archived_at=NULL WHERE id=$1", fid)
	f.want(f.call(e.admin, "PATCH", "/automation-rules/"+rule, map[string]any{"enabled": true}), 200, "resume after the field is restored")

	// A user the rule notifies is deactivated: invalid again, for that reason.
	other := f.person("regular")
	f.share(e.space, "user", other.id, "read")
	notify := e.rule(t, e.editor, "Tell someone", fmt.Sprintf(`{"when":{"type":"item.created"},"then":[{"type":"notify","user_ids":["%s"],"message":"New item {item.title}"}]}`, other.id))
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", other.id)
	e.item(t, e.admin, "Notify me", nil)
	e.drain(t)
	if got := f.call(e.admin, "GET", "/automation-rules/"+notify, nil); got.body["health"] != "invalid" || got.body["health_reason"] != "user_inactive" {
		t.Fatalf("a deactivated recipient makes the rule invalid: %+v", got.body)
	}
}

// A1 / AUT-T19: an action refused by policy stops the rule. The actions before it stay done, the ones after it do not run, and the
// trail records where it stopped. Nothing is rolled back.
func TestRefusedActionStopsTheRule(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	admin := e.admin
	field := f.call(e.editor, "POST", "/spaces/"+e.space+"/custom-fields", map[string]any{"name": "Salary", "type": "number"})
	f.want(field, 201, "field")
	sid := field.body["id"].(string)
	f.want(f.call(e.editor, "PUT", "/spaces/"+e.space+"/custom-fields/"+sid+"/grants",
		map[string]any{"grants": []map[string]any{{"subject_kind": "user", "subject_id": admin.id, "level": "edit"}}}), 200, "only the admin may set Salary")
	typ := f.call(admin, "POST", "/item-types", map[string]any{"key": "offer", "name": "Offer", "base": "task", "space_id": e.space})
	f.want(typ, 201, "type")
	f.want(f.call(admin, "PUT", "/item-types/"+typ.body["id"].(string)+"/fields/"+sid, map[string]any{}), 200, "type takes Salary")

	rule := e.rule(t, e.editor, "Three steps", fmt.Sprintf(`{"when":{"type":"item.created"},"then":[
		{"type":"comment","text":"first"},
		{"type":"set_field","field":"fields.%s","value":1},
		{"type":"comment","text":"third"}]}`, sid))
	made := f.call(e.editor, "POST", "/items", map[string]any{"type_key": "offer", "title": "Offer", "locations": []map[string]any{{"space_id": e.space}}})
	f.want(made, 201, "item")
	e.drain(t)

	runs := e.runs(rule)
	if runs["done"] != 1 || runs["denied"] != 1 || runs["stopped"] != 1 {
		t.Fatalf("one done, one refused, and the stop on record: %v", runs)
	}
	if f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1 AND body_text='first'", made.body["id"]) != "1" ||
		f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1 AND body_text='third'", made.body["id"]) != "0" {
		t.Fatal("the action before the refusal stays done; the one after it does not run")
	}
	stopped := f.queryID("SELECT detail FROM automation_runs WHERE rule_id=$1 AND status='stopped'", rule)
	if stopped != "stopped at action 2 of 3" {
		t.Fatalf("the stop names its position: %q", stopped)
	}
}

// A1: only a workspace admin transfers a rule, the new owner must be able to run it, and the rule runs under its new owner.
func TestOwnerTransferIsAdminOnlyAndRevalidates(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	rule := e.rule(t, e.editor, "Handover", `{"when":{"type":"item.created"},"then":[{"type":"comment","text":"mine"}]}`)
	next := f.person("regular")
	f.share(e.space, "user", next.id, "full")
	outsider := f.person("regular")

	path := "/automation-rules/" + rule + "/owner"
	f.want(f.call(e.editor, "POST", path, map[string]any{"owner_id": next.id}), 403, "the owner cannot hand the rule over")
	f.want(f.call(e.admin, "POST", path, map[string]any{"owner_id": outsider.id}), 422, "the new owner must be able to run rules here")
	f.want(f.call(e.admin, "POST", path, map[string]any{"owner_id": "not-a-user"}), 422, "owner_id must be a user id")
	f.want(f.call(e.admin, "POST", path, map[string]any{"owner_id": next.id}), 200, "admin hands the rule to a Space member")

	e.item(t, e.admin, "After handover", nil)
	e.drain(t)
	if actor := f.queryID("SELECT actor_id::text FROM automation_runs WHERE rule_id=$1 AND status='done' ORDER BY created_at DESC LIMIT 1", rule); actor != next.id {
		t.Fatalf("the rule acts as its new owner: %s", actor)
	}
	if f.queryID("SELECT count(*)::text FROM audit_events WHERE action='automation_rule.owner_transferred' AND target_id=$1 AND details->>'to'=$2", rule, next.id) != "1" {
		t.Fatal("the transfer is on the trail")
	}
}

// A1 delete policy and audit: archive stops a rule without losing its history; restore brings it back disabled; each step is recorded.
func TestArchiveRestoreAndTheTrail(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	rule := e.rule(t, e.editor, "Seasonal", `{"when":{"type":"item.created"},"then":[{"type":"comment","text":"seasonal"}]}`)
	e.item(t, e.admin, "Before archive", nil)
	e.drain(t)
	before := e.runs(rule)["done"]

	f.want(f.call(e.editor, "POST", "/automation-rules/"+rule+"/archive", nil), 200, "archive")
	e.item(t, e.admin, "While archived", nil)
	e.drain(t)
	if e.runs(rule)["done"] != before {
		t.Fatal("an archived rule does not act")
	}
	if got := f.call(e.editor, "GET", "/automation-rules/"+rule+"/runs", nil); len(got.items()) == 0 {
		t.Fatal("archiving keeps the history")
	}

	restored := f.call(e.editor, "POST", "/automation-rules/"+rule+"/restore", nil)
	f.want(restored, 200, "restore")
	if restored.body["archived_at"] != nil || restored.body["enabled"] != false {
		t.Fatalf("a restored rule comes back disabled: %+v", restored.body)
	}
	f.want(f.call(e.editor, "PATCH", "/automation-rules/"+rule, map[string]any{"enabled": true}), 200, "enable again")
	e.item(t, e.admin, "After restore", nil)
	e.drain(t)
	if e.runs(rule)["done"] != before+1 {
		t.Fatal("a restored and enabled rule acts again")
	}

	for _, action := range []string{"automation_rule.created", "automation_rule.published", "automation_rule.enabled", "automation_rule.archived", "automation_rule.restored"} {
		if f.queryID("SELECT count(*)::text FROM audit_events WHERE action=$1 AND target_id=$2", action, rule) == "0" {
			t.Fatalf("%s is on the trail", action)
		}
	}
	f.want(f.call(f.person("regular"), "POST", "/automation-rules/"+rule+"/archive", nil), 404, "a stranger cannot archive it")
}
