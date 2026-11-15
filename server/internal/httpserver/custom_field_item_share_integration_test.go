package httpserver_test

import (
	"net/http"
	"strconv"
	"testing"
)

// QA-D-lane2-004: someone who may edit an item through an item-level share (no Space share) can set the Space's custom field values on it.
func TestItemLevelEditorCanSetSpaceCustomFieldValues(t *testing.T) {
	f := newFixture(t)
	admin, editor, reader := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Work")
	field := f.call(admin, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{"name": "Budget", "type": "number"})
	f.want(field, 201, "field")
	fid := field.body["id"].(string)
	item := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "project", "title": "P", "locations": []map[string]any{{"space_id": space}}})
	f.want(item, 201, "item")
	id := item.body["id"].(string)
	cur := f.call(admin, "GET", "/items/"+id+"/shares", nil)
	f.want(f.call(admin, "PUT", "/items/"+id+"/shares", map[string]any{"shares": []map[string]any{
		{"principal_type": "user", "principal_id": editor.id, "role": "editor"},
		{"principal_type": "user", "principal_id": reader.id, "role": "read"},
	}}, "If-Match", cur.header.Get("ETag")), 200, "item shares")
	version := strconv.Itoa(int(f.call(editor, "GET", "/items/"+id, nil).body["version"].(float64)))
	got := f.call(editor, "PATCH", "/items/"+id, map[string]any{"fields": map[string]any{fid: 1500}}, "If-Match", `"`+version+`"`)
	f.want(got, 200, "item-level editor sets a Space custom field")
	if v := f.call(reader, "PATCH", "/items/"+id, map[string]any{"fields": map[string]any{fid: 2}}, "If-Match", `"`+version+`"`); v.status != 403 && v.status != 404 {
		t.Fatalf("a read-only item share must not edit custom fields: %d", v.status)
	}
}

// QA-D-r3lane2-001: a relation field can never newly store an item the writer cannot view.
func TestRelationFieldRejectsHiddenTarget(t *testing.T) {
	f := newFixture(t)
	admin, editor := f.person("admin"), f.person("regular")
	space, hidden := f.space("Work"), f.space("Confidential")
	field := f.call(admin, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{"name": "Related", "type": "relation", "config": map[string]any{}})
	f.want(field, 201, "relation field")
	fid := field.body["id"].(string)
	mk := func(sp, title string) string {
		r := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": title, "locations": []map[string]any{{"space_id": sp}}})
		f.want(r, 201, title)
		return r.body["id"].(string)
	}
	src, ok, secret := mk(space, "Source"), mk(space, "Visible"), mk(hidden, "Secret")
	cur := f.call(admin, "GET", "/items/"+src+"/shares", nil)
	f.want(f.call(admin, "PUT", "/items/"+src+"/shares", map[string]any{"shares": []map[string]any{{"principal_type": "user", "principal_id": editor.id, "role": "editor"}}}, "If-Match", cur.header.Get("ETag")), 200, "share")
	cur = f.call(admin, "GET", "/items/"+ok+"/shares", nil)
	f.want(f.call(admin, "PUT", "/items/"+ok+"/shares", map[string]any{"shares": []map[string]any{{"principal_type": "user", "principal_id": editor.id, "role": "read"}}}, "If-Match", cur.header.Get("ETag")), 200, "share visible")
	version := strconv.Itoa(int(f.call(editor, "GET", "/items/"+src, nil).body["version"].(float64)))
	got := f.call(editor, "PATCH", "/items/"+src, map[string]any{"fields": map[string]any{fid: []string{ok, secret}}}, "If-Match", `"`+version+`"`)
	if got.status != 422 {
		t.Fatalf("a hidden relation target must be refused: %d %v", got.status, got.body)
	}
}
