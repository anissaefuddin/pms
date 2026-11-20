package httpserver_test

import (
	"net/http"
	"strings"
	"testing"
)

// GET /blueprints/{id}: view on the Space reads the blueprint and its tree; outsiders get 404; a legacy restricted value is removed.
func TestBlueprintReadReturnsTreeAndHidesRestrictedFields(t *testing.T) {
	f := newFixture(t)
	e := setupEditable(t, f)
	path := "/blueprints/" + e.bp

	got := f.call(e.reader, "GET", path, nil)
	f.want(got, 200, "reader reads")
	tree, _ := got.body["tree"].(map[string]any)
	nodes, _ := tree["nodes"].([]any)
	if len(nodes) != 3 || got.body["name"] != "Editable" || got.body["version"] != float64(1) {
		t.Fatalf("read body: %v", got.body)
	}
	f.want(f.call(e.outsider, "GET", path, nil), 404, "outsider is 404")

	// A tree saved before the rule existed may hold a restricted value; the read must not return it.
	owner := f.person("regular")
	f.share(e.source, "user", owner.id, "full")
	salary := f.call(owner, http.MethodPost, "/spaces/"+e.source+"/custom-fields", map[string]any{"name": "Pay", "type": "number"})
	f.want(salary, 201, "field")
	sid := salary.body["id"].(string)
	f.want(f.call(owner, http.MethodPut, "/spaces/"+e.source+"/custom-fields/"+sid+"/grants",
		map[string]any{"grants": []map[string]any{{"subject_kind": "user", "subject_id": owner.id, "level": "edit"}}}), 200, "restrict")
	f.exec(`UPDATE blueprints SET tree = jsonb_set(tree, '{nodes,0,fields}', jsonb_build_object($2::text, 4242)) WHERE id=$1`, e.bp, sid)
	legacy := f.call(e.editor, "GET", path, nil)
	f.want(legacy, 200, "read legacy tree")
	raw := strings.Contains(string(mustJSON(legacy.body)), "4242")
	if raw {
		t.Fatalf("a restricted value was returned by the read")
	}
}
