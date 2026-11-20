package httpserver_test

import (
	"io"
	"net/http"
	"testing"
)

// rawGet returns a response body untouched; fixture.call only decodes JSON.
func (f *fixture) rawGet(p *person, path string) (int, http.Header, string) {
	f.t.Helper()
	req, _ := http.NewRequest(http.MethodGet, f.srv.URL+"/api/v1"+path, nil)
	req.AddCookie(p.cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(body)
}

func TestReportsLifecycleAndAccess(t *testing.T) {
	f := newFixture(t)
	admin, editor, reader, stranger := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Reports Space")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	f.share(space, "user", editor.id, "editor")
	f.share(space, "user", reader.id, "read")
	task := customFieldItem(f, admin, space, "Task that reports never touch")

	listIDs := func(p *person, query string) map[string]map[string]any {
		t.Helper()
		r := f.call(p, "GET", "/reports"+query, nil)
		f.want(r, 200, "list reports")
		out := map[string]map[string]any{}
		for _, raw := range r.body["items"].([]any) {
			item := raw.(map[string]any)
			out[item["id"].(string)] = item
		}
		return out
	}
	create := func(p *person, body map[string]any) reply { return f.call(p, "POST", "/reports", body) }

	// Creation: Create on the Space is required, and sharing "space" needs a Space.
	f.want(create(reader, map[string]any{"name": "Denied", "space_id": space}), 403, "reader cannot create in the Space")
	f.want(create(stranger, map[string]any{"name": "Hidden", "space_id": space}), 404, "hidden Space")
	f.want(create(editor, map[string]any{"name": "No Space", "sharing": "space"}), 422, "space sharing needs a Space")
	f.want(create(editor, map[string]any{"name": "   "}), 422, "empty name")
	for name, def := range map[string]any{
		"unknown field":  map[string]any{"layout": "table", "query": map[string]any{"reporting_on": "tasks", "metric": "count"}, "sql": "x"},
		"bad layout":     map[string]any{"layout": "pie", "query": map[string]any{"reporting_on": "tasks", "metric": "count"}},
		"bad query":      map[string]any{"layout": "table", "query": map[string]any{"reporting_on": "tasks", "metric": "minutes"}},
		"missing layout": map[string]any{"query": map[string]any{"reporting_on": "tasks", "metric": "count"}},
	} {
		f.want(create(editor, map[string]any{"name": "Bad", "definition": def}), 422, name)
	}

	personal := create(editor, map[string]any{"name": "  My numbers  "})
	f.want(personal, 201, "personal report")
	if personal.body["name"] != "My numbers" || personal.body["space"] != nil || personal.body["sharing"] != "private" || personal.header.Get("ETag") != `"1"` {
		t.Fatalf("unexpected personal report: %+v", personal.body)
	}
	if def := personal.body["definition"].(map[string]any); def["layout"] != "table" {
		t.Fatalf("default definition = %v", def)
	}
	personalPath := "/reports/" + personal.body["id"].(string)

	shared := create(editor, map[string]any{"name": "Team status", "space_id": space, "sharing": "space", "definition": map[string]any{
		"layout": "column", "query": map[string]any{"reporting_on": "tasks", "metric": "count", "group_by": "status_group"}}})
	f.want(shared, 201, "Space report")
	sharedPath := "/reports/" + shared.body["id"].(string)
	private := create(editor, map[string]any{"name": "Private in Space", "space_id": space})
	f.want(private, 201, "private report in a Space")
	privatePath := "/reports/" + private.body["id"].(string)

	// Visibility: the list and the detail agree.
	mine := listIDs(editor, "")
	if len(mine) != 3 {
		t.Fatalf("editor sees %d reports, want 3", len(mine))
	}
	row := mine[shared.body["id"].(string)]
	if row["name"] != "Team status" || row["sharing"] != "space" || row["owner"].(map[string]any)["id"] != editor.id ||
		row["space"].(map[string]any)["name"] != "Reports Space" || row["created_at"] == nil || row["updated_at"] == nil || row["subscription"] != nil {
		t.Fatalf("list row = %+v", row)
	}
	if _, has := row["definition"]; has {
		t.Fatal("the list must not carry definitions")
	}
	seen := listIDs(reader, "")
	if len(seen) != 1 || seen[shared.body["id"].(string)] == nil {
		t.Fatalf("reader should see only the shared report, got %v", seen)
	}
	// An admin holds every right on a shared Space, so the shared report is visible; personal and private ones are not.
	if adminSees := listIDs(admin, ""); len(listIDs(stranger, "")) != 0 || len(adminSees) != 1 || adminSees[shared.body["id"].(string)] == nil {
		t.Fatal("other people's personal and private reports must not be listed")
	}
	f.want(f.call(reader, "GET", sharedPath, nil), 200, "reader reads shared report")
	f.want(f.call(reader, "GET", privatePath, nil), 404, "private report hides from Space members")
	f.want(f.call(reader, "GET", personalPath, nil), 404, "personal report is private")
	f.want(f.call(stranger, "GET", sharedPath, nil), 404, "non-member")
	f.want(f.call(admin, "GET", privatePath, nil), 404, "even an admin does not read someone's private report")
	if only := listIDs(editor, "?space_id="+space); len(only) != 2 {
		t.Fatalf("space filter returned %d reports, want 2", len(only))
	}

	// Editing: If-Match, Edit on the Space, owner-only sharing, no moves.
	f.want(f.call(editor, "PATCH", sharedPath, map[string]any{"name": "No If-Match"}), 428, "If-Match required")
	f.want(f.call(editor, "PATCH", sharedPath, map[string]any{"name": "Stale"}, "If-Match", `"9"`), 409, "stale version")
	f.want(f.call(reader, "PATCH", sharedPath, map[string]any{"name": "Denied"}, "If-Match", `"1"`), 403, "reader cannot edit")
	f.want(f.call(stranger, "PATCH", sharedPath, map[string]any{"name": "Denied"}, "If-Match", `"1"`), 404, "non-member cannot edit")
	f.want(f.call(editor, "PATCH", sharedPath, map[string]any{"space_id": space}, "If-Match", `"1"`), 422, "no move")
	renamed := f.call(editor, "PATCH", sharedPath, map[string]any{"name": "Team status v2"}, "If-Match", `"1"`)
	f.want(renamed, 200, "owner renames")
	if renamed.body["name"] != "Team status v2" || renamed.header.Get("ETag") != `"2"` {
		t.Fatalf("renamed = %+v", renamed.body)
	}
	f.want(f.call(editor, "PATCH", sharedPath, map[string]any{"definition": map[string]any{"layout": "bad"}}, "If-Match", `"2"`), 422, "bad definition on update")
	hidden := f.call(editor, "PATCH", sharedPath, map[string]any{"sharing": "private"}, "If-Match", `"2"`)
	f.want(hidden, 200, "owner stops sharing")
	f.want(f.call(reader, "GET", sharedPath, nil), 404, "unshared report disappears for the Space")
	f.want(f.call(editor, "PATCH", sharedPath, map[string]any{"sharing": "space"}, "If-Match", `"3"`), 200, "owner shares again")
	f.want(f.call(editor, "PATCH", personalPath, map[string]any{"sharing": "space"}, "If-Match", `"1"`), 422, "personal report cannot be shared with a Space")
	// A Space member with Edit may change a shared report but not who sees it.
	f.share(space, "user", stranger.id, "editor")
	f.want(f.call(stranger, "PATCH", sharedPath, map[string]any{"name": "Edited by a teammate"}, "If-Match", `"4"`), 200, "Space editor edits shared report")
	f.want(f.call(stranger, "PATCH", sharedPath, map[string]any{"sharing": "private"}, "If-Match", `"5"`), 403, "only the owner changes sharing")
	f.exec("DELETE FROM shares WHERE principal_id=$1", stranger.id)

	// Subscriptions belong to the caller and need View.
	sub := map[string]any{"weekday": 1, "hour": 8}
	f.want(f.call(reader, "PUT", sharedPath+"/subscription", sub), 200, "reader subscribes")
	f.want(f.call(reader, "PUT", sharedPath+"/subscription", map[string]any{"weekday": 7, "hour": 8}), 422, "weekday range")
	f.want(f.call(reader, "PUT", sharedPath+"/subscription", map[string]any{"weekday": 1, "hour": 24}), 422, "hour range")
	f.want(f.call(stranger, "PUT", sharedPath+"/subscription", sub), 404, "no View, no subscription")
	f.want(f.call(reader, "PUT", privatePath+"/subscription", sub), 404, "cannot subscribe to a private report")
	if got := listIDs(reader, "")[shared.body["id"].(string)]["subscription"]; got == nil || got.(map[string]any)["hour"] != 8.0 {
		t.Fatalf("reader subscription = %v", got)
	}
	if got := listIDs(editor, "")[shared.body["id"].(string)]["subscription"]; got != nil {
		t.Fatalf("subscriptions must not leak to other users: %v", got)
	}
	f.want(f.call(reader, "PUT", sharedPath+"/subscription", map[string]any{"weekday": 2, "hour": 9}), 200, "subscription is replaced, not duplicated")
	if n := f.queryID("SELECT count(*)::text FROM report_subscriptions"); n != "1" {
		t.Fatalf("subscriptions = %s, want 1", n)
	}
	f.want(f.call(reader, "DELETE", sharedPath+"/subscription", nil), 204, "unsubscribe")
	if got := listIDs(reader, "")[shared.body["id"].(string)]["subscription"]; got != nil {
		t.Fatalf("subscription remained: %v", got)
	}
	f.want(f.call(reader, "PUT", sharedPath+"/subscription", sub), 200, "subscribe again before delete")

	// Deleting needs Delete, removes the subscriptions, and leaves the data alone.
	f.want(f.call(editor, "DELETE", sharedPath, nil, "If-Match", `"5"`), 403, "editor lacks Delete on the Space")
	f.want(f.call(admin, "DELETE", privatePath, nil, "If-Match", `"1"`), 404, "admin cannot delete someone's private report")
	f.exec("UPDATE shares SET role='full' WHERE principal_id=$1", editor.id)
	f.want(f.call(editor, "DELETE", sharedPath, nil), 428, "If-Match required")
	f.want(f.call(editor, "DELETE", sharedPath, nil, "If-Match", `"1"`), 409, "stale delete")
	f.want(f.call(editor, "DELETE", sharedPath, nil, "If-Match", `"5"`), 204, "owner with Delete removes the report")
	if n := f.queryID("SELECT count(*)::text FROM report_subscriptions"); n != "0" {
		t.Fatalf("subscriptions after delete = %s", n)
	}
	f.want(f.call(editor, "DELETE", personalPath, nil, "If-Match", `"1"`), 204, "delete personal report")
	f.want(f.call(admin, "GET", "/items/"+task, nil), 200, "reports never own data")

	// A deleted Space hides its reports.
	f.exec("UPDATE spaces SET deleted_at=now() WHERE id=$1", space)
	f.want(f.call(editor, "GET", privatePath, nil), 404, "deleted Space hides the report")
	if n := len(listIDs(editor, "")); n != 0 {
		t.Fatalf("deleted Space still lists %d reports", n)
	}
	_ = http.StatusOK
}

func TestReportResultAndExport(t *testing.T) {
	f := newFixture(t)
	admin, reader, stranger := f.person("admin"), f.person("regular"), f.person("regular")
	shared, secret := f.space("Run shared"), f.space("Run secret")
	f.share(shared, "user", admin.id, "full") // A1.2: admin creates this Space's content as an explicit member
	f.share(shared, "user", reader.id, "read")
	active := f.queryID("SELECT id::text FROM statuses WHERE grp='active' ORDER BY name LIMIT 1")
	task := func(space, title string) string {
		id := f.queryID(`INSERT INTO items(type,title,created_by,status_id) VALUES ('task',$1,$2,$3) RETURNING id::text`, title, admin.id, active)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
		return id
	}
	formula := task(shared, `=HYPERLINK("http://evil.example","click")`)
	task(shared, "Plain")
	task(secret, "Hidden")
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,'2026-05-04',45)", formula, admin.id)

	def := func(query map[string]any) map[string]any {
		return map[string]any{"layout": "column", "query": query}
	}
	both := []string{shared, secret}
	// A1.3: the admin owns the report and reads both Spaces as an explicit member; a Workspace admin alone reads neither.
	f.share(secret, "user", admin.id, "read")
	counts := f.call(admin, "POST", "/reports", map[string]any{"name": "Counts", "space_id": shared, "sharing": "space",
		"definition": def(map[string]any{"reporting_on": "tasks", "metric": "count", "group_by": "status_group", "space_ids": both})})
	f.want(counts, 201, "counts report")
	countsPath := "/reports/" + counts.body["id"].(string)

	// The same report answers differently for each viewer; the owner is an admin and sees all three tasks.
	result := func(p *person, path string) reply { return f.call(p, "GET", path+"/result", nil) }
	if r := result(admin, countsPath); r.body["layout"] != "column" || r.body["result"].(map[string]any)["total"] != 3.0 {
		t.Fatalf("admin result = %+v", r.body)
	}
	if r := result(reader, countsPath); r.body["result"].(map[string]any)["total"] != 2.0 {
		t.Fatalf("reader result = %+v", r.body)
	}
	f.want(result(stranger, countsPath), 404, "stranger")
	f.want(f.call(admin, "GET", "/reports/not-a-uuid/result", nil), 404, "bad id")

	// A source that matches nothing is a valid, empty report.
	emptyQuery := f.call(admin, "POST", "/reports", map[string]any{"name": "Nothing", "definition": def(map[string]any{
		"reporting_on": "projects", "metric": "count", "group_by": "type"})})
	f.want(emptyQuery, 201, "projects report")
	if r := result(admin, "/reports/"+emptyQuery.body["id"].(string)); r.body["result"].(map[string]any)["total"] != 0.0 {
		t.Fatalf("empty report = %+v", r.body)
	}

	// The data source can be limited to Projects or Folders, and several can be combined.
	project := f.queryID(`INSERT INTO items(type,title,created_by) VALUES ('project','Run project',$1) RETURNING id::text`, admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, shared)
	child := f.queryID(`INSERT INTO items(type,title,created_by,status_id) VALUES ('task','Inside project',$1,$2) RETURNING id::text`, admin.id, active)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'U')", child, shared, project)
	inside := f.call(admin, "POST", "/analytics/query", map[string]any{"reporting_on": "tasks", "metric": "count", "container_ids": []string{project}})
	f.want(inside, 200, "container scope")
	if inside.body["total"] != 1.0 {
		t.Fatalf("tasks inside the Project = %v, want 1", inside.body["total"])
	}
	f.want(f.call(admin, "POST", "/analytics/query", map[string]any{"reporting_on": "tasks", "metric": "count", "container_ids": []string{"x"}}), 422, "bad container id")

	// CSV export: header, one row per group, formulas defused, same rights as the result.
	time := f.call(admin, "POST", "/reports", map[string]any{"name": "Time per task", "space_id": shared, "sharing": "space",
		"definition": def(map[string]any{"reporting_on": "time_entries", "metric": "minutes", "group_by": "item", "space_ids": both})})
	f.want(time, 201, "time report")
	exportPath := "/reports/" + time.body["id"].(string) + "/export"
	status, header, csv := f.rawGet(admin, exportPath)
	if status != 200 {
		t.Fatalf("export status = %d", status)
	}
	if ct := header.Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Fatalf("content type = %q", ct)
	}
	if header.Get("Content-Disposition") == "" {
		t.Fatal("export must download as a file")
	}
	want := "item,minutes\n\"'=HYPERLINK(\"\"http://evil.example\"\",\"\"click\"\")\",45\n"
	if csv != want {
		t.Fatalf("csv = %q, want %q", csv, want)
	}
	if status, _, _ := f.rawGet(stranger, exportPath); status != 404 {
		t.Fatalf("stranger export = %d, want 404", status)
	}
	if status, _, _ := f.rawGet(reader, exportPath); status != 200 {
		t.Fatalf("reader export = %d, want 200", status)
	}
}
