package httpserver_test

import (
	"net/http"
	"strconv"
	"testing"
)

// Lane-D (QA B2): custom field access after the item-level share change (lane2-004). Actors: item editor, item reader, external limited, outsider; field of another Space.
func TestCustomFieldAccessRegression(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	editor, reader, ext, outsider := f.person("regular"), f.person("regular"), f.person("external"), f.person("regular")
	spaceA, spaceB := f.space("CF A"), f.space("CF B")
	f.share(spaceA, "user", admin.id, "full") // A1.2: admin acts in this Space as an explicit member
	f.share(spaceB, "user", admin.id, "full") // A1.2: admin defines the field of the other Space; the other actors still must not see it
	fieldA := f.call(admin, http.MethodPost, "/spaces/"+spaceA+"/custom-fields", map[string]any{"name": "Budget A", "type": "number"})
	f.want(fieldA, 201, "field A")
	fieldB := f.call(admin, http.MethodPost, "/spaces/"+spaceB+"/custom-fields", map[string]any{"name": "Secret B", "type": "number"})
	f.want(fieldB, 201, "field B")
	fa, fb := fieldA.body["id"].(string), fieldB.body["id"].(string)
	item := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": "In A", "locations": []map[string]any{{"space_id": spaceA}}})
	f.want(item, 201, "item in A")
	id := item.body["id"].(string)
	other := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": "In B", "locations": []map[string]any{{"space_id": spaceB}}})
	f.want(other, 201, "item in B")
	oid := other.body["id"].(string)
	f.want(f.call(admin, "PATCH", "/items/"+oid, map[string]any{"fields": map[string]any{fb: 99}}, "If-Match", `"1"`), 200, "seed B value")
	cur := f.call(admin, "GET", "/items/"+id+"/shares", nil)
	f.want(f.call(admin, "PUT", "/items/"+id+"/shares", map[string]any{"shares": []map[string]any{
		{"principal_type": "user", "principal_id": editor.id, "role": "editor"},
		{"principal_type": "user", "principal_id": reader.id, "role": "read"},
		{"principal_type": "user", "principal_id": ext.id, "role": "limited"},
	}}, "If-Match", cur.header.Get("ETag")), 200, "item shares")
	ver := func(p *person) string {
		return `"` + strconv.Itoa(int(f.call(p, "GET", "/items/"+id, nil).body["version"].(float64))) + `"`
	}
	set := func(p *person, field string, v any) reply {
		return f.call(p, "PATCH", "/items/"+id, map[string]any{"fields": map[string]any{field: v}}, "If-Match", ver(admin))
	}
	denied := func(r reply, who string) {
		t.Helper()
		if r.status != 403 && r.status != 404 && r.status != 422 {
			t.Fatalf("%s must not set the value: %d %v", who, r.status, r.body)
		}
	}
	f.want(set(editor, fa, 10), 200, "item editor sets Space field")
	denied(set(reader, fa, 11), "item reader")
	denied(set(ext, fa, 12), "external limited")
	denied(set(outsider, fa, 13), "outsider")
	// Cross-space: a field of Space B cannot be written onto an item of Space A by anyone, including the item editor.
	for who, p := range map[string]*person{"editor": editor, "admin": admin} {
		if r := set(p, fb, 7); r.status == 200 {
			t.Fatalf("%s wrote a Space B field onto a Space A item", who)
		}
	}
	// Reads: readers and the limited external see the A value, outsiders get nothing, nobody sees the Space B value through item A.
	for who, p := range map[string]*person{"reader": reader, "external": ext, "editor": editor} {
		got := f.call(p, "GET", "/items/"+id, nil)
		f.want(got, 200, who+" reads item")
		fields, _ := got.body["fields"].(map[string]any)
		if _, leak := fields[fb]; leak {
			t.Fatalf("%s sees Space B field on item A: %v", who, fields)
		}
	}
	f.want(f.call(outsider, "GET", "/items/"+id, nil), 404, "outsider read")
	f.want(f.call(editor, "GET", "/items/"+oid, nil), 404, "item editor cannot read the Space B item (and its secret field)")
	// The item-level editor must not manage field definitions (Space level).
	denied(f.call(editor, http.MethodPost, "/spaces/"+spaceA+"/custom-fields", map[string]any{"name": "Mine", "type": "number"}), "item editor creating a field definition")
	denied(f.call(editor, http.MethodPatch, "/spaces/"+spaceA+"/custom-fields/"+fa, map[string]any{"name": "Renamed"}, "If-Match", `"1"`), "item editor renaming a field definition")
}
