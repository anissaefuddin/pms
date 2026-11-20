package httpserver_test

import "testing"

// B3/B5: blueprint rights are separate. use_blueprint creates items from a blueprint, create_blueprint saves one,
// manage_blueprint deletes one. Built-in roles keep what they had; custom roles get exactly the bits they hold.

const (
	bitView, bitCreate, bitEdit        = 1, 32, 16
	bitUseBP, bitCreateBP, bitManageBP = 256, 512, 1024
)

// customRoleWith makes a custom role with the given mask and gives the user a read share on the Space with it.
func customRoleWith(f *fixture, name string, mask int, p *person, space string) {
	f.t.Helper()
	f.exec("INSERT INTO custom_roles(name,caps_mask) VALUES($1,$2)", name, mask)
	roleID := f.queryID("SELECT id::text FROM custom_roles WHERE name=$1", name)
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,custom_role_id) VALUES('space',$1,'user',$2,'read',$3)", space, p.id, roleID)
}

func TestBlueprintRightsAreSeparate(t *testing.T) {
	f := newFixture(t)
	editor := f.person("regular")
	source, target := f.space("Source"), f.space("Target")
	f.share(source, "user", editor.id, "editor")
	f.share(target, "user", editor.id, "editor")
	root := f.call(editor, "POST", "/items", map[string]any{"type": "project", "title": "Kit", "locations": []map[string]any{{"space_id": source}}})
	f.want(root, 201, "root")
	saved := f.call(editor, "POST", "/spaces/"+source+"/blueprints", map[string]any{"name": "Kit BP", "source_item_id": root.body["id"]})
	f.want(saved, 201, "editor saves")
	bp := saved.body["id"].(string)
	use := func(p *person, title string) reply {
		return f.call(p, "POST", "/blueprints/"+bp+"/items", map[string]any{"title": title, "locations": []map[string]any{{"space_id": target}}})
	}

	// Use-only holder: needs create in the target Space to create items there, and use on the source blueprint.
	useOnly := f.person("regular")
	f.share(target, "user", useOnly.id, "editor")
	customRoleWith(f, "use only 2", bitView|bitUseBP, useOnly, source)
	f.want(use(useOnly, "Via use right"), 201, "use right creates items")
	f.want(f.call(useOnly, "POST", "/spaces/"+source+"/blueprints", map[string]any{"name": "Nope", "source_item_id": root.body["id"]}), 403, "use right cannot save")
	f.want(f.call(useOnly, "DELETE", "/blueprints/"+bp, nil), 403, "use right cannot delete")

	// Save-only holder: may save, may not use or delete.
	saveOnly := f.person("regular")
	f.share(target, "user", saveOnly.id, "editor")
	customRoleWith(f, "save only", bitView|bitCreateBP, saveOnly, source)
	f.want(f.call(saveOnly, "POST", "/spaces/"+source+"/blueprints", map[string]any{"name": "Saved by holder", "source_item_id": root.body["id"]}), 201, "create right saves")
	f.want(use(saveOnly, "No use"), 403, "create right cannot use")
	f.want(f.call(saveOnly, "DELETE", "/blueprints/"+bp, nil), 403, "create right cannot delete")

	// Manage-only holder: may delete, may not save or use.
	manageOnly := f.person("regular")
	customRoleWith(f, "manage only", bitView|bitManageBP, manageOnly, source)
	f.want(f.call(manageOnly, "POST", "/spaces/"+source+"/blueprints", map[string]any{"name": "No save", "source_item_id": root.body["id"]}), 403, "manage right cannot save")
	f.want(f.call(manageOnly, "DELETE", "/blueprints/"+bp, nil), 204, "manage right deletes")

	// A reader still cannot use or save; an admin still can everything.
	reader := f.person("regular")
	f.share(source, "user", reader.id, "read")
	f.want(f.call(reader, "POST", "/spaces/"+source+"/blueprints", map[string]any{"name": "No", "source_item_id": root.body["id"]}), 403, "reader cannot save")
}

// Built-in Editor keeps save, use and delete (the preset carries all three new rights).
func TestEditorPresetKeepsBlueprintRights(t *testing.T) {
	f := newFixture(t)
	editor := f.person("regular")
	source, target := f.space("Preset source"), f.space("Preset target")
	f.share(source, "user", editor.id, "editor")
	f.share(target, "user", editor.id, "editor")
	root := f.call(editor, "POST", "/items", map[string]any{"type": "project", "title": "Preset", "locations": []map[string]any{{"space_id": source}}})
	f.want(root, 201, "root")
	saved := f.call(editor, "POST", "/spaces/"+source+"/blueprints", map[string]any{"name": "Preset BP", "source_item_id": root.body["id"]})
	f.want(saved, 201, "save")
	bp := saved.body["id"].(string)
	f.want(f.call(editor, "POST", "/blueprints/"+bp+"/items", map[string]any{"locations": []map[string]any{{"space_id": target}}}), 201, "use")
	f.want(f.call(editor, "DELETE", "/blueprints/"+bp, nil), 204, "delete")
}

