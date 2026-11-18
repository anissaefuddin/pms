package httpserver_test

import (
	"encoding/json"
	"testing"
)

// A4 / AUT-18: a rule exports as a file without ids or history, imports as a draft, and the templates are served.
func TestRuleExportImportAndTemplates(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	src := e.rule(t, e.editor, "Thank", `{"when":{"type":"approval.decided","decision":"approved"},"then":[{"type":"comment","text":"thanks"}]}`)

	exported := f.call(e.editor, "GET", "/automation-rules/"+src+"/export", nil)
	f.want(exported, 200, "export")
	if exported.body["format"] != "pms.automation.v1" || exported.body["name"] != "Thank" || exported.body["id"] != nil {
		t.Fatalf("the export carries the definition and no ids: %+v", exported.body)
	}
	f.want(f.call(f.person("regular"), "GET", "/automation-rules/"+src+"/export", nil), 404, "an outsider cannot export")

	imported := f.call(e.editor, "POST", "/spaces/"+e.space+"/automation-rules/import", exported.body)
	f.want(imported, 201, "import")
	if imported.body["enabled"] != false || imported.body["current_version_id"] != nil || imported.body["id"] == src {
		t.Fatalf("an import is a new, disabled draft: %+v", imported.body)
	}
	copied := f.call(e.editor, "GET", "/automation-rules/"+imported.body["id"].(string), nil)
	if string(mustRaw(t, copied.body["versions"].([]any)[0].(map[string]any)["definition"])) != string(mustRaw(t, exported.body["definition"])) {
		t.Fatal("the import has the exported definition")
	}

	bad := map[string]any{"format": "something-else", "name": "x", "definition": exported.body["definition"]}
	f.want(f.call(e.editor, "POST", "/spaces/"+e.space+"/automation-rules/import", bad), 422, "a file from elsewhere is refused")
	broken := map[string]any{"format": "pms.automation.v1", "name": "x", "definition": map[string]any{"when": map[string]any{"type": "nope"}, "then": []any{}}}
	f.want(f.call(e.editor, "POST", "/spaces/"+e.space+"/automation-rules/import", broken), 422, "an invalid definition is refused")
	f.want(f.call(f.person("regular"), "POST", "/spaces/"+e.space+"/automation-rules/import", exported.body), 404, "an outsider cannot import")

	tpl := f.call(e.editor, "GET", "/automation/templates", nil)
	f.want(tpl, 200, "templates")
	if len(tpl.items()) < 4 {
		t.Fatalf("templates are served: %+v", tpl.items())
	}
}

func mustRaw(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
