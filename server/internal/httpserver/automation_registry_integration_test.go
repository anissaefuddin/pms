package httpserver_test

import (
	"testing"
)

// A2: the registry tells a Space editor what each rule is, who owns it, what it applies to, and how it has been doing.
func TestRegistryShowsEachRulesStateAndHistory(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	live := e.rule(t, e.editor, "Live", `{"when":{"type":"item.created"},"then":[{"type":"comment","text":"hi"}]}`)
	draft := f.call(e.editor, "POST", "/spaces/"+e.space+"/automation-rules", map[string]any{"name": "Draft only", "definition": map[string]any{"when": map[string]any{"type": "item.assigned"}, "if": []any{}, "then": []any{map[string]any{"type": "comment", "text": "later"}}}})
	f.want(draft, 201, "draft rule")
	f.share(e.space, "user", e.admin.id, "full")
	e.item(t, e.admin, "Registry item", nil)
	e.drain(t)

	list := f.call(e.editor, "GET", "/spaces/"+e.space+"/automation-rules", nil)
	f.want(list, 200, "registry")
	byName := map[string]map[string]any{}
	for _, row := range list.items() {
		byName[row["name"].(string)] = row
	}
	live_, dr := byName["Live"], byName["Draft only"]
	if live_ == nil || dr == nil {
		t.Fatalf("both rules are listed: %+v", list.items())
	}
	if live_["owner_name"] == "" || live_["trigger"] != "item.created" || live_["action_count"] != float64(1) || live_["has_draft"] != false {
		t.Fatalf("a published rule shows its owner, trigger and actions: %+v", live_)
	}
	if live_["last_run_status"] != "done" || live_["last_run_at"] == nil || live_["error_count"] != float64(0) {
		t.Fatalf("a rule shows its last run: %+v", live_)
	}
	if dr["has_draft"] != true || dr["current_version_id"] != nil || dr["trigger"] != "item.assigned" {
		t.Fatalf("a rule with only a draft says so: %+v", dr)
	}

	// A refused run counts as an error: the owner lost access, so the rule is paused and the registry says so.
	f.exec("DELETE FROM shares WHERE target_id=$1 AND principal_id=$2", e.space, e.editor.id)
	e.item(t, e.admin, "Refused", nil)
	e.drain(t)
	f.share(e.space, "user", e.editor.id, "full")
	list = f.call(e.editor, "GET", "/spaces/"+e.space+"/automation-rules", nil)
	for _, row := range list.items() {
		if row["name"] == "Live" {
			if row["health"] != "paused" || row["health_reason"] != "owner_no_access" || row["last_run_status"] != "denied" || row["error_count"] != float64(1) {
				t.Fatalf("a paused rule with a refused run shows both: %+v", row)
			}
		}
	}
	runs := f.call(e.admin, "GET", "/automation-rules/"+live+"/runs", nil)
	if len(runs.items()) == 0 || runs.items()[0]["reason"] != "owner_no_access" {
		t.Fatalf("the log line carries the reason code: %+v", runs.items())
	}

	f.want(f.call(f.person("regular"), "GET", "/spaces/"+e.space+"/automation-rules", nil), 404, "outsider sees nothing")
}

// A2: duplicating copies the latest definition into a disabled rule of the duplicator's own, which needs publishing and enabling.
func TestDuplicateCopiesTheDefinitionAsADisabledRule(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	src := e.rule(t, e.editor, "Escalate", `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"importance","value":"high"}]}`)
	reader := f.person("regular")
	f.share(e.space, "user", reader.id, "read")
	f.want(f.call(reader, "POST", "/automation-rules/"+src+"/duplicate", nil), 403, "a reader cannot duplicate")

	copied := f.call(e.admin, "POST", "/automation-rules/"+src+"/duplicate", nil)
	f.want(copied, 201, "duplicate")
	id := copied.body["id"].(string)
	if copied.body["name"] != "Copy of Escalate" || copied.body["enabled"] != false || copied.body["current_version_id"] != nil || copied.body["owner_id"] != e.admin.id {
		t.Fatalf("the copy is disabled, unpublished, and owned by its duplicator: %+v", copied.body)
	}
	got := f.call(e.admin, "GET", "/automation-rules/"+id, nil)
	if len(got.body["versions"].([]any)) != 1 {
		t.Fatalf("the copy has one draft: %+v", got.body["versions"])
	}
	if f.queryID("SELECT definition::text FROM automation_rule_versions WHERE rule_id=$1 AND status='draft'", id) != f.queryID("SELECT definition::text FROM automation_rule_versions WHERE rule_id=$1 AND status='published'", src) {
		t.Fatal("the copy has the same definition as the source")
	}
	f.want(f.call(e.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 422, "a copy must be published before it runs")
	f.want(f.call(f.person("regular"), "POST", "/automation-rules/"+src+"/duplicate", nil), 404, "an outsider cannot duplicate")
}
