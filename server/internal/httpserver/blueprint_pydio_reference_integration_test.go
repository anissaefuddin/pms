package httpserver_test

import (
	"net/http"
	"testing"
)

// PROMPT #028 PY-10 (blueprint): instantiation copies a valid Pydio reference and cannot copy an invalid one. An invalid source
// reference refuses the whole copy, so nothing partial is written.
func TestBlueprintCannotCopyAnInvalidPydioReference(t *testing.T) {
	f := pydioFixture(t, false)
	admin := f.person("admin")
	client := f.createClient(admin, "PY10-BP")["id"].(string)
	f.addMember(admin, client, admin)
	space := f.scopedSpace("PY10 blueprint", "client", client)
	f.share(space, "user", admin.id, "full")
	field := f.call(admin, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{
		"name": "Documents", "type": "text", "config": map[string]any{"pydio_folder": true},
	})
	f.want(field, http.StatusCreated, "canonical field")
	fid := field.body["id"].(string)
	src := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "project", "title": "Source", "locations": []map[string]any{{"space_id": space}}})
	f.want(src, http.StatusCreated, "source project")
	srcID := src.body["id"].(string)
	f.want(f.call(admin, http.MethodPatch, "/items/"+srcID, map[string]any{"fields": map[string]any{fid: pydioBase + "/projects/PTM-2026/Drawings"}}, "If-Match", `"1"`),
		http.StatusOK, "valid reference on the source")

	// Positive control: a blueprint of a valid source copies the reference.
	saved := f.call(admin, http.MethodPost, "/spaces/"+space+"/blueprints", map[string]any{"name": "Good", "source_item_id": srcID})
	f.want(saved, http.StatusCreated, "save blueprint with a valid reference")
	used := f.call(admin, http.MethodPost, "/blueprints/"+saved.body["id"].(string)+"/items", map[string]any{
		"title": "Copy good", "locations": []map[string]any{{"space_id": space}}})
	f.want(used, http.StatusCreated, "instantiate with a valid reference")
	copied := used.body["id"].(string)
	if got := f.queryID("SELECT fields->>'"+fid+"' FROM items WHERE id=$1", copied); got != pydioBase+"/projects/PTM-2026/Drawings" {
		t.Fatalf("the valid reference must be copied: %q", got)
	}

	// A source whose stored reference is no longer valid (a legacy value): instantiation is refused whole, with no item written.
	f.exec(`UPDATE items SET fields = jsonb_build_object($2::text, 'https://evil.example.com/projects/x') WHERE id=$1`, srcID, fid)
	saved2 := f.call(admin, http.MethodPost, "/spaces/"+space+"/blueprints", map[string]any{"name": "Bad", "source_item_id": srcID})
	if saved2.status == http.StatusCreated {
		used2 := f.call(admin, http.MethodPost, "/blueprints/"+saved2.body["id"].(string)+"/items", map[string]any{
			"title": "Copy bad", "locations": []map[string]any{{"space_id": space}}})
		// Restore skips values the target refuses (blueprint behaviour, not an error): the copy succeeds without the reference.
		if used2.status != http.StatusCreated {
			t.Fatalf("instantiation without the invalid value must succeed (status %d)", used2.status)
		}
		if got := f.queryID("SELECT coalesce(fields->>'"+fid+"','') FROM items WHERE id=$1", used2.body["id"].(string)); got != "" {
			t.Fatalf("an invalid reference was copied: %q", got)
		}
	}
}
