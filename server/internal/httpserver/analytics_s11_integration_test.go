package httpserver_test

import (
	"testing"
	"time"
)

// TestAnalyticsWidgetsAreaPivotBurndownCompletion covers the S11.2 widget kinds against the raw rows.
func TestAnalyticsWidgetsAreaPivotBurndownCompletion(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Analytics")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	active := f.queryID("SELECT id::text FROM statuses WHERE grp='active' ORDER BY name LIMIT 1")
	done := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' ORDER BY name LIMIT 1")
	field := f.queryID(`INSERT INTO custom_fields(space_id,name,type,options,position) VALUES ($1,'Points','number','[]',0) RETURNING id::text`, space)
	task := func(title, status, importance string, created, completed string, points int) {
		id := f.queryID(`INSERT INTO items(type,title,created_by,status_id,importance,created_at,completed_at,fields)
VALUES ('task',$1,$2,$3::uuid,$4,current_date-$5::int,CASE WHEN $6<>'' THEN current_date-$6::int END,jsonb_build_object($7::text,$8::int)) RETURNING id::text`,
			title, admin.id, status, importance, created, completed, field, points)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
	}
	task("a", active, "high", "20", "", 2)
	task("b", active, "high", "10", "", 4)
	task("c", done, "high", "20", "5", 6)
	task("d", done, "low", "9", "2", 8)
	dash := f.call(admin, "POST", "/dashboards", map[string]any{"space_id": space, "name": "Board"})
	f.want(dash, 201, "dashboard")
	base := "/dashboards/" + dash.body["id"].(string)
	add := func(name, typ string, query map[string]any) string {
		t.Helper()
		version := f.queryID("SELECT version::text FROM dashboards WHERE id=$1", dash.body["id"])
		f.want(f.call(admin, "POST", base+"/widgets", map[string]any{"name": name, "type": typ, "config": map[string]any{"query": query}}, "If-Match", `"`+version+`"`), 201, name)
		return f.queryID("SELECT id::text FROM widgets WHERE dashboard_id=$1 AND name=$2", dash.body["id"], name)
	}
	data := func(id string) map[string]any {
		t.Helper()
		r := f.call(admin, "GET", base+"/widgets/"+id+"/data", nil)
		f.want(r, 200, "data")
		return r.body["result"].(map[string]any)
	}

	pivot := data(add("Pivot", "pivot", map[string]any{"reporting_on": "tasks", "metric": "count", "group_by": "importance", "break_by": "status_group"}))
	if len(pivot["rows"].([]any)) != 3 || pivot["total"] != float64(4) { // high/active, high/completed, low/completed
		t.Fatalf("pivot: %v", pivot)
	}
	version := f.queryID("SELECT version::text FROM dashboards WHERE id=$1", dash.body["id"])
	f.want(f.call(admin, "POST", base+"/widgets", map[string]any{"name": "Flat pivot", "type": "pivot", "config": map[string]any{"query": map[string]any{"reporting_on": "tasks", "metric": "count", "group_by": "importance"}}}, "If-Match", `"`+version+`"`), 422, "a pivot needs rows and columns")
	area := data(add("Area", "area", map[string]any{"reporting_on": "tasks", "metric": "count", "group_by": "status_group"}))
	if area["total"] != float64(4) {
		t.Fatalf("area: %v", area)
	}

	rate := data(add("Completion", "indicator", map[string]any{"reporting_on": "tasks", "metric": "completion_percent"}))
	if rate["total"] != float64(50) {
		t.Fatalf("2 of 4 completed is 50: %v", rate)
	}
	byImportance := data(add("Completion by importance", "column", map[string]any{"reporting_on": "tasks", "metric": "completion_percent", "group_by": "importance"}))
	for _, raw := range byImportance["rows"].([]any) {
		row := raw.(map[string]any)
		want := map[string]float64{"high": 33.3, "low": 100}[row["group"].(map[string]any)["key"].(string)]
		if row["value"] != want {
			t.Fatalf("completion by importance: %v", byImportance)
		}
	}
	avg := data(add("Average points", "indicator", map[string]any{"reporting_on": "tasks", "metric": "avg_field", "field_id": field}))
	if avg["total"] != float64(5) { // (2+4+6+8)/4
		t.Fatalf("average: %v", avg)
	}

	burn := data(add("Burndown", "burndown", map[string]any{"reporting_on": "tasks", "metric": "count"}))
	rows := burn["rows"].([]any)
	value := func(daysAgo int) float64 { return rows[len(rows)-1-daysAgo].(map[string]any)["value"].(float64) }
	// a: open since day -20; b: since -10; c: since -20 until it closed on day -5; d: since -9 until it closed on day -2.
	for ago, want := range map[int]float64{0: 2, 2: 2, 3: 3, 5: 3, 6: 4, 9: 4, 10: 3, 11: 2, 25: 0} {
		if len(rows) != 30 || value(ago) != want {
			t.Fatalf("burndown %d days ago = %v, want %v (%v)", ago, value(ago), want, rows)
		}
	}
}

