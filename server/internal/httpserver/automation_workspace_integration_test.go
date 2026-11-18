package httpserver_test

import (
	"fmt"
	"testing"
)

// AUT-02 (design approved, decisions D1 to D4). Each test names the design scenario it covers (AUT-T02a..t).

// wsEnv is the Space the owner works in, a second Space, and a workspace rule factory.
type wsEnv struct {
	*autoEnv
	other string
}

func newWsEnv(t *testing.T) wsEnv {
	t.Helper()
	e := newAutoEnv(t)
	other := e.f.space("Other")
	e.f.share(other, "user", e.editor.id, "full")
	return wsEnv{autoEnv: e, other: other}
}

// wsRule creates a workspace rule as an admin, publishes it, hands it to the editor (the owner), and sets its targets.
func (w wsEnv) wsRule(t *testing.T, name, def string, targets ...string) string {
	t.Helper()
	f := w.f
	made := f.call(w.admin, "POST", "/automation/workspace-rules", map[string]any{"name": name, "definition": jsonRaw(def)})
	f.want(made, 201, "workspace rule "+name)
	id := made.body["id"].(string)
	f.want(f.call(w.admin, "POST", "/automation-rules/"+id+"/publish", nil), 200, "publish "+name)
	f.want(f.call(w.admin, "POST", "/automation-rules/"+id+"/owner", map[string]any{"owner_id": w.editor.id}), 200, "hand "+name+" to the owner")
	if len(targets) > 0 {
		f.want(f.call(w.admin, "POST", "/automation-rules/"+id+"/targets", map[string]any{"space_ids": targets}), 200, "targets of "+name)
	}
	return id
}

// itemIn creates a task in a Space as the editor (the owner's own rights).
func (w wsEnv) itemIn(t *testing.T, space, title string) string {
	t.Helper()
	got := w.f.call(w.editor, "POST", "/items", map[string]any{"type": "task", "title": title, "locations": []map[string]any{{"space_id": space}}})
	w.f.want(got, 201, "item "+title)
	return got.body["id"].(string)
}

const completeDef = `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"status_group","value":"completed"}]}`

// AUT-T02a, AUT-T02b, AUT-T02d: only a workspace admin creates a workspace rule; a draft may have no targets.
func TestWorkspaceRuleIsCreatedByAdminsAndDraftsMayHaveNoTargets(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	f.want(f.call(w.editor, "POST", "/automation/workspace-rules", map[string]any{"name": "Mine", "definition": jsonRaw(completeDef)}), 403, "a Space editor cannot create a workspace rule")
	draft := f.call(w.admin, "POST", "/automation/workspace-rules", map[string]any{"name": "Draft", "definition": jsonRaw(completeDef)})
	f.want(draft, 201, "an admin creates a draft without targets")
	if draft.body["scope"] != "workspace" || draft.body["enabled"] != false || draft.body["space_id"] != "" {
		t.Fatalf("a workspace rule has no Space of its own: %+v", draft.body)
	}
}

// AUT-T02c: an active workspace rule needs at least one target Space.
func TestWorkspaceRuleNeedsATargetToBeEnabled(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "No targets", completeDef)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 422, "enable without a target is refused")
}

// AUT-T02e, AUT-T02f: a workspace rule runs only in its target Spaces; the target list is a snapshot, so a Space created later is not
// included.
func TestWorkspaceRuleRunsOnlyInItsTargets(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "Close in targets", completeDef, w.space)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")

	inTarget := w.itemIn(t, w.space, "In target")
	outside := w.itemIn(t, w.other, "Outside target")
	w.drain(t)
	if f.queryID("SELECT grp::text FROM statuses WHERE id=(SELECT status_id FROM items WHERE id=$1)", inTarget) != "completed" {
		t.Fatal("the item in a target Space is completed")
	}
	if f.queryID("SELECT grp::text FROM statuses WHERE id=(SELECT status_id FROM items WHERE id=$1)", outside) == "completed" {
		t.Fatal("an item outside the targets is not touched")
	}

	later := f.space("Created later")
	f.share(later, "user", w.editor.id, "full")
	lateItem := w.itemIn(t, later, "Created later")
	w.drain(t)
	if f.queryID("SELECT grp::text FROM statuses WHERE id=(SELECT status_id FROM items WHERE id=$1)", lateItem) == "completed" {
		t.Fatal("a Space created after the snapshot is not a target")
	}
}

