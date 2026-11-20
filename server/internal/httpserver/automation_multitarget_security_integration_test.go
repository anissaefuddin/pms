package httpserver_test

import (
	"fmt"
	"testing"
)

// wsItemInBoth creates a task in the first Space and places it in the second too, so it matches both targets.
func (w wsEnv) wsItemInBoth(t *testing.T, title string) string {
	t.Helper()
	id := w.itemIn(t, w.space, title)
	w.f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES($1,$2,NULL,'zz')", id, w.other)
	return id
}

// Negative: usage is shown only to those who may see it. A workspace rule's counters go to its owner and to admins; a Space's counter to
// those who can edit the Space; everyone else is refused.
func TestUsageIsShownOnlyToThoseWhoMayManageIt(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "Metered", completeDef, w.space)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")
	outsider := f.person("regular")

	f.want(f.call(outsider, "GET", "/automation/usage?rule_id="+id, nil), 404, "an outsider cannot see the rule's usage")
	f.want(f.call(outsider, "GET", "/automation/usage?space_id="+w.space, nil), 404, "an outsider cannot see a Space's usage")
	f.want(f.call(w.editor, "GET", "/automation/usage", nil), 403, "the workspace total is for admins")
	ownerView := f.call(w.editor, "GET", "/automation/usage?rule_id="+id, nil)
	f.want(ownerView, 200, "the owner sees the rule's usage")
	if ownerView.body["workspace"] != nil {
		t.Fatalf("the owner does not see the workspace total: %+v", ownerView.body)
	}
	adminView := f.call(w.admin, "GET", "/automation/usage?rule_id="+id, nil)
	if adminView.body["workspace"] == nil {
		t.Fatalf("an admin sees the workspace total: %+v", adminView.body)
	}
	f.want(f.call(w.admin, "GET", "/automation/usage", nil), 200, "the workspace total")
}

// Usage states: a rule at 80 percent warns; at its limit it is exhausted and names the throttling cap.
func TestUsageWarnsAndThrottles(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "Warned", completeDef, w.space)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")
	f.want(f.call(w.admin, "PUT", "/automation/limits", map[string]any{"rule_per_hour": 5, "workspace_per_hour": 500, "space_per_hour": 500}), 200, "rule cap of five")
	for i := range 4 {
		w.itemIn(t, w.space, fmt.Sprintf("Warm %d", i))
	}
	w.drain(t)
	warn := f.call(w.editor, "GET", "/automation/usage?rule_id="+id, nil)
	rule := warn.body["rule"].(map[string]any)
	if rule["used"] != float64(4) || rule["state"] != "warning" || warn.body["throttled_by"] != "" {
		t.Fatalf("four of five is a warning, not a throttle: %+v", warn.body)
	}

	f.want(f.call(w.admin, "PUT", "/automation/limits", map[string]any{"rule_per_hour": 4, "workspace_per_hour": 500, "space_per_hour": 500}), 200, "rule cap of four")
	hit := f.call(w.editor, "GET", "/automation/usage?rule_id="+id, nil)
	if hit.body["rule"].(map[string]any)["state"] != "exhausted" || hit.body["throttled_by"] != "rule_quota" {
		t.Fatalf("at the cap the rule is exhausted and says why: %+v", hit.body)
	}
	w.itemIn(t, w.space, "Blocked")
	w.drain(t)
	if f.queryID("SELECT count(*)::text FROM automation_runs WHERE rule_id=$1 AND reason='rule_quota'", id) == "0" {
		t.Fatal("the run is blocked by the same counter the usage shows")
	}
}

// AUT-T02s: confidential Spaces are deferred until the feature exists. Until then, a Space is hidden from people without access, with
// the existing permissions: a non-member gets not found for its workspace rules and its usage.
func TestUnauthorizedSpaceIsHiddenWithTheExistingPermissions(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "Private Space rule", completeDef, w.space)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")
	stranger := f.person("regular")
	f.want(f.call(stranger, "GET", "/spaces/"+w.space+"/workspace-automation", nil), 404, "a non-member sees nothing of the Space")
	f.want(f.call(stranger, "GET", "/automation/usage?space_id="+w.space, nil), 404, "a non-member sees no usage of the Space")
	f.want(f.call(stranger, "GET", "/automation-rules/"+id+"/runs", nil), 404, "a non-member sees no log")
	f.want(f.call(stranger, "POST", "/automation-rules/"+id+"/duplicate", nil), 404, "a non-member cannot copy the rule")
}