// Scope: a blueprint in a shared Space is 'space'; one saved in the owner's Personal Space is 'account'.
func TestBlueprintScopeFollowsTheSpaceKind(t *testing.T) {
	f := newFixture(t)
	editor := f.person("regular")
	shared := f.space("Shared scope")
	f.share(shared, "user", editor.id, "editor")
	personal := f.queryID("SELECT id::text FROM spaces WHERE kind='personal' AND owner_id=$1", editor.id)
	for _, c := range []struct{ space, want string }{{shared, "space"}, {personal, "account"}} {
		root := f.call(editor, "POST", "/items", map[string]any{"type": "task", "title": "Scoped " + c.want, "locations": []map[string]any{{"space_id": c.space}}})
		f.want(root, 201, "root")
		saved := f.call(editor, "POST", "/spaces/"+c.space+"/blueprints", map[string]any{"name": "Scoped " + c.want, "source_item_id": root.body["id"]})
		f.want(saved, 201, "save")
		if saved.body["scope"] != c.want {
			t.Fatalf("scope = %v, want %s", saved.body["scope"], c.want)
		}
	}
	if n := f.queryID("SELECT count(*)::text FROM blueprints WHERE scope='space' AND space_id=$1", shared); n != "1" {
		t.Fatalf("shared blueprint scope rows = %s", n)
	}
}

// Form routing: an author without use_blueprint may not route to a blueprint; an author with it may, and a member
// without any rights can submit and get the blueprint applied in the form's Space.
func TestFormRoutingNeedsTheUseRightAndSubmissionAppliesIt(t *testing.T) {
	f := newFixture(t)
	admin, editor, author, requester := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	intake, work := f.space("Route intake"), f.space("Route work")
	f.share(intake, "user", editor.id, "editor")
	f.share(work, "user", editor.id, "editor")
	f.share(intake, "user", author.id, "editor")
	f.want(f.call(admin, "POST", "/item-types", map[string]any{"key": "bug_route", "name": "Bug", "base": "task"}), 201, "type")
	root := f.call(editor, "POST", "/items", map[string]any{"type": "task", "title": "Template", "locations": []map[string]any{{"space_id": work}}})
	f.want(root, 201, "template root")
	child := f.call(editor, "POST", "/items", map[string]any{"type": "task", "title": "Template child", "locations": []map[string]any{{"space_id": work, "parent_item_id": root.body["id"]}}})
	f.want(child, 201, "template child")
	bp := f.call(editor, "POST", "/spaces/"+work+"/blueprints", map[string]any{"name": "Route BP", "source_item_id": root.body["id"]})
	f.want(bp, 201, "save blueprint")
	bpID := bp.body["id"].(string)
	progress := f.queryID("SELECT id::text FROM statuses WHERE name='In Progress' AND workflow_id=(SELECT id FROM workflows WHERE is_default)")

	form := func(p *person, bp string) reply {
		definition := map[string]any{"pages": []map[string]any{{"title": "One", "elements": []map[string]any{
			{"key": "kind", "kind": "single_choice", "label": "Kind", "required": true, "options": []string{"Bug", "Idea"}},
		}}}}
		response := map[string]any{"item_type": "task", "status_id": progress, "location": map[string]any{"space_id": work},
			"routes": []map[string]any{{"when": map[string]any{"key": "kind", "op": "equals", "value": "Bug"}, "type_key": "bug_route", "blueprint_id": bp}}}
		return f.call(p, "POST", "/spaces/"+intake+"/forms", map[string]any{"name": "Route form", "definition": definition, "response": response})
	}

	// Author with create but without use: refused.
	f.exec("INSERT INTO custom_roles(name,caps_mask) VALUES('create no use',$1)", bitView|bitCreate)
	roleID := f.queryID("SELECT id::text FROM custom_roles WHERE name='create no use'")
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,custom_role_id) VALUES('space',$1,'user',$2,'read',$3)", work, author.id, roleID)
	f.want(form(author, bpID), 422, "author without use right cannot route to a blueprint")

	// Editor (has use): allowed. Then a requester with no rights submits and gets the blueprint applied.
	saved := form(editor, bpID)
	f.want(saved, 201, "editor routes to the blueprint")
	formID := saved.body["id"].(string)
	f.publishForm(editor, formID)
	f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", formID)
	f.want(f.submit(requester, formID, map[string]any{"kind": "Bug"}), 201, "requester submits")
	if n := f.queryID("SELECT count(*)::text FROM items WHERE title='Template child' AND created_by=$1 AND id IN (SELECT item_id FROM item_locations WHERE space_id=$2)", requester.id, work); n != "1" {
		t.Fatalf("the blueprint child was not created for the submission: %s", n)
	}
}
