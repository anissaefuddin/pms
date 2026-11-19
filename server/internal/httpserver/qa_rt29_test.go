package httpserver_test

import (
	"net/http"
	"testing"
)

// RT-29: tasks with no assignee are grouped under a labelled bucket, in the query result and the CSV export.
func TestRT29UnassignedBucketLabel(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("RT29")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	id := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Nobody',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
	r := f.call(admin, http.MethodPost, "/analytics/query", map[string]any{"reporting_on": "tasks", "metric": "count", "group_by": "assignee", "space_ids": []string{space}})
	if r.status != 200 {
		t.Fatalf("query: %d %v", r.status, r.body)
	}
	rows, _ := r.body["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("rows: %v", r.body)
	}
	group, _ := rows[0].(map[string]any)["group"].(map[string]any)
	if group["label"] != "Unassigned" {
		t.Fatalf("unassigned bucket label = %q, want Unassigned (row %v)", group["label"], rows[0])
	}
}
