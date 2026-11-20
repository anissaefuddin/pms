package httpserver_test

import (
	"net/http"
	"testing"
)

// S-5 (LENS RBAC closure): a successful move records one history event for the item, written in the same
// transaction as the move. A refused move, including one that fails after an earlier location was written,
// leaves no event and no location behind.
func TestMoveWritesHistoryOnlyWhenItSucceeds(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	a, b := f.space("Space A"), f.space("Space B")
	f.share(a, "user", admin.id, "full")
	f.share(b, "user", admin.id, "full")
	item := customFieldItem(f, admin, a, "Movable")
	path := "/items/" + item + "/locations"
	loc := func(space string, parent any) map[string]any {
		return map[string]any{"space_id": space, "parent_item_id": parent}
	}
	events := func() string {
		return f.queryID("SELECT count(*)::text FROM activity WHERE item_id=$1 AND action='update' AND changes ? 'locations'", item)
	}

	f.want(f.call(admin, "PUT", path, map[string]any{"locations": []any{loc(b, nil)}}), http.StatusOK, "move A to B")
	if n := events(); n != "1" {
		t.Fatalf("a successful move must write one history event, got %s", n)
	}

	// Refused: an unknown parent is refused (404: it is not visible) after the first location was written in the
	// same request. Neither the event nor the location may remain.
	missing := "00000000-0000-4000-8000-000000000000"
	f.want(f.call(admin, "PUT", path, map[string]any{"locations": []any{loc(a, nil), loc(a, missing)}}), http.StatusNotFound, "unknown parent refused")
	if n := events(); n != "1" {
		t.Fatalf("a refused move must not write a history event, got %s", n)
	}
	if n := f.queryID("SELECT count(*)::text FROM item_locations WHERE item_id=$1 AND space_id=$2 AND deleted_root_id IS NULL", item, a); n != "0" {
		t.Fatalf("a refused move must roll back its earlier location write, got %s", n)
	}
}
