package httpserver_test

import "testing"

// E-01: totals describe the whole filtered set, not the current page, and follow the same filters and permissions as the rows.
func TestProjectTimelogTotalsCoverAllPages(t *testing.T) {
	f := newFixture(t)
	admin, owner := f.person("admin"), f.person("regular")
	space := f.space("Timelog totals")
	f.share(space, "user", owner.id, "limited")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Project',$1) RETURNING id::text", admin.id)
	task := customFieldItem(f, admin, space, "Entries task")
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, space)
	f.exec("UPDATE item_locations SET parent_item_id=$1 WHERE item_id=$2 AND space_id=$3", project, task, space)
	entries := []map[string]any{
		{"item_id": task, "date": "2026-11-10", "minutes": 60, "category": "Design", "billing_type": "billable"},
		{"item_id": task, "date": "2026-11-11", "minutes": 30, "category": "Design", "billing_type": "non_billable"},
		{"item_id": task, "date": "2026-11-12", "minutes": 90, "category": "Build", "billing_type": "billable"},
	}
	for _, input := range entries {
		f.want(f.call(owner, "POST", "/time-entries", input), 201, "log time")
	}
	path := "/items/" + project + "/time-entries"

	// Page size 1 spans three pages; the totals must still be the full sums.
	page := f.call(admin, "GET", path+"?limit=1", nil)
	f.want(page, 200, "first page")
	if len(page.items()) != 1 {
		t.Fatalf("expected one row on the page: %+v", page)
	}
	if page.body["total_minutes"] != float64(180) || page.body["billable_minutes"] != float64(150) || page.body["non_billable_minutes"] != float64(30) {
		t.Fatalf("totals should cover every page: %+v", page.body)
	}

	// The same filter gives the same filtered totals: only Design entries count.
	design := f.call(admin, "GET", path+"?category=design&limit=1", nil)
	f.want(design, 200, "filtered page")
	if design.body["total_minutes"] != float64(90) || design.body["billable_minutes"] != float64(60) {
		t.Fatalf("filtered totals should match the filter: %+v", design.body)
	}
}
