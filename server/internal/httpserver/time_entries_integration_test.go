package httpserver_test

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func TestTimeEntryAuthorization(t *testing.T) {
	f := newFixture(t)
	admin, owner, viewer, stranger := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Time entry")
	f.share(space, "user", owner.id, "limited")
	f.share(space, "user", viewer.id, "read")
	task := customFieldItem(f, admin, space, "Track time")
	empty := f.call(owner, "GET", "/time-entries?from=2026-11-13&to=2026-11-13", nil)
	f.want(empty, 200, "new timesheet has an empty page")
	if _, ok := empty.body["items"].([]any); !ok || len(empty.items()) != 0 {
		t.Fatalf("expected a JSON array for an empty timesheet: %+v", empty)
	}
	input := map[string]any{"item_id": task, "date": "2026-11-13", "minutes": 90, "category": "Delivery", "comment": "Notes", "billing_type": "billable"}
	f.want(f.call(viewer, "POST", "/time-entries", input), 403, "read cannot log time")
	f.want(f.call(stranger, "POST", "/time-entries", input), 404, "hidden task")
	f.want(f.call(owner, "POST", "/time-entries", map[string]any{"item_id": task, "date": "2026-02-30", "minutes": 90}), 422, "invalid date")
	created := f.call(owner, "POST", "/time-entries", input)
	f.want(created, 201, "participant logs time")
	path := "/time-entries/" + created.body["id"].(string)
	f.want(f.call(owner, "POST", "/time-entries", input), 409, "same user task and day")
	f.want(f.call(viewer, "GET", path, nil), 404, "other user's entry is private")
	f.want(f.call(admin, "GET", path, nil), 200, "full role can read")
	f.want(f.call(owner, "PATCH", path, map[string]int{"minutes": 20}), 428, "writes require If-Match")
	f.want(f.call(admin, "PATCH", path, map[string]int{"minutes": 20}, "If-Match", `"1"`), 403, "manager cannot edit another user's hours")
	f.want(f.call(owner, "PATCH", path, map[string]bool{"locked": true}, "If-Match", `"1"`), 403, "participant cannot lock")
	locked := f.call(admin, "PATCH", path, map[string]bool{"locked": true}, "If-Match", `"1"`)
	f.want(locked, 200, "manager locks")
	f.want(f.call(owner, "DELETE", path, nil, "If-Match", locked.header.Get("ETag")), 409, "locked entries cannot be deleted")
	f.want(f.call(owner, "PATCH", path, map[string]int{"minutes": 20}, "If-Match", locked.header.Get("ETag")), 409, "locked entries cannot be edited")
	unlocked := f.call(admin, "PATCH", path, map[string]bool{"locked": false}, "If-Match", locked.header.Get("ETag"))
	f.want(unlocked, 200, "manager unlocks")
	f.want(f.call(owner, "PATCH", path, map[string]int{"minutes": 1441}, "If-Match", unlocked.header.Get("ETag")), 422, "invalid duration")
	f.want(f.call(owner, "PATCH", path, map[string]int{"minutes": 20}, "If-Match", `"1"`), 409, "stale version")
	list := f.call(owner, http.MethodGet, "/time-entries?from=2026-11-13&to=2026-11-13", nil)
	f.want(list, 200, "list own week")
	if len(list.items()) != 1 || list.items()[0]["minutes"] != float64(90) {
		t.Fatalf("unexpected list %+v", list)
	}
	f.want(f.call(owner, "DELETE", path, nil, "If-Match", unlocked.header.Get("ETag")), 204, "delete own entry")
	created = f.call(owner, "POST", "/time-entries", input)
	f.want(created, 201, "recreate the empty cell")
	f.want(f.call(admin, "DELETE", "/items/"+task, nil), 204, "trash task")
	f.want(f.call(owner, "GET", "/time-entries/"+created.body["id"].(string), nil), 404, "deleted task hides old entries")
}

