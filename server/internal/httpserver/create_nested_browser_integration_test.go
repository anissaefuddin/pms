package httpserver_test

import (
	"net/http"
	"os"
	"testing"
)

// Live browser run for nested Task creation (Task → Subtask → Nested Subtask) through the Table and Item Detail.
// Set PMS_TEST_BROWSER=1 to run it against the PostgreSQL fixture; it is skipped otherwise.
func TestBrowserCreateNested(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Nested browser")
	f.share(space, "user", admin.id, "full")
	project := f.call(admin, http.MethodPost, "/items", map[string]any{
		"type": "project", "title": "Nested project", "locations": []map[string]any{{"space_id": space}},
	})
	if project.status != 201 {
		t.Fatalf("project: %d %v", project.status, project.body)
	}
	runBrowserFixture(t, f, []string{"create-nested-live.spec.ts"},
		"PMS_FIXTURE_NESTED_ADMIN="+admin.cookie.String(),
		"PMS_FIXTURE_NESTED_SPACE="+space,
	)
}
