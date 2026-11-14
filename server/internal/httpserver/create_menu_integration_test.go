package httpserver_test

import "testing"

// SPC-14 / D-99: the "+" menu is a per-Space UI convenience. Edit rights change it, If-Match guards it,
// and hiding a type never stops the API from creating items of that type.
func TestSpaceCreateMenu(t *testing.T) {
	f := newFixture(t)
	editor, viewer, outsider := f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Menu")
	f.share(space, "user", editor.id, "editor")
	f.share(space, "user", viewer.id, "read")
	url := "/spaces/" + space + "/create-menu"

	got := f.call(viewer, "GET", url, nil)
	f.want(got, 200, "viewer reads")
	etag := got.header.Get("ETag")
	var keys []string
	for _, e := range got.items() {
		if e["enabled"] != true {
			t.Fatalf("default menu must offer every type: %v", e)
		}
		keys = append(keys, e["key"].(string))
	}
	if len(keys) < 3 {
		t.Fatalf("keys = %v", keys)
	}
	f.want(f.call(outsider, "GET", url, nil), 404, "outsider read")
	f.want(f.call(outsider, "PUT", url, map[string]any{"enabled": keys}, "If-Match", etag), 404, "outsider write")
	f.want(f.call(viewer, "PUT", url, map[string]any{"enabled": keys}, "If-Match", etag), 403, "viewer write")
	f.want(f.call(editor, "PUT", url, map[string]any{"enabled": keys}), 428, "no If-Match")
	f.want(f.call(editor, "PUT", url, map[string]any{"enabled": []string{"nope"}}, "If-Match", etag), 422, "unknown type")
	f.want(f.call(editor, "PUT", url, map[string]any{"enabled": []string{}}, "If-Match", etag), 422, "empty menu")

	on := keys[1:] // hide the first type
	put := f.call(editor, "PUT", url, map[string]any{"enabled": on}, "If-Match", etag)
	f.want(put, 200, "hide one")
	f.want(f.call(editor, "PUT", url, map[string]any{"enabled": on}, "If-Match", etag), 409, "stale version")
	again := f.call(viewer, "GET", url, nil)
	for i, e := range again.items() {
		if e["enabled"] != (i != 0) {
			t.Fatalf("entry %d = %v", i, e)
		}
	}
	// The menu is not access control: the API still creates an item of the hidden type.
	f.want(f.call(editor, "POST", "/items", map[string]any{"type": keys[0], "title": "still works", "locations": []map[string]any{{"space_id": space}}}), 201, "hidden type via API")
	var n int
	if err := f.pool.QueryRow(t.Context(), "SELECT count(*) FROM audit_events WHERE action='space.create_menu_changed'").Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit rows = %d, %v", n, err)
	}
}