func TestProjectTimelog(t *testing.T) {
	f := newFixture(t)
	admin, owner, viewer := f.person("admin"), f.person("regular"), f.person("regular")
	space, other := f.space("Project Timelog"), f.space("Other placement")
	f.share(space, "user", owner.id, "limited")
	f.share(space, "user", viewer.id, "read")
	f.share(other, "user", viewer.id, "full")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Project',$1) RETURNING id::text", admin.id)
	task := customFieldItem(f, admin, space, "Nested task")
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U'),($3,$4,'U')", project, space, task, other)
	f.exec("UPDATE item_locations SET parent_item_id=$1 WHERE item_id=$2 AND space_id=$3", project, task, space)
	created := f.call(owner, "POST", "/time-entries", map[string]any{"item_id": task, "date": "2026-11-13", "minutes": 90, "category": "Delivery", "billing_type": "billable"})
	f.want(created, 201, "nested task entry")
	path := "/items/" + project + "/time-entries"
	f.want(f.call(admin, "POST", "/views", map[string]any{"scope_item_id": project, "name": "Timelog", "type": "timelog", "visibility": "public"}), 201, "create project view")
	f.want(f.call(admin, "POST", "/views", map[string]any{"scope_space_id": space, "name": "Timelog", "type": "timelog", "visibility": "public"}), 422, "reject outside project")
	for _, tc := range []struct {
		person *person
		count  int
	}{{admin, 1}, {owner, 1}, {viewer, 0}} {
		result := f.call(tc.person, "GET", path+"?category=delivery&billing_type=billable&sort=-duration", nil)
		f.want(result, 200, "read permitted project entries")
		if len(result.items()) != tc.count {
			t.Fatalf("unexpected entries: %+v", result)
		}
	}
	f.exec(`INSERT INTO time_entries(item_id,user_id,entry_date,minutes)
 SELECT $1,$2,DATE '2000-01-01'+n,1 FROM generate_series(1,10000) n`, task, admin.id)
	f.exec("ANALYZE time_entries")
	start := time.Now()
	last := f.call(admin, "GET", path+"?limit=50&offset=9950&sort=date", nil)
	f.want(last, 200, "10,001 entries with deep page")
	if len(last.items()) != 50 || last.body["next_offset"] != float64(10000) {
		t.Fatalf("incorrect pagination: %+v", last)
	}
	t.Logf("10,001 entries, deep page: %s", time.Since(start))
}

func TestProjectBudget(t *testing.T) {
	f := newFixture(t)
	admin, reader := f.person("admin"), f.person("regular")
	space := f.space("Budget")
	f.share(space, "user", reader.id, "read")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Project',$1) RETURNING id::text", admin.id)
	task := customFieldItem(f, admin, space, "Task")
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, space)
	f.exec("UPDATE item_locations SET parent_item_id=$1 WHERE item_id=$2", project, task)
	field := f.call(admin, "POST", "/spaces/"+space+"/custom-fields", map[string]any{"type": "currency", "name": "Budget", "config": map[string]any{"currency": "USD", "project_budget": true}})
	f.want(field, 201, "currency budget definition")
	body := map[string]any{"fields": map[string]any{field.body["id"].(string): 1250.75}}
	f.want(f.call(admin, "PATCH", "/items/"+task, body, "If-Match", `"1"`), 422, "budget only applies to project")
	f.want(f.call(reader, "PATCH", "/items/"+project, body, "If-Match", `"1"`), 403, "read access cannot edit budget")
	f.want(f.call(admin, "PATCH", "/items/"+project, body, "If-Match", `"1"`), 200, "persist currency budget")
	f.want(f.call(admin, "POST", "/time-entries", map[string]any{"item_id": task, "date": "2026-11-13", "minutes": 135}), 201, "log time")
	path := "/items/" + project + "/time-summary"
	for _, tc := range []struct {
		actor   *person
		minutes float64
		scope   string
	}{{admin, 135, "project"}, {reader, 0, "permitted_entries"}} {
		res := f.call(tc.actor, "GET", path, nil)
		f.want(res, 200, "get authorized hours")
		if res.body["actual_minutes"] != tc.minutes || res.body["scope"] != tc.scope {
			t.Fatalf("unexpected summary: %+v", res)
		}
	}
}

func TestBrowserM4Flows(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 for Chrome with PostgreSQL")
	}
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	space := f.space("M4 delivery")
	f.share(space, "user", member.id, "limited")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','M4 project',$1) RETURNING id::text", admin.id)
	task := customFieldItem(f, admin, space, "M4 delivery task")
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, space)
	f.exec("UPDATE item_locations SET parent_item_id=$1 WHERE item_id=$2", project, task)
	field := f.call(admin, "POST", "/spaces/"+space+"/custom-fields", map[string]any{"type": "currency", "name": "Budget", "config": map[string]any{"currency": "USD", "project_budget": true}})
	f.want(field, 201, "seed project budget definition")
	runBrowserFixture(t, f, []string{"m4-live.spec.ts"},
		"PMS_M4_PROJECT="+project, "PMS_M4_TASK="+task,
		"PMS_M4_ADMIN_COOKIE="+admin.cookie.String(), "PMS_M4_MEMBER_COOKIE="+member.cookie.String())
}
