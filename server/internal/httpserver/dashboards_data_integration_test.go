package httpserver_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestWidgetDataPresetsAndCustomQueries(t *testing.T) {
	f := newFixture(t)
	admin, member, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	shared, secret := f.space("Widgets shared"), f.space("Widgets secret")
	f.share(shared, "user", admin.id, "full") // A1.2: admin creates this Space's content as an explicit member
	f.share(shared, "user", member.id, "read")
	active := f.queryID("SELECT id::text FROM statuses WHERE grp='active' ORDER BY name LIMIT 1")
	done := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' ORDER BY name LIMIT 1")
	task := func(space, title, status, due string) string {
		id := f.queryID(`INSERT INTO items(type,title,created_by,status_id,due_date,completed_at)
VALUES ('task',$1,$2,$3::uuid,NULLIF($4,'')::date, CASE WHEN $3::text=$5::text THEN now() END) RETURNING id::text`, title, admin.id, status, due, done)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
		return id
	}
	assign := func(id string, p *person) {
		f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", id, p.id)
	}
	today := "2026-01-01" // replaced below by the database's own date so "this week" cannot drift
	today = f.queryID("SELECT current_date::text")
	overdue := f.queryID("SELECT (current_date - 21)::text")
	dueToday := task(shared, "Due today", active, today)
	late := task(shared, "Late", active, overdue)
	task(shared, "Backlog", active, "")
	finished := task(shared, "Finished", done, overdue)
	review := task(shared, "Needs my review", active, "")
	followed := task(shared, "Followed by me", active, "")
	task(secret, "Secret late", active, overdue)
	assign(dueToday, member)
	assign(late, member)
	assign(finished, member)
	assign(review, admin)
	f.exec("INSERT INTO item_followers(item_id,user_id) VALUES ($1,$2)", followed, member.id)
	approval := f.queryID("INSERT INTO approvals(id,item_id,created_by) VALUES (gen_random_uuid(),$1,$2) RETURNING id::text", review, admin.id)
	f.exec("INSERT INTO approval_approvers(approval_id,user_id) VALUES ($1,$2)", approval, member.id)
	f.exec("UPDATE items SET completed_at=now() - interval '1 hour' WHERE id=$1", finished)

	dash := f.call(admin, "POST", "/dashboards", map[string]any{"space_id": shared, "name": "Space board"})
	f.want(dash, 201, "Space dashboard")
	base := "/dashboards/" + dash.body["id"].(string)
	widget := func(p *person, body map[string]any) reply {
		return f.call(p, "POST", base+"/widgets", body, "If-Match", `"1"`)
	}
	addWidget := func(name, typ string, config map[string]any) string {
		t.Helper()
		version := f.queryID("SELECT version::text FROM dashboards WHERE id=$1", dash.body["id"])
		r := f.call(admin, "POST", base+"/widgets", map[string]any{"name": name, "type": typ, "config": config}, "If-Match", `"`+version+`"`)
		f.want(r, 201, "add widget "+name)
		return f.queryID("SELECT id::text FROM widgets WHERE dashboard_id=$1 AND name=$2", dash.body["id"], name)
	}

	// The widget config is a closed schema as well.
	for name, c := range map[string]struct {
		typ    string
		config map[string]any
	}{
		"task list without source": {"task_list", map[string]any{}},
		"list and template":        {"task_list", map[string]any{"list": "to_review", "template": "overdue"}},
		"template named as list":   {"task_list", map[string]any{"list": "overdue"}},
		"list named as template":   {"task_list", map[string]any{"template": "to_review"}},
		"unknown template":         {"task_list", map[string]any{"template": "archived"}},
		"query on a task list":     {"task_list", map[string]any{"list": "completed", "query": map[string]any{"reporting_on": "tasks", "metric": "count"}}},
		"list on a chart":          {"pie", map[string]any{"list": "completed"}},
		"unknown query field":      {"pie", map[string]any{"query": map[string]any{"reporting_on": "tasks", "metric": "count", "sql": "1"}}},
		"invalid query":            {"pie", map[string]any{"query": map[string]any{"reporting_on": "tasks", "metric": "minutes"}}},
		"query in the wrong shape": {"pie", map[string]any{"query": "select 1"}},
	} {
		version := f.queryID("SELECT version::text FROM dashboards WHERE id=$1", dash.body["id"])
		f.want(f.call(admin, "POST", base+"/widgets", map[string]any{"name": "Bad", "type": c.typ, "config": c.config}, "If-Match", `"`+version+`"`), 422, name)
	}

	titles := func(p *person, widgetID string) []string {
		t.Helper()
		r := f.call(p, "GET", base+"/widgets/"+widgetID+"/data", nil)
		f.want(r, 200, "widget data")
		if r.body["kind"] != "tasks" {
			t.Fatalf("kind = %v", r.body["kind"])
		}
		out := []string{}
		for _, raw := range r.body["items"].([]any) {
			out = append(out, raw.(map[string]any)["title"].(string))
		}
		return out
	}
	same := func(got []string, want ...string) {
		t.Helper()
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	// DASH-01 lists, as the member (read-only on the Space) sees them.
	same(titles(member, addWidget("This week", "task_list", map[string]any{"list": "todo_this_week"})), "Due today")
	same(titles(member, addWidget("Review", "task_list", map[string]any{"list": "to_review"})), "Needs my review")
	same(titles(member, addWidget("Done", "task_list", map[string]any{"list": "completed"})), "Finished")
	same(titles(member, addWidget("Mine", "task_list", map[string]any{"list": "assigned_to_me"})), "Late", "Due today") // ordered by due date
	// DASH-02 templates.
	same(titles(member, addWidget("My tasks", "task_list", map[string]any{"template": "my_tasks"})), "Late", "Due today")
	overdueWidget := addWidget("Overdue", "task_list", map[string]any{"template": "overdue"})
	same(titles(member, overdueWidget), "Late") // the Space dashboard keeps to its Space: no Secret late
	same(titles(member, addWidget("Followed", "task_list", map[string]any{"template": "followed"})), "Followed by me")
	same(titles(member, addWidget("Backlog", "task_list", map[string]any{"template": "backlog"})), "Backlog", "Followed by me", "Needs my review")
	// The same widget answers for the viewer, not for the owner: the admin's review list is empty.
	same(titles(admin, addWidget("Review admin", "task_list", map[string]any{"list": "to_review"})))

	// A personal dashboard has no Space limit, but policy still hides what the viewer cannot see.
	personal := f.call(admin, "POST", "/dashboards", map[string]any{"name": "Personal"})
	f.want(personal, 201, "personal dashboard")
	pBase := "/dashboards/" + personal.body["id"].(string)
	f.want(f.call(admin, "POST", pBase+"/widgets", map[string]any{"name": "Overdue everywhere", "type": "task_list", "config": map[string]any{"template": "overdue"}}, "If-Match", `"1"`), 201, "personal widget")
	pWidget := f.queryID("SELECT id::text FROM widgets WHERE dashboard_id=$1", personal.body["id"])
	r := f.call(admin, "GET", pBase+"/widgets/"+pWidget+"/data", nil)
	f.want(r, 200, "admin overdue everywhere")
	// D-17 A1: the admin sees only the Late task; the Secret task in a Space the admin has no grant on is not counted.
	if n := len(r.body["items"].([]any)); n != 1 {
		t.Fatalf("admin overdue = %v, want the 1 visible task", r.body["items"])
	}
	f.want(f.call(member, "GET", pBase+"/widgets/"+pWidget+"/data", nil), 404, "someone else's personal dashboard")

	// Custom widgets run the engine with the viewer's rights.
	query := map[string]any{"reporting_on": "tasks", "metric": "count", "group_by": "status_group", "space_ids": []string{shared, secret}}
	custom := addWidget("By status", "pie", map[string]any{"show_legend": true, "pie_style": "donut", "query": query})
	data := func(p *person) map[string]any {
		t.Helper()
		r := f.call(p, "GET", base+"/widgets/"+custom+"/data", nil)
		f.want(r, 200, "custom widget data")
		if r.body["kind"] != "series" {
			t.Fatalf("kind = %v", r.body["kind"])
		}
		return r.body["result"].(map[string]any)
	}
	// D-17 A1: without a grant on the secret Space, the admin's widget total is the same as the member's shared total.
	if got := data(admin)["total"]; got != 6.0 {
		t.Fatalf("admin total = %v, want 6 (only the shared Space)", got)
	}
	if got := data(member)["total"]; got != 6.0 {
		t.Fatalf("member total = %v, want 6 (the secret Space is hidden)", got)
	}
	raw := f.queryID(`SELECT count(*)::text FROM items i JOIN item_locations l ON l.item_id=i.id WHERE l.space_id=$1 AND i.type='task'`, shared)
	if fmt.Sprint(data(member)["total"]) != raw {
		t.Fatalf("custom widget total differs from raw count %s", raw)
	}
	// Without a query the chart has nothing to show, and strangers learn nothing.
	empty := addWidget("Empty chart", "bar", map[string]any{})
	f.want(f.call(member, "GET", base+"/widgets/"+empty+"/data", nil), 422, "no data definition")
	f.want(f.call(outsider, "GET", base+"/widgets/"+custom+"/data", nil), 404, "outsider")
	f.want(f.call(member, "GET", pBase+"/widgets/"+custom+"/data", nil), 404, "widget of another dashboard")
	f.want(f.call(member, "GET", base+"/widgets/not-a-uuid/data", nil), 404, "bad id")
	_ = widget
	_ = http.StatusOK
}

func TestStarsAndStarredWidget(t *testing.T) {
	f := newFixture(t)
	admin, member, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	shared, secret := f.space("Stars shared"), f.space("Stars secret")
	f.share(shared, "user", member.id, "read")
	active := f.queryID("SELECT id::text FROM statuses WHERE grp='active' ORDER BY name LIMIT 1")
	done := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' ORDER BY name LIMIT 1")
	task := func(space, title, status string) string {
		id := f.queryID(`INSERT INTO items(type,title,created_by,status_id) VALUES ('task',$1,$2,$3) RETURNING id::text`, title, admin.id, status)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
		return id
	}
	open, closed, hidden := task(shared, "Open and starred", active), task(shared, "Done and starred", done), task(secret, "Secret", active)

	// Starring needs View: an Item the caller cannot see is a plain 404, and nothing is stored.
	f.want(f.call(member, "PUT", "/items/"+hidden+"/star", nil), 404, "hidden item")
	f.want(f.call(outsider, "PUT", "/items/"+open+"/star", nil), 404, "no access")
	f.want(f.call(member, "PUT", "/items/not-a-uuid/star", nil), 404, "bad id")
	f.want(f.call(nil, "PUT", "/items/"+open+"/star", nil), 401, "anonymous")
	if n := f.queryID("SELECT count(*)::text FROM item_stars"); n != "0" {
		t.Fatalf("stars after refused requests = %s", n)
	}
	f.want(f.call(member, "PUT", "/items/"+open+"/star", nil), 204, "star")
	f.want(f.call(member, "PUT", "/items/"+open+"/star", nil), 204, "starring twice is idempotent")
	f.want(f.call(member, "PUT", "/items/"+closed+"/star", nil), 204, "star a completed task")
	// D-17 A1: a Workspace admin without a grant on the secret Space cannot star its task; with an explicit grant it can.
	f.want(f.call(admin, "PUT", "/items/"+hidden+"/star", nil), 404, "admin without a grant cannot star the secret task")
	f.share(secret, "user", admin.id, "read")
	f.want(f.call(admin, "PUT", "/items/"+hidden+"/star", nil), 204, "admin with an explicit grant stars the secret task")
	f.exec("DELETE FROM shares WHERE target_type='space' AND target_id=$1 AND principal_id=$2", secret, admin.id)
	if n := f.queryID("SELECT count(*)::text FROM item_stars WHERE user_id=$1", member.id); n != "2" {
		t.Fatalf("member stars = %s, want 2", n)
	}

	titles := func(p *person) []string {
		t.Helper()
		r := f.call(p, "GET", "/stars", nil)
		f.want(r, 200, "list stars")
		out := []string{}
		for _, item := range r.items() {
			out = append(out, item["title"].(string))
		}
		return out
	}
	if got := fmt.Sprint(titles(member)); got != "[Done and starred Open and starred]" { // newest star first
		t.Fatalf("member stars = %s", got)
	}
	// The admin's star on Secret was made with a temporary grant; with the grant gone, a starred item the admin cannot view is not shown.
	if got := fmt.Sprint(titles(admin)); got != "[]" {
		t.Fatalf("stars are personal and visible-only: admin = %s", got)
	}
	if len(titles(outsider)) != 0 {
		t.Fatal("outsider has no stars")
	}

	// The starred template lists the caller's own active starred tasks, and only those they can still see.
	board := f.call(member, "POST", "/dashboards", map[string]any{"name": "Stars"})
	f.want(board, 201, "dashboard")
	base := "/dashboards/" + board.body["id"].(string)
	f.want(f.call(member, "POST", base+"/widgets", map[string]any{"name": "Starred", "type": "task_list", "config": map[string]any{"template": "starred"}}, "If-Match", `"1"`), 201, "widget")
	widget := f.queryID("SELECT id::text FROM widgets WHERE dashboard_id=$1", board.body["id"])
	data := f.call(member, "GET", base+"/widgets/"+widget+"/data", nil)
	f.want(data, 200, "starred data")
	if items := data.body["items"].([]any); len(items) != 1 || items[0].(map[string]any)["title"] != "Open and starred" {
		t.Fatalf("starred widget = %v, want only the open starred task", data.body["items"])
	}
	// Losing access hides the starred Item everywhere without deleting the star.
	f.exec("DELETE FROM shares WHERE principal_id=$1", member.id)
	if len(titles(member)) != 0 {
		t.Fatal("a star on an Item the member can no longer see must not be listed")
	}
	if n := f.queryID("SELECT count(*)::text FROM item_stars WHERE user_id=$1", member.id); n != "2" {
		t.Fatalf("stars kept = %s, want 2", n)
	}
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('space',$1,'user',$2,'read')", shared, member.id)

	// Unstarring is idempotent, touches only the caller's row, and works even without View.
	f.want(f.call(member, "DELETE", "/items/"+open+"/star", nil), 204, "unstar")
	f.want(f.call(member, "DELETE", "/items/"+open+"/star", nil), 204, "unstar twice")
	f.want(f.call(outsider, "DELETE", "/items/"+closed+"/star", nil), 204, "someone else's star is untouched")
	if got := fmt.Sprint(titles(member)); got != "[Done and starred]" {
		t.Fatalf("member stars after unstar = %s", got)
	}
	// Deleting the Item removes its stars.
	f.exec("DELETE FROM items WHERE id=$1", closed)
	if n := f.queryID("SELECT count(*)::text FROM item_stars WHERE item_id=$1", closed); n != "0" {
		t.Fatalf("stars of a deleted item = %s", n)
	}
}
