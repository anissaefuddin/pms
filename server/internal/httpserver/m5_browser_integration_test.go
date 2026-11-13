package httpserver_test

import (
	"fmt"
	"math"
	"os"
	"testing"
)

// TestBrowserM5Flows (S5.3.1) seeds a small workspace with hand-countable data and lets Chrome build a dashboard and
// a report through the UI only, against the real API and PostgreSQL.
func TestBrowserM5Flows(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	admin, bob := f.person("admin"), f.person("regular")
	f.exec("UPDATE users SET first_name='Ada',last_name='Admin' WHERE id=$1", admin.id)
	f.exec("UPDATE users SET first_name='Bob',last_name='Builder' WHERE id=$1", bob.id)
	space := f.space("M5 Delivery")
	f.share(space, "user", bob.id, "editor")
	active := f.queryID("SELECT id::text FROM statuses WHERE grp='active' ORDER BY name LIMIT 1")
	done := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' ORDER BY name LIMIT 1")
	task := func(title, status, importance, due string, assignee *person) string {
		id := f.queryID(`INSERT INTO items(type,title,created_by,status_id,importance,due_date) VALUES ('task',$1,$2,$3::uuid,$4,NULLIF($5,'')::date) RETURNING id::text`,
			title, admin.id, status, importance, due)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
		if assignee != nil {
			f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", id, assignee.id)
		}
		return id
	}
	late := f.queryID("SELECT (current_date - 10)::text")
	later := f.queryID("SELECT (current_date - 5)::text")
	soon := f.queryID("SELECT (current_date + 10)::text")
	oneID := task("Overdue one", active, "high", late, bob)
	task("Overdue two", active, "normal", later, admin)
	fresh := task("Fresh work", active, "normal", soon, bob)
	task("Finished work", done, "normal", "", admin)
	task("Nobody owns this", active, "normal", "", nil)
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,current_date,60)", oneID, bob.id)
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,current_date,120)", fresh, admin.id)

	workdays := f.queryID(`SELECT count(*)::text FROM generate_series(date_trunc('month', current_date), date_trunc('month', current_date) + interval '1 month - 1 day', interval '1 day') d
WHERE extract(dow FROM d) BETWEEN 1 AND 5`)
	var capacity float64
	fmt.Sscan(workdays, &capacity)
	capacity *= 8 * 60
	percent := func(minutes float64) string { return fmt.Sprint(math.Round(minutes/capacity*1000) / 10) }

	runBrowserFixture(t, f, []string{"m5-live.spec.ts"},
		"PMS_FIXTURE_M5_ADMIN="+admin.cookie.String(),
		"PMS_FIXTURE_M5_FRESH="+fresh,
		"PMS_FIXTURE_M5_UTIL_BOB="+percent(60),
		"PMS_FIXTURE_M5_UTIL_ADA="+percent(120),
	)
}
