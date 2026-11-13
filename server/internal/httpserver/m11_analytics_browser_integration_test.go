package httpserver_test

import (
	"os"
	"testing"
)

// TestBrowserM11AnalyticsFlows runs the analytics board and public dashboard link against the real API and PostgreSQL.
func TestBrowserM11AnalyticsFlows(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	admin := f.person("admin")
	f.exec("UPDATE users SET first_name='Ada',last_name='Admin' WHERE id=$1", admin.id)
	space := f.space("M11 Analytics")
	f.share(space, "user", admin.id, "full")
	active := f.queryID("SELECT id::text FROM statuses WHERE grp='active' ORDER BY name LIMIT 1")
	done := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' ORDER BY name LIMIT 1")
	for i, status := range []string{active, active, done} {
		id := f.queryID("INSERT INTO items(type,title,created_by,status_id,importance) VALUES ('task',$1,$2,$3,'high') RETURNING id::text", "Task "+string(rune('A'+i)), admin.id, status)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
	}
	runBrowserFixture(t, f, []string{"m11-analytics-live.spec.ts"}, "PMS_FIXTURE_M11A_ADMIN="+admin.cookie.String(), "PMS_FIXTURE_M11A_SPACE="+space)
}
