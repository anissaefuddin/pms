package httpserver_test

import (
	"net/http"
	"testing"
)

// S-1 (LENS RBAC closure): only an admin or regular user may hold the explicit Space Admin role.
// External, collaborator and viewer users are refused both as a new admin and as the new owner of a transfer.
// Ordinary shares (editor, limited) keep working for those user types.
func TestSpaceAdminRequiresAdminOrRegularUserType(t *testing.T) {
	f := newFixture(t)
	owner := f.person("regular")
	regular := f.person("regular")
	external := f.person("external")
	collaborator := f.person("collaborator")
	viewer := f.person("viewer")
	space := f.space("Governed")
	f.share(space, "user", owner.id, "full")
	f.exec("INSERT INTO space_admins(space_id,user_id) VALUES ($1,$2)", space, owner.id)
	f.share(space, "user", regular.id, "full")
	f.share(space, "user", external.id, "editor")
	f.share(space, "user", collaborator.id, "limited")
	f.share(space, "user", viewer.id, "read")

	admins := "/spaces/" + space + "/admins"
	for name, p := range map[string]*person{"external": external, "collaborator": collaborator, "viewer": viewer} {
		f.want(f.call(owner, "POST", admins, map[string]any{"user_id": p.id}), http.StatusUnprocessableEntity, "a "+name+" user cannot become Space Admin")
		if n := f.queryID("SELECT count(*)::text FROM space_admins WHERE space_id=$1 AND user_id=$2", space, p.id); n != "0" {
			t.Fatalf("a refused grant must not write (%s): %s", name, n)
		}
	}
	f.want(f.call(owner, "POST", admins, map[string]any{"user_id": regular.id}), http.StatusNoContent, "a regular member can become Space Admin")

	// Transfer of ownership makes the new owner an admin too: the same rule applies.
	f.want(f.call(owner, "POST", "/spaces/"+space+"/transfer-ownership", map[string]any{"user_id": external.id}), http.StatusUnprocessableEntity, "transfer to an external user")
	f.want(f.call(owner, "POST", "/spaces/"+space+"/transfer-ownership", map[string]any{"user_id": regular.id}), http.StatusNoContent, "transfer to a regular member")

	// Ordinary shares for the same users are not affected: the external editor still sees the Space.
	f.want(f.call(external, "GET", "/spaces/"+space, nil), http.StatusOK, "an external editor still reads the Space")
}