// AUT-T02g, AUT-T02i: a run needs the owner's rights in the Space. Where the owner has lost access, that Space is refused and recorded;
// the other Spaces keep running, and the rule is not paused.
func TestWorkspaceRuleRefusesOnlyWhereTheOwnerCannotAct(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "Two Spaces", completeDef, w.space, w.other)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")
	f.exec("DELETE FROM shares WHERE target_id=$1 AND principal_id=$2", w.space, w.editor.id)

	refused := w.itemIn(t, w.other, "Still allowed")
	blocked := f.call(w.admin, "POST", "/items", map[string]any{"type": "task", "title": "Blocked", "locations": []map[string]any{{"space_id": w.space}}})
	f.want(blocked, 201, "admin creates the item in the first Space")
	w.drain(t)
	if f.queryID("SELECT grp::text FROM statuses WHERE id=(SELECT status_id FROM items WHERE id=$1)", refused) != "completed" {
		t.Fatal("the other target still runs")
	}
	if f.queryID("SELECT count(*)::text FROM automation_runs WHERE rule_id=$1 AND reason='owner_no_access' AND space_id=$2", id, w.space) == "0" {
		t.Fatal("the refused Space is on the trail with its reason")
	}
	got := f.call(w.admin, "GET", "/automation-rules/"+id, nil)
	if got.body["health"] != "ok" {
		t.Fatalf("losing access in one Space does not pause the whole rule: %+v", got.body)
	}
}

// AUT-T02j, AUT-T02k: status categories map to the item's own workflow; a category the workflow does not have is refused with
// status_missing, not with a failure.
func TestWorkspaceRuleMovesByStatusCategory(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "Close by category", completeDef, w.space)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")
	item := w.itemIn(t, w.space, "Category")
	w.drain(t)
	if f.queryID("SELECT grp::text FROM statuses WHERE id=(SELECT status_id FROM items WHERE id=$1)", item) != "completed" {
		t.Fatal("the category resolves to the workflow's completed status")
	}

	// A category with no status in the item's workflow: the run records status_missing.
	missing := w.wsRule(t, "Cancel", `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"status_group","value":"cancelled"}]}`, w.space)
	f.exec("UPDATE statuses SET grp='active' WHERE grp='cancelled'")
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+missing, map[string]any{"enabled": true}), 200, "enable")
	w.itemIn(t, w.space, "No cancelled status")
	w.drain(t)
	if f.queryID("SELECT count(*)::text FROM automation_runs WHERE rule_id=$1 AND reason='status_missing'", missing) == "0" {
		t.Fatal("a missing category is recorded as status_missing")
	}
}

// AUT-T02l, AUT-T02m: a workspace rule may use Workspace fields, and a Space field only when it has exactly one target. Field ids
// differ between Spaces, so a field with the same name in another Space is not matched.
func TestWorkspaceRuleFieldsAreWorkspaceLevelOrOneTarget(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	field := f.call(w.editor, "POST", "/spaces/"+w.space+"/custom-fields", map[string]any{"name": "Budget", "type": "number"})
	f.want(field, 201, "Budget in the first Space")
	fid := field.body["id"].(string)
	f.call(w.editor, "POST", "/spaces/"+w.other+"/custom-fields", map[string]any{"name": "Budget", "type": "number"})
	def := fmt.Sprintf(`{"when":{"type":"item.created"},"if":[{"field":"fields.%s","op":"set"}],"then":[{"type":"set_field","field":"status_group","value":"completed"}]}`, fid)

	two := w.wsRule(t, "Two targets", def, w.space, w.other)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+two, map[string]any{"enabled": true}), 422, "a Space field with two targets is refused")
	other := w.wsRule(t, "Other only", def, w.other)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+other, map[string]any{"enabled": true}), 422, "a same-named field in another Space is not the same field")
	one := w.wsRule(t, "One target", def, w.space)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+one, map[string]any{"enabled": true}), 200, "a Space field with one matching target is accepted")
}

