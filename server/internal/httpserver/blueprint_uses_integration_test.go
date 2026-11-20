package httpserver_test

import "testing"

// SPC-13 follow-up: each successful use is counted in the blueprint summary; a refused use is not counted.
func TestBlueprintUsesAreCounted(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space, other := f.space("Source"), f.space("Target")
	f.share(space, "user", admin.id, "full") // A1.2: the admin saves and uses blueprints in both Spaces
	f.share(other, "user", admin.id, "full")
	root := f.call(admin, "POST", "/items", map[string]any{"type": "project", "title": "Launch", "locations": []map[string]any{{"space_id": space}}})
	f.want(root, 201, "root")
	saved := f.call(admin, "POST", "/spaces/"+space+"/blueprints", map[string]any{"name": "Counted", "source_item_id": root.body["id"]})
	f.want(saved, 201, "save")
	id := saved.body["id"].(string)
	uses := func() float64 {
		t.Helper()
		for _, row := range f.call(admin, "GET", "/spaces/"+space+"/blueprints", nil).items() {
			if row["id"] == id {
				return row["uses"].(float64)
			}
		}
		t.Fatal("the blueprint is listed")
		return -1
	}
	if got := uses(); got != 0 {
		t.Fatalf("a new blueprint has no uses: %v", got)
	}
	f.want(f.call(admin, "POST", "/blueprints/"+id+"/items", map[string]any{"title": "First", "locations": []map[string]any{{"space_id": other}}}), 201, "use 1")
	f.want(f.call(admin, "POST", "/blueprints/"+id+"/items", map[string]any{"title": "Second", "locations": []map[string]any{{"space_id": other}}}), 201, "use 2")
	if got := uses(); got != 2 {
		t.Fatalf("two successful uses are counted: %v", got)
	}
	f.want(f.call(admin, "POST", "/blueprints/"+id+"/items", map[string]any{"locations": []map[string]any{}}), 422, "refused use")
	if got := uses(); got != 2 {
		t.Fatalf("a refused use is not counted: %v", got)
	}
}
