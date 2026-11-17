package httpserver_test

import (
	"net/http"
	"testing"
)

// CIT-D05: Quick Access is a visibility setting. Only custom, active types can have it; the Space's Create menu lists the
// Workspace types and that Space's own types; changing it needs the same rights as editing the type.
func TestItemTypeQuickAccessValidationAndMenu(t *testing.T) {
	f := newFixture(t)
	admin, editorA, editorB := f.person("admin"), f.person("regular"), f.person("regular")
	spaceA, spaceB := f.space("QA Alpha"), f.space("QA Beta")
	f.share(spaceA, "user", editorA.id, "editor")
	f.share(spaceB, "user", editorB.id, "editor")

	ws := f.call(admin, http.MethodPost, "/item-types", map[string]any{"key": "qa_ws", "name": "QA workspace", "base": "task"})
	f.want(ws, http.StatusCreated, "Workspace type")
	wsID := ws.body["id"].(string)
	if ws.body["quick_access"] != false {
		t.Fatalf("Quick Access is off for a new type: %v", ws.body)
	}
	f.want(f.call(admin, http.MethodPatch, "/item-types/"+wsID, map[string]any{"quick_access": true}, "If-Match", `"1"`), http.StatusOK, "admin turns Quick Access on")
	f.want(f.call(admin, http.MethodPatch, "/item-types/"+wsID, map[string]any{"active": false, "quick_access": true}, "If-Match", `"2"`), http.StatusUnprocessableEntity, "an inactive type cannot be in Quick Access")
	f.want(f.call(editorB, http.MethodPatch, "/item-types/"+wsID, map[string]any{"quick_access": false}, "If-Match", `"2"`), http.StatusForbidden, "a Space editor cannot change a Workspace type")

	local := f.call(editorA, http.MethodPost, "/item-types", map[string]any{"key": "qa_local", "name": "QA local", "base": "task", "space_id": spaceA})
	f.want(local, http.StatusCreated, "Space type")
	localID := local.body["id"].(string)
	f.want(f.call(editorA, http.MethodPatch, "/item-types/"+localID, map[string]any{"quick_access": true}, "If-Match", `"1"`), http.StatusOK, "the Space editor turns on its own type's Quick Access")

	menuA := f.call(editorA, http.MethodGet, "/spaces/"+spaceA+"/create-menu", nil)
	f.want(menuA, http.StatusOK, "Alpha create menu")
	quickA := map[string]bool{}
	for _, entry := range menuA.body["items"].([]any) {
		e := entry.(map[string]any)
		if e["quick_access"] == true {
			quickA[e["key"].(string)] = true
		}
	}
	if !quickA["qa_ws"] || !quickA["qa_local"] {
		t.Fatalf("Alpha's menu shows both Quick Access types: %v", menuA.body)
	}
	menuB := f.call(editorB, http.MethodGet, "/spaces/"+spaceB+"/create-menu", nil)
	f.want(menuB, http.StatusOK, "Beta create menu")
	for _, entry := range menuB.body["items"].([]any) {
		if key := entry.(map[string]any)["key"]; key == "qa_local" {
			t.Fatalf("Alpha's Space type is not in Beta's menu: %v", menuB.body)
		}
	}
	builtin := f.queryID("SELECT id::text FROM item_type_defs WHERE key='task' AND is_builtin")
	f.want(f.call(admin, http.MethodPatch, "/item-types/"+builtin, map[string]any{"quick_access": true}, "If-Match", `"1"`), http.StatusUnprocessableEntity, "built-in types have no Quick Access setting")
}
