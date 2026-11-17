package httpserver_test

import (
	"net/http"
	"testing"
)

// Workspace custom fields: admins manage them; a Workspace item type can use only these; their values are settable on items in
// any Space the item sits in, and a Space's own list does not show them.
func TestWorkspaceCustomFields(t *testing.T) {
	f := newFixture(t)
	admin, editor, owner, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	spaceA := f.space("Alpha")
	f.share(spaceA, "user", editor.id, "editor")
	f.share(spaceA, "user", owner.id, "full")

	field := f.call(admin, http.MethodPost, "/custom-fields", map[string]any{"name": "Cost center", "type": "number"})
	f.want(field, 201, "admin creates a Workspace field")
	if field.body["space_id"] != "" {
		t.Fatalf("a Workspace field has no Space: %v", field.body)
	}
	fid := field.body["id"].(string)
	f.want(f.call(editor, http.MethodPost, "/custom-fields", map[string]any{"name": "Mine", "type": "number"}), 403, "editor cannot create a Workspace field")
	f.want(f.call(editor, http.MethodGet, "/custom-fields", nil), 403, "editor cannot list Workspace fields")

	spaceField := f.call(owner, http.MethodPost, "/spaces/"+spaceA+"/custom-fields", map[string]any{"name": "Budget A", "type": "number"})
	f.want(spaceField, 201, "editor creates a Space field")
	sid := spaceField.body["id"].(string)
	listed := f.call(owner, http.MethodGet, "/spaces/"+spaceA+"/custom-fields", nil)
	if len(listed.items()) != 1 || listed.items()[0]["id"] != sid {
		t.Fatalf("a Space's list shows its own fields only: %v", listed.body)
	}

	wsType := f.call(admin, http.MethodPost, "/item-types", map[string]any{"key": "ws_req", "name": "Workspace request", "base": "task"})
	f.want(wsType, 201, "Workspace type")
	wsID := wsType.body["id"].(string)
	f.want(f.call(admin, http.MethodPut, "/item-types/"+wsID+"/fields/"+fid, map[string]any{}), 200, "Workspace type takes a Workspace field")
	f.want(f.call(admin, http.MethodPut, "/item-types/"+wsID+"/fields/"+sid, map[string]any{}), 422, "Workspace type refuses a Space field")

	item := f.call(editor, http.MethodPost, "/items", map[string]any{"type_key": "ws_req", "title": "Req", "locations": []map[string]any{{"space_id": spaceA}}})
	f.want(item, 201, "item of the Workspace type in Alpha")
	itemID := item.body["id"].(string)
	f.want(f.call(editor, http.MethodPatch, "/items/"+itemID, map[string]any{"fields": map[string]any{fid: 42}}, "If-Match", `"1"`), 200, "value of a Workspace field")
	got := f.call(editor, http.MethodGet, "/items/"+itemID, nil)
	if values, _ := got.body["fields"].(map[string]any); values[fid] != float64(42) {
		t.Fatalf("the Workspace field value is shown on the item: %v", got.body["fields"])
	}
	f.want(f.call(outsider, http.MethodGet, "/items/"+itemID, nil), 404, "an outsider cannot see the item or its value")
}
