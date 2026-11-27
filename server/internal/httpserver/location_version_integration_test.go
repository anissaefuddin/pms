package httpserver_test

import (
	"sort"
	"sync"
	"testing"
)

// LOCK-01: a location edit is an optimistic-concurrency write like PATCH. A stale version is refused before anything changes,
// two writers with the same version cannot both commit, and a successful edit bumps the version.
func TestLocationEditRefusesStaleVersion(t *testing.T) {
	c := newInternalAssignCast(t)
	f := c.f
	task := f.call(c.full, "POST", "/items", map[string]any{"type": "task", "title": "Locked work", "locations": []map[string]string{{"space_id": c.space}}})
	f.want(task, 201, "task")
	id := task.body["id"].(string)
	if v := f.queryID("SELECT version::text FROM items WHERE id=$1", id); v != "1" {
		t.Fatalf("a new item starts at version %s, want 1", v)
	}
	f.want(f.call(c.full, "PATCH", "/items/"+id, map[string]any{"title": "Locked work, edited"}, "If-Match", `"1"`), 200, "edit the title")
	other := f.spaceWithoutAdmins("Second place")
	f.share(other, "user", c.full.id, "full")
	f.want(f.call(c.full, "PUT", "/items/"+id+"/locations", map[string]any{"locations": []map[string]any{{"space_id": c.space}, {"space_id": other}}}, "If-Match", `"1"`),
		409, "a stale version is refused")
	if n := f.queryID("SELECT count(*)::text FROM item_locations WHERE item_id=$1 AND deleted_root_id IS NULL", id); n != "1" {
		t.Fatalf("a refused edit changed the locations: %s rows", n)
	}
	if v := f.queryID("SELECT version::text FROM items WHERE id=$1", id); v != "2" {
		t.Fatalf("a refused edit changed the version: %s", v)
	}
	f.want(f.call(c.full, "PUT", "/items/"+id+"/locations", map[string]any{"locations": []map[string]any{{"space_id": c.space}, {"space_id": other}}}, "If-Match", `"2"`),
		200, "the current version is accepted")
	if v := f.queryID("SELECT version::text FROM items WHERE id=$1", id); v != "3" {
		t.Fatalf("a successful location edit must bump the version: got %s, want 3", v)
	}
}

func TestLocationEditNeedsAVersionHeader(t *testing.T) {
	c := newInternalAssignCast(t)
	f := c.f
	task := f.call(c.full, "POST", "/items", map[string]any{"type": "task", "title": "Header work", "locations": []map[string]string{{"space_id": c.space}}})
	id := task.body["id"].(string)
	body := map[string]any{"locations": []map[string]any{{"space_id": c.space}}}
	f.want(f.call(c.full, "PUT", "/items/"+id+"/locations", body, "If-Match", ""), 428, "no If-Match is refused")
	f.want(f.call(c.full, "PUT", "/items/"+id+"/locations", body, "If-Match", "soon"), 422, "a malformed If-Match is refused")
}

func TestLocationEditConcurrentWritersCannotBothCommit(t *testing.T) {
	c := newInternalAssignCast(t)
	f := c.f
	task := f.call(c.full, "POST", "/items", map[string]any{"type": "task", "title": "Race work", "locations": []map[string]string{{"space_id": c.space}}})
	id := task.body["id"].(string)
	other := f.spaceWithoutAdmins("Race place")
	f.share(other, "user", c.full.id, "full")
	version := f.queryID("SELECT version::text FROM items WHERE id=$1", id)
	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i, want := range [][]map[string]any{{{"space_id": c.space}}, {{"space_id": c.space}, {"space_id": other}}} {
		wg.Add(1)
		go func(i int, want []map[string]any) {
			defer wg.Done()
			statuses[i] = f.call(c.full, "PUT", "/items/"+id+"/locations", map[string]any{"locations": want}, "If-Match", `"`+version+`"`).status
		}(i, want)
	}
	wg.Wait()
	sort.Ints(statuses)
	if statuses[0] != 200 || statuses[1] != 409 {
		t.Fatalf("two writers with version %s: statuses %v, want exactly one 200 and one 409", version, statuses)
	}
	if v := f.queryID("SELECT version::text FROM items WHERE id=$1", id); v != "2" {
		t.Fatalf("the winning edit must bump the version once: got %s", v)
	}
}

// Cross-Customer and read-only actors are refused with the current version, and the refused edit changes nothing.
func TestLocationEditDeniedWithoutRightsChangesNothing(t *testing.T) {
	c := newInternalAssignCast(t)
	f := c.f
	task := f.call(c.full, "POST", "/items", map[string]any{"type": "task", "title": "Guarded work", "locations": []map[string]string{{"space_id": c.space}}})
	id := task.body["id"].(string)
	version := f.queryID("SELECT version::text FROM items WHERE id=$1", id)
	body := map[string]any{"locations": []map[string]any{{"space_id": c.space}, {"space_id": f.spaceWithoutAdmins("Elsewhere")}}}
	f.want(f.call(c.outsiderAdmin, "PUT", "/items/"+id+"/locations", body, "If-Match", `"`+version+`"`), 404, "no grant is not found")
	f.want(f.call(c.read, "PUT", "/items/"+id+"/locations", body, "If-Match", `"`+version+`"`), 403, "read only cannot edit")
	if v := f.queryID("SELECT version::text FROM items WHERE id=$1", id); v != version {
		t.Fatalf("a denied edit changed the version: %s, want %s", v, version)
	}
	if n := f.queryID("SELECT count(*)::text FROM item_locations WHERE item_id=$1 AND deleted_root_id IS NULL", id); n != "1" {
		t.Fatalf("a denied edit changed the locations: %s rows", n)
	}
}
