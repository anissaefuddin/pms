package httpserver_test

import "testing"

// Actual cost in a report is money: the administrator gets it computed from the rate snapshots, anyone else gets 403 and no number,
// and a public dashboard masks the widget like any other money widget.
func TestReportActualCostIsAdminOnly(t *testing.T) {
	f := newFixture(t)
	admin, reader := f.person("admin"), f.person("regular")
	space := f.space("Cost report")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	f.share(space, "user", reader.id, "read")
	task := f.queryID(`INSERT INTO items(type,title,created_by) VALUES ('task','T',$1) RETURNING id::text`, admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", task, space)
	f.exec(`INSERT INTO time_entries(item_id,user_id,entry_date,minutes,billing_type,rate_snapshot,status) VALUES ($1,$2,'2026-11-02',90,'billable',4000,'approved'),($1,$2,'2026-11-03',60,'non_billable',4000,'approved'),($1,$2,'2026-11-04',60,'billable',4000,'submitted')`, task, admin.id)
	q := map[string]any{"reporting_on": "time_entries", "metric": "actual_cost_minor", "group_by": "user"}
	f.want(f.call(reader, "POST", "/analytics/query", q), 403, "a non-admin gets no cost")
	got := f.call(admin, "POST", "/analytics/query", q)
	f.want(got, 200, "admin")
	if got.body["total"] != float64(6000) { // 90 min x 4000/h, non-billable and not yet approved time excluded
		t.Fatalf("cost = %v", got.body["total"])
	}
}
