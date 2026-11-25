package httpserver_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestTimeEntryAuthorization(t *testing.T) {
	f := newFixture(t)
	admin, owner, viewer, stranger := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Time entry")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
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
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates the project and its views as an explicit member
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
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
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

func TestRatesSnapshotKeepHistoryAndCostIsAdminOnly(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	space := f.space("Cost")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	f.share(space, "user", member.id, "editor")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','P',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", project, space)
	task := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','T',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'A')", task, space, project)
	role := f.queryID("INSERT INTO resource_roles(name) VALUES ('Dev') RETURNING id::text")
	f.exec("INSERT INTO resource_user_roles(user_id,role_id) VALUES ($1,$2)", member.id, role)

	f.want(f.call(member, "POST", "/rates", map[string]any{"user_id": member.id, "rate_minor": 5000, "effective_from": "2026-01-01"}), 403, "members cannot set rates")
	f.want(f.call(member, "GET", "/rates", nil), 403, "members cannot read rates")
	f.want(f.call(admin, "POST", "/rates", map[string]any{"rate_minor": 5000, "effective_from": "2026-01-01"}), 422, "needs a subject")
	f.want(f.call(admin, "POST", "/rates", map[string]any{"user_id": member.id, "role_id": role, "rate_minor": 1, "effective_from": "2026-01-01"}), 422, "not both")
	f.want(f.call(admin, "POST", "/rates", map[string]any{"user_id": member.id, "rate_minor": -1, "effective_from": "2026-01-01"}), 422, "negative")
	f.want(f.call(admin, "POST", "/rates", map[string]any{"role_id": role, "rate_minor": 4000, "effective_from": "2026-01-01"}), 201, "role rate")
	f.want(f.call(admin, "POST", "/rates", map[string]any{"user_id": member.id, "rate_minor": 6000, "effective_from": "2026-03-01"}), 201, "user rate")
	f.want(f.call(admin, "POST", "/rates", map[string]any{"user_id": member.id, "rate_minor": 7000, "effective_from": "2026-03-01"}), 422, "duplicate date")

	entry := func(date string, minutes int, billing string) {
		f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes,billing_type,status) VALUES ($1,$2,$3,$4,$5,'approved')", task, member.id, date, minutes, billing)
	}
	entry("2026-02-10", 60, "billable")      // before the personal rate: the role rate applies
	entry("2026-03-10", 90, "billable")      // personal rate
	entry("2026-03-11", 600, "non_billable") // never billed
	// A later change must not touch what was already logged.
	f.want(f.call(admin, "POST", "/rates", map[string]any{"user_id": member.id, "rate_minor": 9000, "effective_from": "2026-03-05"}), 201, "new version")
	entry("2026-03-12", 60, "billable")
	// 1h*4000 + 1.5h*6000 + 1h*9000 = 22000
	sum := f.call(admin, "GET", "/items/"+project+"/time-summary", nil)
	f.want(sum, 200, "summary")
	if sum.body["actual_cost_minor"] != float64(22000) || sum.body["actual_minutes"] != float64(810) {
		t.Fatalf("cost: %v", sum.body)
	}
	hidden := f.call(member, "GET", "/items/"+project+"/time-summary", nil)
	f.want(hidden, 200, "member summary")
	if _, leaked := hidden.body["actual_cost_minor"]; leaked {
		t.Fatalf("a member must not see cost: %v", hidden.body)
	}
}

