package httpserver_test

import (
	"os"
	"testing"
)

func TestBrowserPilotPerformance(t *testing.T) {
	if os.Getenv("PMS_TEST_PILOT_PERFORMANCE") != "1" {
		t.Skip("set PMS_TEST_PILOT_PERFORMANCE=1 to measure the real browser against a 10,000-Item database")
	}
	f := newFixture(t)
	admin := f.person("admin")
	space, other := f.space("Pilot performance"), f.space("Background dataset")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Pilot benchmark project',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, space)
	table := f.queryID("SELECT id::text FROM views WHERE scope_item_id=$1 AND type='table'", project)
	board := f.queryID("SELECT id::text FROM views WHERE scope_item_id=$1 AND type='board'", project)
	status := f.queryID("SELECT id::text FROM statuses WHERE name='New' ORDER BY position LIMIT 1")
	f.exec(`WITH tasks AS (
  INSERT INTO items(type,title,created_by,status_id)
  SELECT 'task','Pilot benchmark ' || lpad(n::text,5,'0'),$1,$2 FROM generate_series(1,10000) n
  RETURNING id, substring(title FROM '[0-9]+$') AS suffix
)
INSERT INTO item_locations(item_id,space_id,parent_item_id,rank)
SELECT id, CASE WHEN suffix::int<=500 THEN $3::uuid ELSE $4::uuid END,
CASE WHEN suffix::int<=500 THEN $5::uuid END, suffix FROM tasks`, admin.id, status, space, other, project)
	f.exec("ANALYZE items")
	f.exec("ANALYZE item_locations")
	runBrowserFixture(t, f, []string{"pilot-perf.spec.ts"},
		"PMS_FIXTURE_PERF_PROJECT="+project,
		"PMS_FIXTURE_PERF_TABLE="+table,
		"PMS_FIXTURE_PERF_BOARD="+board,
		"PMS_FIXTURE_FIRST_COOKIE="+admin.cookie.String(),
	)
}
