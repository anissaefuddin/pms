package httpserver_test

import (
	"net/http"
	"testing"
)

// Audit probe (LENS RBAC audit, move): moving a Task so that it sits under its own descendant Task must be refused.
// The type rule alone (task inside task is allowed) does not stop a loop, so the server has to check the hierarchy.
func TestMoveTaskUnderOwnDescendantIsRefused(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	a := f.space("Space A")
	f.share(a, "user", admin.id, "full")
	parent := customFieldItem(f, admin, a, "Parent task")
	child := customFieldItem(f, admin, a, "Child task")

	// child goes under parent (valid).
	f.want(f.call(admin, http.MethodPut, "/items/"+child+"/locations", map[string]any{
		"locations": []any{map[string]any{"space_id": a, "parent_item_id": parent}},
	}), http.StatusOK, "child under parent")

	// parent goes under its own child: a cycle, must be refused.
	res := f.call(admin, http.MethodPut, "/items/"+parent+"/locations", map[string]any{
		"locations": []any{map[string]any{"space_id": a, "parent_item_id": child}},
	})
	t.Logf("refusal: %v", res.body)
	f.want(res, http.StatusUnprocessableEntity, "parent under its own descendant")

	// Nothing was written: parent still has no parent of its own.
	if n := f.queryID("SELECT count(*)::text FROM item_locations WHERE item_id=$1 AND parent_item_id IS NOT NULL AND deleted_root_id IS NULL", parent); n != "0" {
		t.Fatalf("a refused move must not write a location: %s", n)
	}
}
