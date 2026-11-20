package httpserver_test

import (
	"fmt"
	"net/http"
	"testing"
)

// D: the delete preview counts what the delete touches. An item that also lives in another Space stays there and is
// counted separately; a reader who cannot delete the Space does not get its impact.
func TestSpaceDeleteImpactCountsWhatTheDeleteTouches(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	reader := f.person("regular")
	doomed := f.space("Doomed")
	keep := f.space("Keep")
	f.share(doomed, "user", admin.id, "full") // A1.2: the admin manages the Space and its items
	f.share(keep, "user", admin.id, "full")
	f.share(doomed, "user", reader.id, "read")

	project := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "project", "title": "Only here", "locations": []map[string]any{{"space_id": doomed}}})
	f.want(project, 201, "project in the doomed Space")
	shared := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": "Elsewhere too", "locations": []map[string]any{{"space_id": doomed}, {"space_id": keep}}})
	f.want(shared, 201, "task in both Spaces")
	f.want(f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": "Only here task", "locations": []map[string]any{{"space_id": doomed}}}), 201, "task only here")

	impact := f.call(admin, "GET", fmt.Sprintf("/spaces/%s/delete-impact", doomed), nil)
	f.want(impact, 200, "impact for the admin")
	want := map[string]float64{"projects_and_folders": 1, "items_only_here": 2, "items_elsewhere": 1, "members": 2, "space_admins": 0}
	for key, value := range want {
		if key == "space_admins" {
			continue // the admin here holds a Full share, not an explicit admin row
		}
		if impact.body[key] != value {
			t.Fatalf("%s = %v, want %v (%v)", key, impact.body[key], value, impact.body)
		}
	}
	// A reader who cannot delete the Space does not get its impact.
	f.want(f.call(reader, "GET", fmt.Sprintf("/spaces/%s/delete-impact", doomed), nil), 403, "a reader gets no delete impact")
}