// DEF-011: log, submit, review. Cost and approved minutes follow the approval; nobody reviews their own time; a submitted entry is
// frozen until a reviewer rejects it. DEF-010: a Workspace Admin who sees only part of the entries is told no cost.
func TestTimesheetSubmitAndApprove(t *testing.T) {
	f := newFixture(t)
	manager, member, peer := f.person("admin"), f.person("regular"), f.person("regular")
	onlooker := f.unlinkedAdmin()
	space := f.spaceWithoutAdmins("Timesheet")
	f.share(space, "user", manager.id, "full")
	f.share(space, "user", member.id, "editor")
	f.share(space, "user", peer.id, "editor")
	f.share(space, "user", onlooker.id, "read")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','P',$1) RETURNING id::text", manager.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", project, space)
	task := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','T',$1) RETURNING id::text", manager.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'A')", task, space, project)
	f.want(f.call(manager, "POST", "/rates", map[string]any{"user_id": member.id, "rate_minor": 6000, "effective_from": "2026-01-01"}), 201, "rate")
	log := func(p *person, date string) string {
		made := f.call(p, "POST", "/time-entries", map[string]any{"item_id": task, "date": date, "minutes": 60, "billing_type": "billable"})
		f.want(made, 201, "log")
		if made.body["status"] != "draft" {
			t.Fatalf("new time is a draft: %v", made.body)
		}
		return made.body["id"].(string)
	}
	inWeek, later := log(member, "2026-11-23"), log(member, "2026-12-01")
	own := log(manager, "2026-11-23")
	summary := func(p *person) map[string]any {
		got := f.call(p, "GET", "/items/"+project+"/time-summary", nil)
		f.want(got, 200, "summary")
		return got.body
	}
	if got := summary(manager); got["actual_cost_minor"] != float64(0) || got["approved_minutes"] != float64(0) || got["actual_minutes"] != float64(180) {
		t.Fatalf("unapproved time costs nothing: %v", got)
	}
	week := map[string]any{"from": "2026-11-23", "to": "2026-11-29"}
	if got := f.call(member, "POST", "/time-entries/submit", week); got.status != 200 || got.body["submitted"] != float64(1) {
		t.Fatalf("submit only the week: %+v", got)
	}
	f.want(f.call(manager, "POST", "/time-entries/submit", week), 200, "manager submits too")
	f.want(f.call(member, "POST", "/time-entries/submit", map[string]any{"from": "2026-11-29", "to": "2026-11-23"}), 422, "bad range")
	f.want(f.call(member, "PATCH", "/time-entries/"+inWeek, map[string]any{"minutes": 90}, "If-Match", `"2"`), 409, "a submitted entry is frozen")
	f.want(f.call(member, "DELETE", "/time-entries/"+inWeek, nil, "If-Match", `"2"`), 409, "and cannot be deleted")

	list := f.call(manager, "GET", "/time-entries/review", nil)
	if list.status != 200 || len(list.items()) != 1 || list.items()[0]["id"] != inWeek {
		t.Fatalf("the reviewer sees other people's submitted time only: %v", list.body)
	}
	for _, p := range []*person{member, peer, onlooker} {
		if got := f.call(p, "GET", "/time-entries/review", nil); got.status != 200 || len(got.items()) != 0 {
			t.Fatalf("no Full access, nothing to review: %v", got.body)
		}
	}
	review := func(p *person, id string, version int, decision, note string) reply {
		return f.call(p, "POST", "/time-entries/"+id+"/review", map[string]any{"decision": decision, "note": note}, "If-Match", fmt.Sprintf(`"%d"`, version))
	}
	f.want(review(peer, inWeek, 2, "approved", ""), 404, "a peer cannot review")
	f.want(review(member, inWeek, 2, "approved", ""), 403, "nobody reviews their own time")
	f.want(review(manager, own, 2, "approved", ""), 403, "not even with Full access")
	f.want(review(manager, later, 1, "approved", ""), 409, "a draft cannot be reviewed")
	f.want(review(manager, inWeek, 1, "approved", ""), 409, "stale version")
	f.want(review(manager, inWeek, 2, "maybe", ""), 422, "unknown decision")
	f.want(review(manager, inWeek, 2, "rejected", "Wrong task"), 200, "reject")
	f.want(f.call(member, "PATCH", "/time-entries/"+inWeek, map[string]any{"minutes": 90}, "If-Match", `"3"`), 200, "a rejected entry can be corrected")
	f.want(f.call(member, "POST", "/time-entries/submit", week), 200, "submit again")
	approved := review(manager, inWeek, 5, "approved", "")
	f.want(approved, 200, "approve")
	if approved.body["status"] != "approved" || approved.body["review_note"] != "" {
		t.Fatalf("approved: %v", approved.body)
	}
	if by := f.queryID("SELECT reviewed_by::text FROM time_entries WHERE id=$1 AND reviewed_at IS NOT NULL", inWeek); by != manager.id {
		t.Fatalf("the reviewer is stored: %s", by)
	}
	if got := summary(manager); got["actual_cost_minor"] != float64(9000) || got["approved_minutes"] != float64(90) {
		t.Fatalf("approved time costs: %v", got)
	}
	partial := summary(onlooker)
	if _, told := partial["actual_cost_minor"]; told || partial["scope"] != "permitted_entries" {
		t.Fatalf("a partial cost is a wrong cost, so it is left out: %v", partial)
	}
	f.want(review(manager, inWeek, 6, "rejected", "Reopen"), 200, "rejecting an approved entry reopens it")
	if got := summary(manager); got["actual_cost_minor"] != float64(0) {
		t.Fatalf("a reopened entry costs nothing: %v", got)
	}
}
