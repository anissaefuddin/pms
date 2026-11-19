package httpserver_test

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func TestBrowserProjectProgress(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	admin, editor := f.person("admin"), f.person("regular")
	space := f.space("Project progress browser fixture")
	f.share(space, "user", editor.id, "editor")
	create := func(kind, title, parent string) string {
		t.Helper()
		location := map[string]string{"space_id": space}
		if parent != "" {
			location["parent_item_id"] = parent
		}
		res := f.call(admin, http.MethodPost, "/items", map[string]any{
			"type": kind, "title": title, "locations": []any{location},
		})
		f.want(res, http.StatusCreated, "seed "+title)
		return res.body["id"].(string)
	}
	project := create("project", "Progress roadmap", "")
	folder := create("folder", "Nested folder", project)
	task := create("task", "Nested task", folder)
	runBrowserFixture(t, f, []string{"project-progress-live.spec.ts"},
		"PMS_FIXTURE_PROGRESS_SPACE="+space,
		"PMS_FIXTURE_PROGRESS_PROJECT="+project,
		"PMS_FIXTURE_PROGRESS_TASK="+task,
		"PMS_FIXTURE_PROGRESS_ADMIN_COOKIE="+admin.cookie.String(),
		"PMS_FIXTURE_PROGRESS_EDITOR_COOKIE="+editor.cookie.String(),
	)
}

func expectProjectProgress(t *testing.T, res reply, method string, percent int, risk string) {
	t.Helper()
	if res.status != http.StatusOK || res.body["progress_method"] != method ||
		res.body["progress_percent"] != float64(percent) || res.body["progress_risk"] != risk {
		t.Fatalf("project progress = %+v, want %s/%d/%s", res, method, percent, risk)
	}
}

func TestProjectProgressMethodsAndRisk(t *testing.T) {
	f := newFixture(t)
	admin, reader := f.person("admin"), f.person("regular")
	space := f.space("Project progress")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	f.share(space, "user", reader.id, "read")
	now := time.Now().UTC()
	due := func(days int) string { return now.AddDate(0, 0, days).Format(time.DateOnly) }
	completed := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' LIMIT 1")
	cancelled := f.queryID("SELECT id::text FROM statuses WHERE grp='cancelled' LIMIT 1")
	create := func(kind, title, parent string, fields map[string]any) reply {
		t.Helper()
		if fields == nil {
			fields = make(map[string]any)
		}
		fields["type"], fields["title"] = kind, title
		loc := map[string]string{"space_id": space}
		if parent != "" {
			loc["parent_item_id"] = parent
		}
		fields["locations"] = []any{loc}
		res := f.call(admin, http.MethodPost, "/items", fields)
		f.want(res, http.StatusCreated, "create "+title)
		return res
	}
	project := create("project", "Roadmap", "", nil)
	id := project.body["id"].(string)
	path := "/items/" + id
	read := func() reply { return f.call(reader, http.MethodGet, path, nil) }
	expectProjectProgress(t, read(), "task_count", 0, "green")
	folder := create("folder", "Folder", id, nil).body["id"].(string)
	child := create("task", "Completed", folder, map[string]any{
		"status_id": completed, "duration_days": 1, "due_date": due(-1), "progress_percent": 15,
	}).body["id"].(string)
	open := create("task", "Partial subtask", child, map[string]any{
		"duration_days": 3, "due_date": due(2), "progress_percent": 40,
	})
	taskPath := "/items/" + open.body["id"].(string)
	create("task", "Cancelled", id, map[string]any{"status_id": cancelled, "due_date": due(-3), "progress_percent": 100})
	deleted := create("task", "Deleted", id, map[string]any{"due_date": due(-3)}).body["id"].(string)
	f.want(f.call(admin, http.MethodDelete, "/items/"+deleted, nil), http.StatusNoContent, "delete task before computing progress")

	expectProjectProgress(t, read(), "task_count", 50, "yellow")
	f.want(f.call(reader, http.MethodPatch, path, map[string]string{"progress_method": "weighted_duration"}, "If-Match", `"1"`),
		http.StatusForbidden, "project method needs Edit")
	f.want(f.call(admin, http.MethodPatch, path, map[string]any{"progress_percent": 50}, "If-Match", `"1"`),
		http.StatusUnprocessableEntity, "project progress is derived")
	f.want(f.call(admin, http.MethodPatch, taskPath, map[string]string{"progress_method": "weighted_duration"}, "If-Match", `"1"`),
		http.StatusUnprocessableEntity, "task cannot choose a project method")
	f.want(f.call(admin, http.MethodPatch, taskPath, map[string]any{"progress_percent": 101}, "If-Match", `"1"`),
		http.StatusUnprocessableEntity, "task percent must be in range")
	weighted := f.call(admin, http.MethodPatch, path, map[string]string{"progress_method": "weighted_duration"}, "If-Match", `"1"`)
	expectProjectProgress(t, weighted, "weighted_duration", 55, "yellow")
	f.want(f.call(admin, http.MethodPatch, path, map[string]string{"progress_method": "average_percent"}, "If-Match", `"1"`),
		http.StatusConflict, "stale project version")
	average := f.call(admin, http.MethodPatch, path, map[string]string{"progress_method": "average_percent"}, "If-Match", weighted.header.Get("ETag"))
	expectProjectProgress(t, average, "average_percent", 70, "yellow")
	list := f.call(reader, http.MethodGet, "/spaces/"+space+"/items", nil)
	f.want(list, http.StatusOK, "list Space roots for Table")
	for _, row := range list.items() {
		if row["id"] == id {
			expectProjectProgress(t, reply{status: http.StatusOK, body: row}, "average_percent", 70, "yellow")
		}
	}
	if rows := list.items(); len(rows) != 1 || rows[0]["id"] != id {
		t.Fatalf("expected project as the only live root: %v", rows)
	}

	changed := f.call(admin, http.MethodPatch, taskPath, map[string]any{
		"progress_percent": nil, "due_date": due(-1),
	}, "If-Match", `"1"`)
	f.want(changed, http.StatusOK, "clear task percent and make it overdue")
	if changed.body["progress_percent"] != nil {
		t.Fatalf("manual value was not cleared: %+v", changed)
	}
	overdue := read()
	expectProjectProgress(t, overdue, "average_percent", 50, "red")
	if overdue.header.Get("ETag") != average.header.Get("ETag") {
		t.Fatalf("derived progress should not change the Project ETag: %q versus %q", overdue.header.Get("ETag"), average.header.Get("ETag"))
	}
	closed := f.call(admin, http.MethodPatch, path, map[string]string{"status_id": completed}, "If-Match", overdue.header.Get("ETag"))
	expectProjectProgress(t, closed, "average_percent", 50, "green")
}

