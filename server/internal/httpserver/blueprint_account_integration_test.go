package httpserver_test

import "testing"

// Account blueprints (instance scope): the owner and instance admins may see, use, edit, and delete them; any other user
// gets 404. A Personal Space still lists the owner's account blueprints, and admins still cannot open someone's Personal Space.
func TestAccountBlueprintIsOwnerAndAdminOnly(t *testing.T) {
	f := newFixture(t)
	admin, owner, other := f.person("admin"), f.person("regular"), f.person("regular")
	personal := f.queryID("SELECT id::text FROM spaces WHERE kind='personal' AND owner_id=$1", owner.id)
	shared := f.space("Account test shared")
	f.share(shared, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	f.share(shared, "user", other.id, "editor")

	root := f.call(owner, "POST", "/items", map[string]any{"type": "project", "title": "Owner project", "locations": []map[string]any{{"space_id": personal}}})
	f.want(root, 201, "owner project")
	saved := f.call(owner, "POST", "/spaces/"+personal+"/blueprints", map[string]any{"name": "My kit", "source_item_id": root.body["id"]})
	f.want(saved, 201, "owner saves an account blueprint")
	if saved.body["scope"] != "account" || saved.body["space_id"] != nil || saved.body["owner_id"] != owner.id {
		t.Fatalf("account blueprint summary: %v", saved.body)
	}
	bp := saved.body["id"].(string)
	if n := f.queryID("SELECT count(*)::text FROM blueprints WHERE id=$1 AND space_id IS NULL AND owner_id=$2", bp, owner.id); n != "1" {
		t.Fatalf("account blueprint is not owned by its owner: %s", n)
	}

	// The owner sees it in the Personal Space and in the account list.
	inPersonal := f.call(owner, "GET", "/spaces/"+personal+"/blueprints", nil)
	f.want(inPersonal, 200, "owner lists the Personal Space")
	if len(inPersonal.items()) != 1 {
		t.Fatalf("owner's Personal Space list: %v", inPersonal.body)
	}
	f.want(f.call(owner, "GET", "/account-blueprints", nil), 200, "owner lists account blueprints")

	// Another user cannot see, edit, use, or delete it, and cannot open the owner's Personal Space.
	f.want(f.call(other, "GET", "/blueprints/"+bp, nil), 404, "other cannot read")
	f.want(f.call(other, "PATCH", "/blueprints/"+bp, map[string]any{"version": 1, "name": "Other"}), 404, "other cannot edit")
	f.want(f.call(other, "POST", "/blueprints/"+bp+"/items", map[string]any{"locations": []map[string]any{{"space_id": shared}}}), 404, "other cannot use")
	f.want(f.call(other, "DELETE", "/blueprints/"+bp, nil), 404, "other cannot delete")
	f.want(f.call(other, "GET", "/spaces/"+personal+"/blueprints", nil), 404, "other cannot list the owner's Personal Space")
	if got := f.call(other, "GET", "/account-blueprints", nil); len(got.items()) != 0 {
		t.Fatalf("other sees account blueprints: %v", got.body)
	}

	// An instance admin may read, edit, use, and delete it, and still cannot open the owner's Personal Space.
	f.want(f.call(admin, "GET", "/blueprints/"+bp, nil), 200, "admin reads")
	f.want(f.call(admin, "PATCH", "/blueprints/"+bp, map[string]any{"version": 1, "name": "Renamed by admin"}), 200, "admin edits")
	f.want(f.call(admin, "POST", "/blueprints/"+bp+"/items", map[string]any{"locations": []map[string]any{{"space_id": shared}}}), 201, "admin uses")
	f.want(f.call(admin, "GET", "/spaces/"+personal+"/blueprints", nil), 404, "admin still cannot open the Personal Space")
	f.want(f.call(admin, "DELETE", "/blueprints/"+bp, nil), 204, "admin deletes")
}

// Names are unique per owner for account blueprints: two owners may both have "My kit".
func TestAccountBlueprintNamesAreUniquePerOwner(t *testing.T) {
	f := newFixture(t)
	a, b := f.person("regular"), f.person("regular")
	for _, owner := range []*person{a, b} {
		personal := f.queryID("SELECT id::text FROM spaces WHERE kind='personal' AND owner_id=$1", owner.id)
		root := f.call(owner, "POST", "/items", map[string]any{"type": "task", "title": "Solo", "locations": []map[string]any{{"space_id": personal}}})
		f.want(root, 201, "item")
		f.want(f.call(owner, "POST", "/spaces/"+personal+"/blueprints", map[string]any{"name": "My kit", "source_item_id": root.body["id"]}), 201, "first My kit")
	}
	personal := f.queryID("SELECT id::text FROM spaces WHERE kind='personal' AND owner_id=$1", a.id)
	root := f.call(a, "POST", "/items", map[string]any{"type": "task", "title": "Again", "locations": []map[string]any{{"space_id": personal}}})
	f.want(root, 201, "item")
	f.want(f.call(a, "POST", "/spaces/"+personal+"/blueprints", map[string]any{"name": "my KIT", "source_item_id": root.body["id"]}), 422, "same owner, same name refused")
}
