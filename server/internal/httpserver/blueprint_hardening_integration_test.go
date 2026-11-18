package httpserver_test

import (
	"net/http"
	"strings"
	"testing"
)

// Blueprint hardening (BP-B0/B1, see docs/BLUEPRINT_ANALISA.md): cross-Space copies (X1), restricted fields (X2),
// hierarchy limits without silent truncation (X3), invalid stored trees (no partial mutation), audit events, and finish_date (X4).

func hardeningSetup(t *testing.T, f *fixture) (admin, alice, bob *person, source, library, target string) {
	t.Helper()
	admin, alice, bob = f.person("admin"), f.person("regular"), f.person("regular")
	source, library, target = f.space("Source"), f.space("Library"), f.space("Target")
	f.share(source, "user", alice.id, "editor")
	f.share(library, "user", alice.id, "editor")
	f.share(library, "user", bob.id, "read")
	f.share(target, "user", bob.id, "editor")
	return
}

// X1: a copy must not carry content from a Space the blueprint's Space does not own (the item must sit in the target Space).
func TestBlueprintSaveRejectsItemsOutsideTargetSpace(t *testing.T) {
	f := newFixture(t)
	_, alice, bob, source, library, target := hardeningSetup(t, f)
	item := f.call(alice, "POST", "/items", map[string]any{"type": "project", "title": "Acquisition", "locations": []map[string]any{{"space_id": source}}})
	f.want(item, 201, "item in source")
	id := item.body["id"].(string)
	f.exec(`UPDATE items SET description='{"type":"doc","content":[]}', description_text='confidential' WHERE id=$1`, id)

	f.want(f.call(alice, "POST", "/spaces/"+library+"/blueprints", map[string]any{"name": "Leak", "source_item_id": id}), 422, "item from another Space")
	if n := f.queryID("SELECT count(*)::text FROM blueprints WHERE space_id=$1", library); n != "0" {
		t.Fatalf("a refused copy left a blueprint behind: %s", n)
	}
	// Bob reads the Library but cannot see Source; he gets nothing from it.
	f.want(f.call(bob, "GET", "/spaces/"+library+"/blueprints", nil), 200, "bob lists library")
	_ = target
}

// X2: a restricted field's value is never copied into a blueprint, and an old tree that still holds one does not restore it.
func TestBlueprintNeverCopiesRestrictedFields(t *testing.T) {
	f := newFixture(t)
	admin, alice, _, source, _, _ := hardeningSetup(t, f)
	owner := f.person("regular")
	f.share(source, "user", owner.id, "full")

	salary := f.call(owner, http.MethodPost, "/spaces/"+source+"/custom-fields", map[string]any{"name": "Salary", "type": "number"})
	f.want(salary, 201, "Salary field")
	sid := salary.body["id"].(string)
	f.want(f.call(owner, http.MethodPut, "/spaces/"+source+"/custom-fields/"+sid+"/grants",
		map[string]any{"grants": []map[string]any{{"subject_kind": "user", "subject_id": owner.id, "level": "edit"}}}), 200, "restrict Salary")

	item := f.call(owner, "POST", "/items", map[string]any{"type": "project", "title": "Offer", "locations": []map[string]any{{"space_id": source}}})
	f.want(item, 201, "offer")
	id := item.body["id"].(string)
	f.exec("UPDATE items SET fields=jsonb_build_object($2::text,5000) WHERE id=$1", id, sid)

	saved := f.call(alice, "POST", "/spaces/"+source+"/blueprints", map[string]any{"name": "Offer template", "source_item_id": id})
	f.want(saved, 201, "alice saves (no grant on Salary)")
	bpID := saved.body["id"].(string)
	if tree := f.queryID("SELECT tree::text FROM blueprints WHERE id=$1", bpID); strings.Contains(tree, "5000") || strings.Contains(tree, sid) {
		t.Fatalf("restricted value or field id leaked into the stored tree")
	}

	// An old tree that already holds the restricted value must not restore it, even for a user who holds the grant.
	f.exec(`INSERT INTO blueprints(space_id,name,tree,created_by) VALUES($1,'Legacy',
	  jsonb_build_object('nodes', jsonb_build_array(jsonb_build_object('parent',-1,'type','project','title','Legacy','importance','high',
	    'assignee_ids','[]'::jsonb,'fields', jsonb_build_object($2::text, 7777))), 'dependencies','[]'::jsonb), $3)`, source, sid, admin.id)
	legacyID := f.queryID("SELECT id::text FROM blueprints WHERE name='Legacy'")
	used := f.call(owner, "POST", "/blueprints/"+legacyID+"/items", map[string]any{"locations": []map[string]any{{"space_id": source}}})
	f.want(used, 201, "owner uses legacy blueprint")
	newID := used.body["id"].(string)
	if fields := f.queryID("SELECT fields::text FROM items WHERE id=$1", newID); strings.Contains(fields, sid) {
		t.Fatalf("a restricted field was restored from a blueprint: %s", fields)
	}
}

