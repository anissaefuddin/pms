package httpserver_test

import (
	"net/http"
	"testing"
)

// B5-03: the owner filter lists projects by their Project Manager. "none" finds projects without one; a user id finds that
// user's projects. Tasks never match, because only projects have an owner.
func TestOwnerFilterListsProjectsByProjectManager(t *testing.T) {
	f := newFixture(t)
	owner := f.person("regular")
	space := f.call(owner, "POST", "/spaces", map[string]any{"name": "Owner filter", "scope_state": "internal"})
	f.want(space, http.StatusCreated, "the owner creates the internal Space")
	sid := space.body["id"].(string)
	ownedProject := f.call(owner, "POST", "/items", map[string]any{"type": "project", "title": "Owned", "owner_id": owner.id,
		"locations": []map[string]string{{"space_id": sid}}})
	f.want(ownedProject, http.StatusCreated, "a project with a Project Manager")
	loosePro := f.call(owner, "POST", "/items", map[string]any{"type": "project", "title": "Loose", "locations": []map[string]string{{"space_id": sid}}})
	f.want(loosePro, http.StatusCreated, "a project without one")
	task := f.call(owner, "POST", "/items", map[string]any{"type": "task", "title": "Plain task", "locations": []map[string]string{{"space_id": sid}}})
	f.want(task, http.StatusCreated, "a task")

	titles := func(query string) map[string]bool {
		t.Helper()
		r := f.call(owner, "GET", "/spaces/"+sid+"/items?"+query, nil)
		f.want(r, http.StatusOK, "list with "+query)
		out := map[string]bool{}
		for _, row := range r.items() {
			out[row["title"].(string)] = true
		}
		return out
	}
	if got := titles("owner=none"); !got["Loose"] || got["Owned"] || got["Plain task"] {
		t.Fatalf("owner=none must list only the project without a Project Manager: %v", got)
	}
	if got := titles("owner=" + owner.id); !got["Owned"] || got["Loose"] || got["Plain task"] {
		t.Fatalf("a user id must list only that user's projects: %v", got)
	}
	f.want(f.call(owner, "GET", "/spaces/"+sid+"/items?owner=not-a-uuid", nil), http.StatusUnprocessableEntity, "a malformed owner is refused")
}
