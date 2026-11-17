package httpserver_test

import "testing"

// CIT-D01: a Workspace type is the system administrator's and works in every Space; a Space type belongs to one
// Space, is managed by people who can edit it, and only items created in that Space can use it.
func TestItemTypeScope(t *testing.T) {
	f := newFixture(t)
	admin, editorA, editorB, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular")
	spaceA, spaceB := f.space("Alpha"), f.space("Beta")
	f.share(spaceA, "user", editorA.id, "editor")
	f.share(spaceA, "user", reader.id, "read")
	f.share(spaceB, "user", editorB.id, "editor")

	ws := f.call(admin, "POST", "/item-types", map[string]any{"key": "wsrisk", "name": "Workspace risk", "base": "task"})
	f.want(ws, 201, "admin creates a Workspace type")
	if ws.body["space_id"] != nil {
		t.Fatalf("a Workspace type has no Space: %v", ws.body["space_id"])
	}
	f.want(f.call(editorA, "POST", "/item-types", map[string]any{"key": "wsbad", "name": "Not allowed", "base": "task"}), 403, "a Space editor cannot create a Workspace type")

	local := f.call(editorA, "POST", "/item-types", map[string]any{"key": "alphadeliv", "name": "Alpha deliverable", "base": "task", "space_id": spaceA})
	f.want(local, 201, "a Space editor creates a type in that Space")
	if local.body["space_id"] != spaceA {
		t.Fatalf("the Space type is scoped to its Space: %v", local.body["space_id"])
	}
	localID := local.body["id"].(string)
	f.want(f.call(reader, "POST", "/item-types", map[string]any{"key": "alpharead", "name": "Read only", "base": "task", "space_id": spaceA}), 403, "a reader cannot create a type in the Space")
	f.want(f.call(editorB, "POST", "/item-types", map[string]any{"key": "alphaother", "name": "Other Space", "base": "task", "space_id": spaceA}), 404, "no access to the Space is 404")

	f.want(f.call(editorA, "PATCH", "/item-types/"+localID, map[string]any{"name": "Alpha deliverable v2"}, "If-Match", `"1"`), 200, "the Space editor renames the type")
	f.want(f.call(reader, "PATCH", "/item-types/"+localID, map[string]any{"name": "No"}, "If-Match", `"2"`), 403, "a reader cannot change it")
	f.want(f.call(outsider, "PATCH", "/item-types/"+localID, map[string]any{"name": "No"}, "If-Match", `"2"`), 404, "an outsider cannot see it")
	f.want(f.call(editorA, "PATCH", "/item-types/"+localID, map[string]any{"space_id": spaceB}, "If-Match", `"2"`), 422, "the scope cannot change")

	// Listing: a Space's types show only to people who can see that Space; Workspace types show to everyone.
	keysFor := func(p *person) map[string]bool {
		out := map[string]bool{}
		for _, item := range f.call(p, "GET", "/item-types", nil).body["items"].([]any) {
			out[item.(map[string]any)["key"].(string)] = true
		}
		return out
	}
	if seen := keysFor(editorB); seen["alphadeliv"] || !seen["wsrisk"] {
		t.Fatalf("Beta editor sees Workspace types but not Alpha's: %v", seen)
	}
	if seen := keysFor(admin); !seen["alphadeliv"] {
		t.Fatalf("the administrator sees every type")
	}

	// Items: a Space type works only inside its Space; a Workspace type works anywhere.
	item := func(p *person, space, typeKey string) reply {
		return f.call(p, "POST", "/items", map[string]any{"type": "task", "title": "Item", "type_key": typeKey, "locations": []map[string]any{{"space_id": space}}})
	}
	f.want(item(editorA, spaceA, "alphadeliv"), 201, "an item in its own Space uses the Space type")
	f.want(item(admin, spaceB, "alphadeliv"), 422, "an item in another Space cannot use the Space type")
	f.want(item(admin, spaceB, "wsrisk"), 201, "any Space can use a Workspace type")
}