func TestProjectProgressFollowsAuthorizedLiveLocations(t *testing.T) {
	f := newFixture(t)
	admin, reader := f.person("admin"), f.person("regular")
	direct, personalOwner := f.person("regular"), f.person("regular")
	visible, hidden := f.space("Visible placements"), f.space("Hidden placements")
	personal := f.queryID("SELECT id::text FROM spaces WHERE kind='personal' AND owner_id=$1", personalOwner.id)
	f.share(visible, "user", reader.id, "read")
	f.share(visible, "user", admin.id, "full") // A1.2: the admin places and removes the project's folders as an explicit member
	completed := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' LIMIT 1")
	overdue := time.Now().UTC().AddDate(0, 0, -1).Format(time.DateOnly)
	place := func(id, space, parent string) {
		t.Helper()
		f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,NULLIF($3,'')::uuid,'U')", id, space, parent)
	}
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Multi-space project',$1) RETURNING id::text", admin.id)
	for _, space := range []string{visible, hidden, personal} {
		place(project, space, "")
	}
	done := f.queryID("INSERT INTO items(type,title,created_by,status_id) VALUES ('task','Shared completed',$1,$2) RETURNING id::text", admin.id, completed)
	place(done, visible, project)
	place(done, hidden, project)
	makeOverdue := func(title string) string {
		return f.queryID("INSERT INTO items(type,title,created_by,due_date) VALUES ('task',$1,$2,$3::text::date) RETURNING id::text", title, admin.id, overdue)
	}
	hiddenOverdue := makeOverdue("Hidden overdue")
	place(hiddenOverdue, hidden, project)
	place(makeOverdue("Foreign personal overdue"), personal, project)
	folder := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('folder','Deleted placement',$1) RETURNING id::text", admin.id)
	place(folder, visible, project)
	survivor := makeOverdue("Survives outside project")
	place(survivor, visible, folder)
	place(survivor, visible, "")
	path := "/items/" + project
	expectProjectProgress(t, f.call(reader, http.MethodGet, path, nil), "task_count", 50, "red")
	f.want(f.call(admin, http.MethodDelete, "/items/"+folder, nil), http.StatusNoContent, "remove an intermediate project placement")
	f.want(f.call(reader, http.MethodGet, "/items/"+survivor, nil), http.StatusOK, "task remains live outside project")
	expectProjectProgress(t, f.call(reader, http.MethodGet, path, nil), "task_count", 100, "green")
	// D-17 A1: the admin's progress counts only the placements the admin can see, the same as the reader's view.
	expectProjectProgress(t, f.call(admin, http.MethodGet, path, nil), "task_count", 100, "green")
	directProject := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Direct share',$1) RETURNING id::text", admin.id)
	place(directProject, hidden, "")
	place(done, hidden, directProject)
	place(hiddenOverdue, hidden, directProject)
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,'user',$2,'read')", directProject, direct.id)
	directView := f.call(direct, http.MethodGet, "/items/"+directProject, nil)
	expectProjectProgress(t, directView, "task_count", 50, "red")
	if len(directView.body["locations"].([]any)) != 0 {
		t.Fatalf("project directly shared across hidden spaces disclosed placement metadata: %v", directView.body)
	}
}
