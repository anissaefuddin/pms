package httpserver_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestRelationFieldsValidateTargetsAndDoNotLeakNames(t *testing.T) {
	f := newFixture(t)
	admin, editor := f.person("admin"), f.person("regular")
	open, secret := f.space("Open"), f.space("Secret")
	f.share(open, "user", admin.id, "full") // A1.2: admin acts in this Space as an explicit member
	f.share(open, "user", editor.id, "full")
	mk := func(space, title string) string {
		id := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task',$1,$2) RETURNING id::text", title, admin.id)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,$3)", id, space, title)
		return id
	}
	a, b, hidden := mk(open, "Alpha"), mk(open, "Beta"), mk(secret, "Top secret plan")
	field := f.call(admin, "POST", "/spaces/"+open+"/custom-fields", map[string]any{"name": "Blocks", "type": "relation"})
	f.want(field, 201, "relation field")
	fid := field.body["id"].(string)
	f.want(f.call(admin, "POST", "/spaces/"+open+"/custom-fields", map[string]any{"name": "Bad", "type": "text", "config": map[string]any{"target_type_key": "bug"}}), 422, "target type only on relation")
	set := func(p *person, item string, value any, version string) reply {
		return f.call(p, "PATCH", "/items/"+item, map[string]any{"fields": map[string]any{fid: value}}, "If-Match", `"`+version+`"`)
	}
	f.want(set(editor, a, []string{b}, "1"), 200, "relate to a visible item")
	rel := f.call(editor, "GET", "/items/"+a+"/relations", nil)
	groups := rel.items()
	if rel.status != 200 || len(groups) != 1 || groups[0]["hidden"] != float64(0) || groups[0]["items"].([]any)[0].(map[string]any)["title"] != "Beta" {
		t.Fatalf("relations: %+v", rel)
	}
	// Hidden and missing targets, self reference, duplicates and bad shapes all fail the same way, naming nothing.
	refused := map[string]any{}
	for name, value := range map[string]any{"hidden": []string{hidden}, "missing": []string{uuid.NewString()}} {
		got := set(editor, a, value, "2")
		if got.status != 422 || strings.Contains(string(mustJSON(got.body)), "Top secret") {
			t.Fatalf("%s: %+v", name, got)
		}
		refused[name] = got.body["detail"]
	}
	if refused["hidden"] != refused["missing"] {
		t.Fatalf("hidden and missing answer differently: %v", refused)
	}
	f.want(set(editor, a, []string{a}, "2"), 422, "self reference")
	f.want(set(editor, a, []string{b, b}, "2"), 422, "duplicates")
	f.want(set(editor, a, "x", "2"), 422, "not a list")
	// A target removed from the actor's view later stays stored but is only counted, never named.
	// D-17 A1: the admin can relate to the secret item only while holding a grant on its Space; the grant is removed after.
	f.share(secret, "user", admin.id, "full")
	f.want(set(admin, a, []string{b, hidden}, "2"), 200, "admin relates to the secret item")
	f.exec("DELETE FROM shares WHERE target_type='space' AND target_id=$1 AND principal_id=$2", secret, admin.id)
	got := f.call(editor, "GET", "/items/"+a+"/relations", nil)
	g := got.items()[0]
	if g["hidden"] != float64(1) || len(g["items"].([]any)) != 1 || strings.Contains(string(mustJSON(got.body)), "Top secret") {
		t.Fatalf("hidden target leaked: %+v", got)
	}
	// Keeping a target the editor cannot see is allowed; adding one is not.
	f.want(set(editor, a, []string{b, hidden}, "3"), 200, "existing hidden target is kept")
	if n := f.queryID("SELECT count(*)::text FROM item_relations WHERE field_id=$1 AND from_item_id=$2", fid, a); n != "2" {
		t.Fatalf("relation rows: %s", n)
	}
	if n := f.queryID("SELECT count(*)::text FROM item_relations WHERE to_item_id=$1", b); n != "1" {
		t.Fatalf("reverse lookup: %s", n)
	}
	f.want(set(editor, a, nil, "4"), 200, "clear")
	if n := f.queryID("SELECT count(*)::text FROM item_relations WHERE from_item_id=$1", a); n != "0" {
		t.Fatalf("rows after clear: %s", n)
	}
	// Deleted targets disappear from the list; the list itself obeys access to the source.
	f.want(set(editor, a, []string{b}, "5"), 200, "relate again")
	f.exec("UPDATE items SET deleted_at=now() WHERE id=$1", b)
	if got := f.call(editor, "GET", "/items/"+a+"/relations", nil); len(got.items()) != 0 {
		t.Fatalf("deleted target listed: %+v", got)
	}
	outsider := f.person("regular")
	f.want(f.call(outsider, "GET", "/items/"+a+"/relations", nil), 404, "no access to the source")
	// A field limited to one item type refuses other types.
	limited := f.call(admin, "POST", "/spaces/"+open+"/custom-fields", map[string]any{"name": "Only epics", "type": "relation", "config": map[string]any{"target_type_key": "epic"}})
	f.want(limited, 201, "limited field")
	c := mk(open, "Gamma")
	got = f.call(editor, "PATCH", "/items/"+a, map[string]any{"fields": map[string]any{limited.body["id"].(string): []string{c}}}, "If-Match", `"6"`)
	if got.status != 422 {
		t.Fatalf("wrong target type accepted: %+v", got)
	}
}
