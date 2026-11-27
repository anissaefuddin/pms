package httpserver_test

import (
	"net/http"
	"testing"
)

// Assignments (CF-D02, DR-07, DR-08): a Workspace field applies in a Space only once assigned; a project assignment narrows a
// field to its subtree; removing an assignment hides values but keeps them stored; only Full-role members assign.
func TestCustomFieldAssignments(t *testing.T) {
	f := newFixture(t)
	admin, owner, editor, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Assign space")
	other := f.space("Other space")
	f.share(space, "user", owner.id, "full")
	f.share(space, "user", editor.id, "editor")
	f.share(other, "user", owner.id, "full")

	ws := f.call(admin, http.MethodPost, "/custom-fields", map[string]any{"name": "Region", "type": "text"})
	f.want(ws, 201, "Workspace field")
	wsID := ws.body["id"].(string)
	local := f.call(owner, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{"name": "Phase", "type": "text"})
	f.want(local, 201, "Space field")
	localID := local.body["id"].(string)

	project := f.queryID("INSERT INTO items(type,title,created_by,scope_state) VALUES ('project','Launch',$1,'internal') RETURNING id::text", owner.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, space)
	task := f.queryID("INSERT INTO items(type,title,created_by,scope_state) VALUES ('task','Inside',$1,'internal') RETURNING id::text", owner.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'U')", task, space, project)
	outside := f.queryID("INSERT INTO items(type,title,created_by,scope_state) VALUES ('task','Outside',$1,'internal') RETURNING id::text", owner.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'V')", outside, space)
	f.exec("UPDATE items SET fields=jsonb_build_object($2::text,'north',$3::text,'phase one') WHERE id IN ($1,$4)", task, wsID, localID, outside)

	// Workspace field: hidden until the Space is assigned (DR-08); stored values are not deleted.
	before := f.call(editor, http.MethodGet, "/items/"+outside, nil)
	if values, _ := before.body["fields"].(map[string]any); values[wsID] != nil {
		t.Fatalf("an unassigned Workspace field is hidden in the Space: %v", before.body["fields"])
	}
	f.want(f.call(editor, http.MethodPost, "/spaces/"+space+"/custom-fields/"+wsID+"/assignments", map[string]any{"scope_kind": "space", "scope_id": space}), 403, "editor cannot assign")
	f.want(f.call(outsider, http.MethodPost, "/spaces/"+space+"/custom-fields/"+wsID+"/assignments", map[string]any{"scope_kind": "space", "scope_id": space}), 404, "outsider sees nothing")
	f.want(f.call(owner, http.MethodPost, "/spaces/"+space+"/custom-fields/"+wsID+"/assignments", map[string]any{"scope_kind": "space", "scope_id": other}), 422, "a Space assignment names its own Space")
	f.want(f.call(owner, http.MethodPost, "/spaces/"+space+"/custom-fields/"+localID+"/assignments", map[string]any{"scope_kind": "space", "scope_id": space}), 422, "a Space field already belongs to its Space")

	assigned := f.call(owner, http.MethodPost, "/spaces/"+space+"/custom-fields/"+wsID+"/assignments", map[string]any{"scope_kind": "space", "scope_id": space})
	f.want(assigned, 201, "Full role assigns the Workspace field")
	after := f.call(editor, http.MethodGet, "/items/"+outside, nil)
	if values, _ := after.body["fields"].(map[string]any); values[wsID] != "north" {
		t.Fatalf("an assigned Workspace field shows its value: %v", after.body["fields"])
	}
	// The Space's field list (what views render) includes the assigned Workspace field for any viewer.
	ids := func(list []map[string]any) map[string]bool {
		out := map[string]bool{}
		for _, field := range list {
			out[field["id"].(string)] = true
		}
		return out
	}
	if got := ids(f.call(editor, http.MethodGet, "/spaces/"+space+"/fields", nil).items()); !got[wsID] || !got[localID] {
		t.Fatalf("the Space field list has its own and assigned fields: %v", got)
	}
	choices := f.call(owner, http.MethodGet, "/spaces/"+space+"/field-choices", nil)
	if got := choices.items(); len(got) != 1 || got[0]["assigned"] != true {
		t.Fatalf("the picker shows the assignment: %v", choices.body)
	}
	f.want(f.call(editor, http.MethodGet, "/spaces/"+space+"/field-choices", nil), 403, "only Full members see the Workspace field choices")

	// Project assignment: the Space field applies inside the project only.
	f.want(f.call(owner, http.MethodPost, "/spaces/"+space+"/custom-fields/"+localID+"/assignments", map[string]any{"scope_kind": "project", "scope_id": outside}), 422, "a scope must be a project or folder")
	f.want(f.call(owner, http.MethodPost, "/spaces/"+space+"/custom-fields/"+localID+"/assignments", map[string]any{"scope_kind": "project", "scope_id": project}), 201, "assign the Space field to a project")
	inside := f.call(editor, http.MethodGet, "/items/"+task, nil)
	if values, _ := inside.body["fields"].(map[string]any); values[localID] != "phase one" {
		t.Fatalf("an item inside the project sees the field: %v", inside.body["fields"])
	}
	outsideNow := f.call(editor, http.MethodGet, "/items/"+outside, nil)
	if values, _ := outsideNow.body["fields"].(map[string]any); values[localID] != nil {
		t.Fatalf("an item outside the project does not see the field: %v", outsideNow.body["fields"])
	}
	f.want(f.call(editor, http.MethodPatch, "/items/"+outside, map[string]any{"fields": map[string]any{localID: "x"}}, "If-Match", `"1"`), 422, "no write outside the project")

	// A filter cannot reveal values outside the project either.
	filter := f.call(editor, http.MethodGet, "/items/"+project+"/children?field="+localID+":phase%20one", nil)
	titles := map[string]bool{}
	for _, it := range filter.items() {
		titles[it["title"].(string)] = true
	}
	if !titles["Inside"] {
		t.Fatalf("the filter finds the item where the field applies: %v", titles)
	}
	rootFilter := f.call(editor, http.MethodGet, "/spaces/"+space+"/items?field="+localID+":phase%20one", nil)
	for _, it := range rootFilter.items() {
		if it["title"] == "Outside" {
			t.Fatal("the filter never matches an item where the field does not apply")
		}
	}

	// Removing the project assignment hides the value again but keeps it stored.
	f.want(f.call(owner, http.MethodDelete, "/spaces/"+space+"/custom-fields/"+localID+"/assignments/project/"+project, nil), 204, "remove project assignment")
	gone := f.call(editor, http.MethodGet, "/items/"+task, nil)
	if values, _ := gone.body["fields"].(map[string]any); values[localID] != "phase one" {
		t.Fatalf("with no project assignment left the field applies to the whole Space again: %v", gone.body["fields"])
	}
	if f.queryID("SELECT fields->>$2 FROM items WHERE id=$1", task, localID) != "phase one" {
		t.Fatal("the stored value survives the assignment removal")
	}

	// Removing the Space assignment of the Workspace field hides its values in that Space.
	f.want(f.call(owner, http.MethodDelete, "/spaces/"+space+"/custom-fields/"+wsID+"/assignments/space/"+space, nil), 204, "remove Space assignment")
	if got := ids(f.call(editor, http.MethodGet, "/spaces/"+space+"/fields", nil).items()); got[wsID] {
		t.Fatalf("an unassigned Workspace field is not in the Space's field list: %v", got)
	}
	hidden := f.call(editor, http.MethodGet, "/items/"+outside, nil)
	if values, _ := hidden.body["fields"].(map[string]any); values[wsID] != nil {
		t.Fatalf("a removed Space assignment hides the Workspace field: %v", hidden.body["fields"])
	}
	if f.queryID("SELECT fields->>$2 FROM items WHERE id=$1", outside, wsID) != "north" {
		t.Fatal("the Workspace value survives the removal")
	}
}
