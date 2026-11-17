package httpserver_test

import (
	"net/http"
	"testing"
)

// Archive, restore, and impact: the impact counts what depends on a field, restore brings an archived field back only when its
// name is still free, and only Space managers (or the admin for Workspace fields) can read the impact or restore.
func TestCustomFieldArchiveRestoreImpact(t *testing.T) {
	f := newFixture(t)
	admin, owner, editor, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	spaceA := f.space("Alpha")
	f.share(spaceA, "user", owner.id, "full")
	f.share(spaceA, "user", editor.id, "editor")

	created := f.call(owner, http.MethodPost, "/spaces/"+spaceA+"/custom-fields", map[string]any{"name": "Budget", "type": "number"})
	f.want(created, 201, "Full role creates a Space field")
	fid := created.body["id"].(string)

	typ := f.call(admin, http.MethodPost, "/item-types", map[string]any{"key": "lc_req", "name": "Lifecycle request", "base": "task", "space_id": spaceA})
	f.want(typ, 201, "a type to attach the field to")
	f.want(f.call(admin, http.MethodPut, "/item-types/"+typ.body["id"].(string)+"/fields/"+fid, map[string]any{}), 200, "attach field to type")

	item := f.call(editor, http.MethodPost, "/items", map[string]any{"type_key": "lc_req", "title": "Req", "locations": []map[string]any{{"space_id": spaceA}}})
	f.want(item, 201, "item in Alpha")
	f.want(f.call(editor, http.MethodPatch, "/items/"+item.body["id"].(string), map[string]any{"fields": map[string]any{fid: 7}}, "If-Match", `"1"`), 200, "value set")

	impact := f.call(owner, http.MethodGet, "/spaces/"+spaceA+"/custom-fields/"+fid+"/impact", nil)
	f.want(impact, 200, "Full role reads impact")
	if impact.body["items"] != float64(1) || impact.body["item_types"] != float64(1) {
		t.Fatalf("impact counts the value and the type attachment: %v", impact.body)
	}
	f.want(f.call(editor, http.MethodGet, "/spaces/"+spaceA+"/custom-fields/"+fid+"/impact", nil), 403, "editor cannot read impact")
	f.want(f.call(outsider, http.MethodGet, "/spaces/"+spaceA+"/custom-fields/"+fid+"/impact", nil), 404, "outsider cannot see the field")

	f.want(f.call(owner, http.MethodDelete, "/spaces/"+spaceA+"/custom-fields/"+fid, nil), 204, "archive")
	f.want(f.call(owner, http.MethodPost, "/spaces/"+spaceA+"/custom-fields/"+fid+"/restore", nil), 200, "restore an archived field")
	restored := f.call(owner, http.MethodGet, "/spaces/"+spaceA+"/custom-fields", nil)
	if got := restored.items(); len(got) != 1 || got[0]["archived_at"] != nil {
		t.Fatalf("the restored field is active again: %v", restored.body)
	}
	f.want(f.call(owner, http.MethodPost, "/spaces/"+spaceA+"/custom-fields/"+fid+"/restore", nil), 200, "restoring an active field is a no-op")

	f.want(f.call(owner, http.MethodDelete, "/spaces/"+spaceA+"/custom-fields/"+fid, nil), 204, "archive again")
	taken := f.call(owner, http.MethodPost, "/spaces/"+spaceA+"/custom-fields", map[string]any{"name": "Budget", "type": "number"})
	f.want(taken, 201, "a new active field takes the name")
	f.want(f.call(owner, http.MethodPost, "/spaces/"+spaceA+"/custom-fields/"+fid+"/restore", nil), 422, "restore refuses a name that is active again")
	f.want(f.call(editor, http.MethodPost, "/spaces/"+spaceA+"/custom-fields/"+fid+"/restore", nil), 403, "editor cannot restore")

	wsField := f.call(admin, http.MethodPost, "/custom-fields", map[string]any{"name": "Cost center", "type": "number"})
	f.want(wsField, 201, "admin creates a Workspace field")
	wsID := wsField.body["id"].(string)
	f.want(f.call(admin, http.MethodGet, "/custom-fields/"+wsID+"/impact", nil), 200, "admin reads Workspace impact")
	f.want(f.call(owner, http.MethodGet, "/custom-fields/"+wsID+"/impact", nil), 403, "Space owner cannot read Workspace impact")
}
