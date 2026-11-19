package httpserver_test

import "testing"

// Boundaries and item-level shares for Blueprint save (see blueprints.go).

func TestBlueprintAcceptsExactlyThirtyLevels(t *testing.T) {
	f := newFixture(t)
	_, alice, _, source, _, _ := hardeningSetup(t, f)
	root := f.call(alice, "POST", "/items", map[string]any{"type": "project", "title": "Edge", "locations": []map[string]any{{"space_id": source}}})
	f.want(root, 201, "root")
	parent := root.body["id"].(string)
	for i := 0; i < 30; i++ { // 30 levels below the root is the limit
		r := f.call(alice, "POST", "/items", map[string]any{"type": "folder", "title": "Level", "locations": []map[string]any{{"space_id": source, "parent_item_id": parent}}})
		f.want(r, 201, "folder level")
		parent = r.body["id"].(string)
	}
	saved := f.call(alice, "POST", "/spaces/"+source+"/blueprints", map[string]any{"name": "Thirty", "source_item_id": root.body["id"]})
	f.want(saved, 201, "30 levels below the root")
	if saved.body["item_count"] != float64(31) {
		t.Fatalf("item_count = %v, want 31", saved.body["item_count"])
	}
}

func TestBlueprintRefusesItemsWithTheirOwnShares(t *testing.T) {
	f := newFixture(t)
	_, alice, bob, source, _, _ := hardeningSetup(t, f)
	item := f.call(alice, "POST", "/items", map[string]any{"type": "project", "title": "Shared alone", "locations": []map[string]any{{"space_id": source}}})
	f.want(item, 201, "item")
	id := item.body["id"].(string)
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,created_by) VALUES('item',$1,'user',$2,'read',$3)", id, bob.id, alice.id)
	f.want(f.call(alice, "POST", "/spaces/"+source+"/blueprints", map[string]any{"name": "Shared", "source_item_id": id}), 422, "item with its own share")
	if n := f.queryID("SELECT count(*)::text FROM blueprints WHERE name='Shared'"); n != "0" {
		t.Fatalf("a refused save was stored")
	}
}