func TestDashboardLayoutAndWidgetDescription(t *testing.T) {
	f := newFixture(t)
	admin, other := f.person("admin"), f.person("regular")
	dash := f.call(admin, "POST", "/dashboards", map[string]any{"name": "Board"})
	f.want(dash, 201, "dashboard")
	if dash.body["layout"] != "columns" {
		t.Fatalf("default layout: %v", dash.body)
	}
	id := dash.body["id"].(string)
	etag := func() string { return `"` + f.queryID("SELECT version::text FROM dashboards WHERE id=$1", id) + `"` }
	f.want(f.call(admin, "PATCH", "/dashboards/"+id, map[string]any{}, "If-Match", etag()), 422, "nothing to change")
	f.want(f.call(admin, "PATCH", "/dashboards/"+id, map[string]any{"layout": "diagonal"}, "If-Match", etag()), 422, "unknown layout")
	f.want(f.call(other, "PATCH", "/dashboards/"+id, map[string]any{"layout": "stack"}, "If-Match", etag()), 404, "someone else's personal board")
	set := f.call(admin, "PATCH", "/dashboards/"+id, map[string]any{"layout": "stack"}, "If-Match", etag())
	f.want(set, 200, "stack")
	if set.body["layout"] != "stack" || set.body["name"] != "Board" {
		t.Fatalf("layout only keeps the name: %v", set.body)
	}
	copied := f.call(admin, "POST", "/dashboards/"+id+"/duplicate", map[string]any{"name": "Copy"})
	f.want(copied, 201, "duplicate")
	if copied.body["layout"] != "stack" {
		t.Fatalf("a copy keeps the layout: %v", copied.body)
	}
	add := func(config map[string]any) reply {
		return f.call(admin, "POST", "/dashboards/"+id+"/widgets", map[string]any{"name": "W", "type": "task_list", "config": config}, "If-Match", etag())
	}
	f.want(add(map[string]any{"list": "completed", "description": "Finished this week"}), 201, "description")
	long := make([]byte, 501)
	for i := range long {
		long[i] = 'x'
	}
	f.want(add(map[string]any{"list": "completed", "description": string(long)}), 422, "description too long")
}