// AUT-T02n, AUT-T02o, AUT-T02p, AUT-T02q: the three caps count the hour; an admin changes them and each change is on the trail; a
// Space editor cannot read or change them.
func TestWorkspaceLimitsCountAndAreAudited(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	f.want(f.call(w.editor, "GET", "/automation/limits", nil), 403, "a Space editor cannot read the limits")
	f.want(f.call(w.editor, "PUT", "/automation/limits", map[string]any{"rule_per_hour": 1, "workspace_per_hour": 500, "space_per_hour": 500}), 403, "a Space editor cannot change the limits")

	id := w.wsRule(t, "Capped", completeDef, w.space)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")
	f.want(f.call(w.admin, "PUT", "/automation/limits", map[string]any{"rule_per_hour": 2, "workspace_per_hour": 500, "space_per_hour": 500}), 200, "rule cap of two an hour")
	if f.queryID("SELECT count(*)::text FROM audit_events WHERE action='automation_limits.updated'") != "1" {
		t.Fatal("the change is on the trail")
	}
	for i := range 3 {
		w.itemIn(t, w.space, fmt.Sprintf("Burst %d", i))
	}
	w.drain(t)
	if e := f.queryID("SELECT count(*)::text FROM automation_runs WHERE rule_id=$1 AND status='done' AND action_index=0", id); e != "2" {
		t.Fatalf("two runs fit under the cap: %s", e)
	}
	if f.queryID("SELECT count(*)::text FROM automation_runs WHERE rule_id=$1 AND status='quota_blocked' AND reason='rule_quota'", id) == "0" {
		t.Fatal("the third is blocked and says which cap")
	}

	f.want(f.call(w.admin, "PUT", "/automation/limits", map[string]any{"rule_per_hour": 100, "workspace_per_hour": 1, "space_per_hour": 500}), 200, "workspace cap of one")
	w.itemIn(t, w.space, "Over workspace cap")
	w.drain(t)
	if f.queryID("SELECT count(*)::text FROM automation_runs WHERE rule_id=$1 AND reason='workspace_quota'", id) == "0" {
		t.Fatal("the workspace cap is counted across workspace rules")
	}

	f.want(f.call(w.admin, "PUT", "/automation/limits", map[string]any{"rule_per_hour": 100, "workspace_per_hour": 500, "space_per_hour": 1}), 200, "space cap of one")
	w.itemIn(t, w.space, "Over space cap")
	w.drain(t)
	if f.queryID("SELECT count(*)::text FROM automation_runs WHERE rule_id=$1 AND reason='space_quota'", id) == "0" {
		t.Fatal("the Space cap applies to workspace rules too")
	}
	f.want(f.call(w.admin, "PUT", "/automation/limits", map[string]any{"rule_per_hour": 0, "workspace_per_hour": 500, "space_per_hour": 500}), 422, "a limit must be at least 1")
}

// AUT-T02h, AUT-T02t (part): an admin sees a workspace rule's outcomes, not its content; the owner sees the content.
func TestWorkspaceLogShowsOutcomesNotContentToAdmins(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "Private content", completeDef, w.space)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")
	w.itemIn(t, w.space, "Secret title")
	w.drain(t)

	asAdmin := f.call(w.admin, "GET", "/automation-rules/"+id+"/runs", nil).items()
	asOwner := f.call(w.editor, "GET", "/automation-rules/"+id+"/runs", nil).items()
	if len(asAdmin) == 0 || asAdmin[0]["item_title"] != nil || asAdmin[0]["detail"] != "" {
		t.Fatalf("an admin sees the outcome without the title: %+v", asAdmin)
	}
	if len(asOwner) == 0 || asOwner[0]["item_title"] != "Secret title" {
		t.Fatalf("the owner sees the title: %+v", asOwner)
	}
	f.want(f.call(w.editor, "GET", "/automation/workspace-rules", nil), 403, "a Space editor cannot list workspace rules")
}

// AUT-T02r: Space rules behave as before; the regression suites cover this (see the automation tests).

// AUT-T02s (confidential Space) and AUT-T02t (dry-run per Space) are not run: the confidential Space concept does not exist yet.

// A Space editor sees that a workspace rule runs in their Space: its name and state, not its owner or content (decision D4).
func TestSpaceEditorSeesOnlyTheNameOfWorkspaceRulesRunningInTheirSpace(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "Visible name", completeDef, w.space)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")

	got := f.call(w.editor, "GET", "/spaces/"+w.space+"/workspace-automation", nil)
	f.want(got, 200, "the Space lists its workspace rules")
	items := got.items()
	if len(items) != 1 || items[0]["name"] != "Visible name" || items[0]["enabled"] != true {
		t.Fatalf("the name and state are shown: %+v", items)
	}
	if _, has := items[0]["owner_id"]; has {
		t.Fatal("the owner is not shown to a Space editor")
	}
	f.want(f.call(w.editor, "GET", "/spaces/"+w.other+"/workspace-automation", nil), 200, "another Space the editor can edit")
	if n := len(f.call(w.editor, "GET", "/spaces/"+w.other+"/workspace-automation", nil).items()); n != 0 {
		t.Fatalf("a Space that is not a target lists nothing: %d", n)
	}
	f.want(f.call(f.person("regular"), "GET", "/spaces/"+w.space+"/workspace-automation", nil), 404, "an outsider cannot see the Space")
}
