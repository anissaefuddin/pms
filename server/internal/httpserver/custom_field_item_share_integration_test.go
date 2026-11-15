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
