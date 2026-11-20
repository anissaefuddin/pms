package httpserver_test

import (
	"net/http"
	"testing"
)

func TestReplaceItemLocations(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	a, b := f.space("Space A"), f.space("Space B")
	f.share(a, "user", admin.id, "full") // A1.2: the admin places items in both spaces as an explicit member
	f.share(b, "user", admin.id, "full")
	item := customFieldItem(f, admin, a, "Shared item")
	path := "/items/" + item + "/locations"
	loc := func(space string, parent any) map[string]any {
		return map[string]any{"space_id": space, "parent_item_id": parent}
	}
	titles := func(space string) []string {
		res := f.call(admin, http.MethodGet, "/spaces/"+space+"/items", nil)
		f.want(res, http.StatusOK, "list "+space)
		out := []string{}
		for _, it := range res.body["items"].([]any) {
			out = append(out, it.(map[string]any)["title"].(string))
		}
		return out
	}

	got := f.call(admin, http.MethodPut, path, map[string]any{"locations": []any{loc(a, nil), loc(b, nil)}})
	f.want(got, http.StatusOK, "add a second location")
	if n := len(got.body["locations"].([]any)); n != 2 {
		t.Fatalf("locations = %v", got.body["locations"])
	}
	if len(titles(b)) != 1 || len(titles(a)) != 1 {
		t.Fatalf("one item, shown in both Spaces: %v %v", titles(a), titles(b))
	}
	f.want(f.call(admin, http.MethodPut, path, map[string]any{"locations": []any{loc(a, nil), loc(b, nil)}}), http.StatusOK, "same list again changes nothing")
	f.want(f.call(admin, http.MethodPut, path, map[string]any{"locations": []any{loc(a, nil), loc(a, nil)}}), http.StatusUnprocessableEntity, "duplicate")
	f.want(f.call(admin, http.MethodPut, path, map[string]any{"locations": []any{}}), http.StatusUnprocessableEntity, "empty list")

	f.want(f.call(admin, http.MethodPut, path, map[string]any{"locations": []any{loc(b, nil)}}), http.StatusOK, "remove A")
	if len(titles(a)) != 0 || len(titles(b)) != 1 {
		t.Fatalf("removing a location must not delete the item: %v %v", titles(a), titles(b))
	}
	if f.queryID("SELECT count(*)::text FROM items WHERE id=$1 AND deleted_at IS NULL", item) != "1" {
		t.Fatal("item deleted")
	}

	// A location under the item's own descendant is a cycle.
	child := customFieldItem(f, admin, b, "Child")
	f.exec("UPDATE item_locations SET parent_item_id=$1 WHERE item_id=$2", item, child)
	f.exec("UPDATE items SET type='project' WHERE id=$1", item)
	f.want(f.call(admin, http.MethodPut, path, map[string]any{"locations": []any{loc(b, nil), loc(b, child)}}), http.StatusUnprocessableEntity, "cycle")
	if got := f.call(admin, http.MethodGet, "/items/"+item, nil); len(got.body["locations"].([]any)) != 1 {
		t.Fatalf("a refused change must leave the locations alone: %v", got.body["locations"])
	}

	// Without Create in the target Space the location is refused; a stranger cannot edit at all.
	reader := f.person("regular")
	f.want(f.call(reader, http.MethodPut, path, map[string]any{"locations": []any{loc(a, nil)}}), http.StatusNotFound, "no access")
}
