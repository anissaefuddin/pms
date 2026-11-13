package httpserver_test

import (
	"os"
	"testing"
)

// TestBrowserM10Flows runs E2E-PAR-08 (Gantt baseline) against the real API and PostgreSQL.
func TestBrowserM10Flows(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	admin := f.person("admin")
	f.exec("UPDATE users SET first_name='Ada',last_name='Admin' WHERE id=$1", admin.id)
	space := f.space("M10 Work")
	f.share(space, "user", admin.id, "full")
	project := f.queryID("INSERT INTO items(type,title,created_by,start_date,due_date) VALUES ('project','Launch',$1,'2026-11-02','2026-11-13') RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", project, space)
	task := f.queryID("INSERT INTO items(type,title,created_by,start_date,due_date) VALUES ('task','Design',$1,'2026-11-02','2026-11-04') RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'A')", task, space, project)
	f.want(f.call(admin, "POST", "/views", map[string]any{"scope_item_id": project, "name": "Plan", "type": "gantt", "visibility": "public"}), 201, "gantt view")
	runBrowserFixture(t, f, []string{"m10-live.spec.ts"}, "PMS_FIXTURE_M10_ADMIN="+admin.cookie.String(), "PMS_FIXTURE_M10_PROJECT="+project, "PMS_FIXTURE_M10_TASK="+task)
}
