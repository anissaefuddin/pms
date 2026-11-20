package httpserver_test

import (
	"net/http"
	"os"
	"testing"
)

// R5 live: a Task made in the Board shows the same status, parent, and dates to the Table and to the server data the other views read.
// Set PMS_TEST_BROWSER=1 to run it; it is skipped otherwise.
func TestBrowserCrossView(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Cross view")
	f.share(space, "user", admin.id, "full")
	project := f.call(admin, http.MethodPost, "/items", map[string]any{
		"type": "project", "title": "Cross project", "locations": []map[string]any{{"space_id": space}},
	})
	if project.status != 201 {
		t.Fatalf("project: %d %v", project.status, project.body)
	}
	// An empty Board shows an empty state, not columns. One seed item makes the columns appear for the quick create.
	if seed := f.call(admin, http.MethodPost, "/items", map[string]any{
		"type": "task", "title": "Seed", "locations": []map[string]any{{"space_id": space, "parent_item_id": project.body["id"]}},
	}); seed.status != 201 {
		t.Fatalf("seed: %d %v", seed.status, seed.body)
	}
	runBrowserFixture(t, f, []string{"cross-view-live.spec.ts"},
		"PMS_FIXTURE_XV_ADMIN="+admin.cookie.String(),
		"PMS_FIXTURE_XV_PROJECT="+project.body["id"].(string),
	)
}