// X3: a hierarchy deeper than the limit is refused, not silently cut short.
func TestBlueprintRefusesHierarchyBeyondDepthLimit(t *testing.T) {
	f := newFixture(t)
	_, alice, _, source, _, _ := hardeningSetup(t, f)
	root := f.call(alice, "POST", "/items", map[string]any{"type": "project", "title": "Deep", "locations": []map[string]any{{"space_id": source}}})
	f.want(root, 201, "root")
	parent := root.body["id"].(string)
	for i := 0; i < 31; i++ {
		r := f.call(alice, "POST", "/items", map[string]any{"type": "folder", "title": "Level", "locations": []map[string]any{{"space_id": source, "parent_item_id": parent}}})
		f.want(r, 201, "folder level")
		parent = r.body["id"].(string)
	}
	f.want(f.call(alice, "POST", "/spaces/"+source+"/blueprints", map[string]any{"name": "Too deep", "source_item_id": root.body["id"]}), 422, "32 levels")
	if n := f.queryID("SELECT count(*)::text FROM blueprints WHERE name='Too deep'"); n != "0" {
		t.Fatalf("a refused deep copy was stored")
	}
}

// Invalid stored trees are refused before anything is created (no partial mutation, no panic).
func TestBlueprintRefusesInvalidTreeWithoutMutation(t *testing.T) {
	f := newFixture(t)
	_, alice, _, source, _, _ := hardeningSetup(t, f)
	f.exec(`INSERT INTO blueprints(space_id,name,tree,created_by) VALUES($1,'Broken',
	  '{"nodes":[{"parent":-1,"type":"project","title":"Root","importance":"high","assignee_ids":[]},
	             {"parent":5,"type":"task","title":"Orphan","importance":"high","assignee_ids":[]}],"dependencies":[]}'::jsonb, $2)`, source, alice.id)
	id := f.queryID("SELECT id::text FROM blueprints WHERE name='Broken'")
	before := f.queryID("SELECT count(*)::text FROM items")
	f.want(f.call(alice, "POST", "/blueprints/"+id+"/items", map[string]any{"locations": []map[string]any{{"space_id": source}}}), 422, "invalid tree")
	if after := f.queryID("SELECT count(*)::text FROM items"); after != before {
		t.Fatalf("an invalid tree left items behind: %s -> %s", before, after)
	}
}

// Audit: save, use and delete leave an event in the same transaction.
func TestBlueprintWritesAuditEvents(t *testing.T) {
	f := newFixture(t)
	_, alice, _, source, _, _ := hardeningSetup(t, f)
	item := f.call(alice, "POST", "/items", map[string]any{"type": "project", "title": "Audited", "locations": []map[string]any{{"space_id": source}}})
	f.want(item, 201, "item")
	saved := f.call(alice, "POST", "/spaces/"+source+"/blueprints", map[string]any{"name": "Audited bp", "source_item_id": item.body["id"]})
	f.want(saved, 201, "save")
	id := saved.body["id"].(string)
	f.want(f.call(alice, "POST", "/blueprints/"+id+"/items", map[string]any{"locations": []map[string]any{{"space_id": source}}}), 201, "use")
	f.want(f.call(alice, "DELETE", "/blueprints/"+id, nil), 204, "delete")
	for _, action := range []string{"blueprint.save", "blueprint.use", "blueprint.delete"} {
		if n := f.queryID("SELECT count(*)::text FROM audit_events WHERE action=$1 AND actor_id=$2::uuid", action, alice.id); n == "0" {
			t.Fatalf("no audit event for %s", action)
		}
	}
}

// X4: finish_date anchors the tree's last date, so the latest item finishes on that day.
func TestBlueprintUseAnchorsOnFinishDate(t *testing.T) {
	f := newFixture(t)
	_, alice, _, source, _, _ := hardeningSetup(t, f)
	root := f.call(alice, "POST", "/items", map[string]any{"type": "project", "title": "Launch", "locations": []map[string]any{{"space_id": source}}})
	f.want(root, 201, "root")
	rootID := root.body["id"].(string)
	plan := f.call(alice, "POST", "/items", map[string]any{"type": "task", "title": "Plan", "start_date": "2026-03-02", "due_date": "2026-03-04",
		"locations": []map[string]any{{"space_id": source, "parent_item_id": rootID}}})
	f.want(plan, 201, "plan")
	ship := f.call(alice, "POST", "/items", map[string]any{"type": "task", "title": "Ship", "start_date": "2026-03-09", "due_date": "2026-03-10",
		"locations": []map[string]any{{"space_id": source, "parent_item_id": rootID}}})
	f.want(ship, 201, "ship")
	saved := f.call(alice, "POST", "/spaces/"+source+"/blueprints", map[string]any{"name": "Finish test", "source_item_id": rootID})
	f.want(saved, 201, "save")
	id := saved.body["id"].(string)

	f.want(f.call(alice, "POST", "/blueprints/"+id+"/items", map[string]any{"locations": []map[string]any{{"space_id": source}}, "start_date": "2026-06-01", "finish_date": "2026-06-30"}), 422, "start and finish together")
	f.want(f.call(alice, "POST", "/blueprints/"+id+"/items", map[string]any{"locations": []map[string]any{{"space_id": source}}, "finish_date": "2026-06-30"}), 201, "use by finish")
	got := f.queryID("SELECT due_date::text FROM items WHERE title='Ship' AND created_by=$1 AND due_date='2026-06-30'", alice.id)
	if got != "2026-06-30" {
		t.Fatalf("Ship did not finish on the finish date: %q", got)
	}
}
