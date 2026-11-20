package httpserver_test

import (
	"net/http"
	"testing"
)

// Move regression for parent and child items (LENS RBAC closure). Name matches the CI authorization pattern ('Move').
// Covers inheritance after a move, source and destination checks, cycle refusal, and rollback when a later
// location fails.
func TestMoveParentChildInheritanceValidationCycleAndRollback(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.spaceWithoutAdmins("Tree")
	f.share(space, "user", admin.id, "full")

	parent := customFieldItem(f, admin, space, "Parent")
	child := customFieldItem(f, admin, space, "Child")
	target := customFieldItem(f, admin, space, "Target")
	path := func(item string) string { return "/items/" + item + "/locations" }
	place := func(who *person, item string, parentID any) reply {
		return f.call(who, "PUT", path(item), map[string]any{
			"locations": []any{map[string]any{"space_id": space, "parent_item_id": parentID}},
		})
	}
	itemShare := func(item string, p *person, role string) {
		f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,'user',$2,$3::share_role)", item, p.id, role)
	}
	parentOf := func(item string) string {
		return f.queryID("SELECT COALESCE(parent_item_id::text,'') FROM item_locations WHERE item_id=$1 AND deleted_root_id IS NULL", item)
	}

	f.want(place(admin, child, parent), http.StatusOK, "child under parent")

	t.Run("inheritance follows the new parent after a move", func(t *testing.T) {
		reader := f.person("regular")
		itemShare(target, reader, "read")
		if got := f.call(reader, "GET", "/items/"+child, nil); got.status != http.StatusNotFound {
			t.Fatalf("a reader of the target must not see the child under the parent yet: %d", got.status)
		}
		f.want(place(admin, child, target), http.StatusOK, "admin moves the child under the target")
		f.want(f.call(reader, "GET", "/items/"+child, nil), http.StatusOK, "the child is inherited from the new parent")
		if got := f.call(reader, "GET", "/items/"+parent, nil); got.status != http.StatusNotFound {
			t.Fatalf("a share on the new parent must not expose the old parent: %d", got.status)
		}
		// Removing the share on the new parent takes the inherited access away at once.
		f.exec("DELETE FROM shares WHERE target_type='item' AND target_id=$1 AND principal_id=$2", target, reader.id)
		f.want(f.call(reader, "GET", "/items/"+child, nil), http.StatusNotFound, "inheritance ends with the share")
		f.want(place(admin, child, parent), http.StatusOK, "admin moves the child back")
	})

	sibling := customFieldItem(f, admin, space, "Sibling")
	f.want(place(admin, sibling, parent), http.StatusOK, "sibling under parent")

	t.Run("source: a mover without edit on the source parent is refused", func(t *testing.T) {
		mover := f.person("regular")
		itemShare(parent, mover, "read")   // sees the sibling through the source parent, cannot edit there
		itemShare(target, mover, "editor") // could create under the destination
		f.want(place(mover, sibling, target), http.StatusForbidden, "no edit on the source parent")
		if got := parentOf(sibling); got != parent {
			t.Fatalf("a refused move must keep the source parent: %s", got)
		}
	})

	t.Run("destination: a mover without create on the destination is refused", func(t *testing.T) {
		mover := f.person("regular")
		itemShare(parent, mover, "editor") // may edit the source
		itemShare(target, mover, "read")   // may see the destination only
		f.want(place(mover, sibling, target), http.StatusForbidden, "no create on the destination")
		if got := parentOf(sibling); got != parent {
			t.Fatalf("a refused move must keep the parent: %s", got)
		}
	})

	t.Run("a cycle is refused and nothing is written", func(t *testing.T) {
		f.want(place(admin, parent, sibling), http.StatusUnprocessableEntity, "parent under its own descendant")
		if got := parentOf(parent); got != "" {
			t.Fatalf("a refused cycle must leave the parent at the top: %q", got)
		}
	})

	t.Run("rollback: a later location failure undoes the earlier write", func(t *testing.T) {
		missing := "00000000-0000-4000-8000-000000000000"
		f.want(f.call(admin, "PUT", path(sibling), map[string]any{"locations": []any{
			map[string]any{"space_id": space, "parent_item_id": target},
			map[string]any{"space_id": space, "parent_item_id": missing},
		}}), http.StatusNotFound, "second location refused")
		if got := parentOf(sibling); got != parent {
			t.Fatalf("the first location of a refused move must be rolled back: parent %q", got)
		}
		if n := f.queryID("SELECT count(*)::text FROM item_locations WHERE item_id=$1 AND parent_item_id=$2 AND deleted_root_id IS NULL", sibling, target); n != "0" {
			t.Fatalf("no location under the target may remain: %s", n)
		}
	})

	t.Run("cross-space: a child moves into another Space only where the mover can create", func(t *testing.T) {
		other := f.spaceWithoutAdmins("Other")
		f.share(other, "user", admin.id, "full")
		otherParent := customFieldItem(f, admin, other, "Other parent")
		f.want(f.call(admin, "PUT", path(sibling), map[string]any{"locations": []any{
			map[string]any{"space_id": other, "parent_item_id": otherParent},
		}}), http.StatusOK, "admin with create in the other Space")
		if n := f.queryID("SELECT count(*)::text FROM item_locations WHERE item_id=$1 AND space_id=$2 AND deleted_root_id IS NULL", sibling, other); n != "1" {
			t.Fatalf("the cross-space move must be recorded once: %s", n)
		}
	})
}
