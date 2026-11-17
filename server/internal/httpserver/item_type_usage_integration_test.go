package httpserver_test

import (
	"net/http"
	"testing"
)

// Usage counts live Items per item type; only administrators see it, and deleted Items are not counted.
func TestItemTypeUsageCountsLiveItems(t *testing.T) {
	f := newFixture(t)
	admin, editor := f.person("admin"), f.person("regular")
	space := f.space("Usage")
	f.share(space, "user", editor.id, "editor")
	typed := f.call(admin, http.MethodPost, "/item-types", map[string]any{"key": "usage_req", "name": "Usage request", "base": "task"})
	f.want(typed, http.StatusCreated, "type")
	typeID := typed.body["id"].(string)
	var first string
	for i, title := range []string{"One", "Two", "Three"} {
		it := f.call(editor, http.MethodPost, "/items", map[string]any{"type_key": "usage_req", "title": title, "locations": []map[string]any{{"space_id": space}}})
		f.want(it, http.StatusCreated, "item "+title)
		if i == 0 {
			first = it.body["id"].(string)
		}
	}
	f.want(f.call(admin, http.MethodDelete, "/items/"+first, nil), http.StatusNoContent, "delete one item")

	usage := f.call(admin, http.MethodGet, "/item-types/usage", nil)
	f.want(usage, http.StatusOK, "admin reads usage")
	if n, _ := usage.body["items"].(map[string]any)[typeID].(float64); n != 2 {
		t.Fatalf("two live items of the type: %v", usage.body)
	}
	f.want(f.call(editor, http.MethodGet, "/item-types/usage", nil), http.StatusForbidden, "non-admins do not see usage")

	// A Space's own usage: anyone who can edit the Space sees its live items per type; a reader and an outsider do not.
	spaceUsage := f.call(editor, http.MethodGet, "/spaces/"+space+"/item-types/usage", nil)
	f.want(spaceUsage, http.StatusOK, "editor reads Space usage")
	if n, _ := spaceUsage.body["items"].(map[string]any)[typeID].(float64); n != 2 {
		t.Fatalf("two live items of the type in the Space: %v", spaceUsage.body)
	}
	reader, outsider := f.person("regular"), f.person("regular")
	f.share(space, "user", reader.id, "read")
	f.want(f.call(reader, http.MethodGet, "/spaces/"+space+"/item-types/usage", nil), http.StatusForbidden, "a reader does not see Space usage")
	f.want(f.call(outsider, http.MethodGet, "/spaces/"+space+"/item-types/usage", nil), http.StatusNotFound, "an outsider does not see the Space")
}
