package httpserver_test

import "testing"

// D: a locked Space is protected: it cannot be deleted until an administrator unlocks it.
func TestLockedSpaceCannotBeDeletedWhileLocked(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	owner := f.person("regular")
	space := f.space("Protected")
	f.share(space, "user", owner.id, "full")
	f.share(space, "user", admin.id, "full") // A1.2: the admin locks and unlocks the Space as its explicit member
	f.lock(admin, space, true)
	f.want(f.call(owner, "DELETE", "/spaces/"+space, nil), 422, "a locked space is not deleted")
	f.lock(admin, space, false)
	f.want(f.call(owner, "DELETE", "/spaces/"+space, nil), 204, "after unlocking it can be deleted")
}
