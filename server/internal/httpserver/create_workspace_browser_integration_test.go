package httpserver_test

import (
	"os"
	"testing"
)

// R3 live: Workspace Quick Create in a real browser (Create → Task → name only → Enter). PMS_TEST_BROWSER=1 to run.
func TestBrowserWorkspaceQuickCreate(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	admin := f.person("admin")
	runBrowserFixture(t, f, []string{"create-workspace-live.spec.ts"}, "PMS_FIXTURE_WS_ADMIN="+admin.cookie.String())
}