func TestPublicDashboardLinkExpiryPasswordAndFinancialMasking(t *testing.T) {
	f := newFixture(t)
	admin, reader := f.person("admin"), f.person("regular")
	space := f.space("Public board")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	f.share(space, "user", reader.id, "read")
	money := f.queryID(`INSERT INTO custom_fields(space_id,name,type,options,position) VALUES ($1,'Budget','currency','[]',0) RETURNING id::text`, space)
	f.exec("INSERT INTO items(type,title,created_by,fields) VALUES ('project','P',$1,jsonb_build_object($2::text,1000))", admin.id, money)
	dash := f.call(admin, "POST", "/dashboards", map[string]any{"space_id": space, "name": "Client view"})
	f.want(dash, 201, "dashboard")
	id := dash.body["id"].(string)
	version := func() string { return `"` + f.queryID("SELECT version::text FROM dashboards WHERE id=$1", id) + `"` }
	next := 0 // widgets are listed by position, so give each its own; equal positions come back in random id order
	add := func(name, typ string, query map[string]any) {
		f.want(f.call(admin, "POST", "/dashboards/"+id+"/widgets", map[string]any{"name": name, "type": typ, "position": next, "config": map[string]any{"query": query}}, "If-Match", version()), 201, name)
		next++
	}
	add("Open", "indicator", map[string]any{"reporting_on": "tasks", "metric": "count"})
	add("Budget total", "indicator", map[string]any{"reporting_on": "projects", "metric": "sum_field", "field_id": money})
	add("Billable", "indicator", map[string]any{"reporting_on": "time_entries", "metric": "billable_minutes"})
	add("Actual cost", "indicator", map[string]any{"reporting_on": "time_entries", "metric": "actual_cost_minor"})

	link := "/dashboards/" + id + "/public-link"
	soon := time.Now().Add(time.Hour)
	f.want(f.call(reader, "GET", link, nil), 403, "a reader cannot manage the link")
	f.want(f.call(reader, "PUT", link, map[string]any{"expires_at": soon}), 403, "nor create it")
	off := f.call(admin, "GET", link, nil)
	f.want(off, 200, "status")
	if off.body["enabled"] != false {
		t.Fatalf("no link yet: %v", off.body)
	}
	f.want(f.call(admin, "PUT", link, map[string]any{"expires_at": time.Now().Add(-time.Hour)}), 422, "past expiry")
	f.want(f.call(admin, "PUT", link, map[string]any{"expires_at": soon, "password": "short"}), 422, "weak password")
	made := f.call(admin, "PUT", link, map[string]any{"expires_at": soon, "password": "correct horse battery"})
	f.want(made, 200, "enable")
	token := made.body["token"].(string)
	if f.call(admin, "GET", link, nil).body["token"] != nil {
		t.Fatal("the token is shown once")
	}
	view := func(token, password string) reply {
		return f.call(nil, "POST", "/public/dashboards/"+token, map[string]any{"password": password})
	}
	f.want(view(token, ""), 401, "password required")
	f.want(view(token, "wrong password here"), 401, "wrong password")
	bogus := "y" + token[1:]
	if token[0] == 'y' {
		bogus = "z" + token[1:]
	}
	f.want(view(bogus, "correct horse battery"), 404, "unknown token")
	got := view(token, "correct horse battery")
	f.want(got, 200, "public view")
	widgets := got.body["widgets"].([]any)
	if len(widgets) != 4 || widgets[3].(map[string]any)["masked"] != true || widgets[0].(map[string]any)["masked"] != nil || widgets[1].(map[string]any)["masked"] != true || widgets[2].(map[string]any)["masked"] != true {
		t.Fatalf("money must be masked, the rest shown: %v", widgets)
	}
	for _, w := range widgets[1:] {
		if w.(map[string]any)["data"] != nil {
			t.Fatalf("a masked widget carries no data: %v", w)
		}
	}

	f.exec("UPDATE dashboard_public_links SET expires_at=now()-interval '1 second'")
	f.want(view(token, "correct horse battery"), 404, "expired")
	again := f.call(admin, "PUT", link, map[string]any{"expires_at": soon})
	f.want(again, 200, "re-enable without a password")
	f.want(view(token, ""), 404, "the old token no longer works")
	f.want(f.call(nil, "POST", "/public/dashboards/"+again.body["token"].(string), nil), 200, "open link")
	f.want(f.call(admin, "DELETE", link, nil), 204, "disable")
	f.want(f.call(nil, "POST", "/public/dashboards/"+again.body["token"].(string), nil), 404, "revoked")
}
