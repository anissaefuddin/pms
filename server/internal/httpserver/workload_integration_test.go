package httpserver_test

import (
	"os"
	"slices"
	"testing"
	"time"
)

func TestResourceWorkload(t *testing.T) {
	f := newFixture(t)
	admin, first, second := f.person("admin"), f.person("regular"), f.person("regular")
	space, extra := f.space("Workload"), f.space("Second placement")
	f.share(space, "user", first.id, "editor")
	f.share(space, "user", second.id, "editor")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Project',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", project, space)
	task := f.queryID(`INSERT INTO items(type,title,created_by,status_id,effort_minutes,start_date,due_date)
 VALUES('task','Task',$1,(SELECT id FROM statuses WHERE grp='active' LIMIT 1),601,'2026-11-16','2026-11-20') RETURNING id::text`, admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'A'),($1,$4,NULL,'A')", task, space, project, extra)
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES($1,$2),($1,$3)", task, first.id, second.id)
	f.exec("INSERT INTO work_schedule_exceptions(day,name) VALUES('2026-11-18','Holiday')")
	capPath := "/users/" + first.id + "/capacity"
	in := map[string]any{"working_days": []int{1, 2, 3, 4, 5}, "daily_hours": 4, "leave_dates": []string{"2026-11-17"}}
	f.want(f.call(second, "PUT", capPath, in), 403, "cannot edit another person's capacity")
	f.want(f.call(first, "PUT", capPath, in), 200, "edit own capacity")
	f.want(f.call(admin, "GET", capPath, nil), 200, "read capacity")
	path := "/workload?from=2026-11-16&to=2026-11-20"
	totals := func(data reply, id, key string) int {
		t.Helper()
		for _, raw := range data.body["people"].([]any) {
			p := raw.(map[string]any)
			if p["id"] != id {
				continue
			}
			sum := 0
			for _, v := range p[key].([]any) {
				sum += int(v.(float64))
			}
			return sum
		}
		t.Fatalf("missing person %s", id)
		return 0
	}
	got := f.call(admin, "GET", path, nil)
	f.want(got, 200, "workload")
	if totals(got, first.id, "capacity") != 720 || totals(got, first.id, "planned")+totals(got, second.id, "planned") != 601 {
		t.Fatalf("capacity or workload mismatch: %v", got.body)
	}
	f.want(f.call(admin, "PATCH", "/items/"+task, map[string]any{"assignee_ids": []string{second.id}}, "If-Match", `"1"`), 200, "reassign through item API")
	if audit := f.queryID("SELECT count(*)::text FROM activity WHERE item_id=$1 AND changes ? 'assignee_ids'", task); audit != "1" {
		t.Fatalf("expected one assignment audit event, got %s", audit)
	}
	got = f.call(admin, "GET", path, nil)
	if totals(got, first.id, "planned") != 0 || totals(got, second.id, "planned") != 601 {
		t.Fatalf("workload did not move: %v", got.body)
	}
	f.want(f.call(first, "POST", "/resource-directory", map[string]string{"kind": "role", "name": "Designer"}), 403, "directory is admin managed")
	f.want(f.call(admin, "POST", "/resource-directory", map[string]string{"kind": "role", "name": "Designer"}), 204, "create role")
	f.want(f.call(admin, "POST", "/resource-directory", map[string]string{"kind": "skill", "name": "Illustration"}), 204, "create skill")
	role := f.queryID("SELECT id::text FROM resource_roles WHERE name='Designer'")
	skill := f.queryID("SELECT id::text FROM resource_skills WHERE name='Illustration'")
	for kind, id := range map[string]string{"role": role, "skill": skill} {
		f.want(f.call(admin, "PUT", "/resource-directory", map[string]any{"kind": kind, "id": id, "user_id": first.id, "assigned": true}), 204, "assign directory entry")
	}
	got = f.call(admin, "GET", path+"&role="+role+"&skill="+skill, nil)
	if len(got.body["people"].([]any)) != 1 || totals(got, first.id, "capacity") != 720 {
		t.Fatalf("directory filtering: %v", got.body)
	}
	book := map[string]any{"project_id": project, "user_id": first.id, "start_date": "2026-11-16", "end_date": "2026-11-20", "minutes_per_day": 300}
	f.want(f.call(admin, "POST", "/resource-bookings", book), 204, "book person")
	delete(book, "user_id")
	book["role_id"] = role
	f.want(f.call(admin, "POST", "/resource-bookings", book), 204, "book role")
	got = f.call(admin, "GET", path, nil)
	f.want(got, 200, "booking workload")
	if totals(got, first.id, "booked") != 900 || len(got.body["role_demand"].([]any)) != 1 {
		t.Fatalf("booking totals: %v", got.body)
	}
	demand := got.body["role_demand"].([]any)[0].(map[string]any)
	if demand["booked"].([]any)[0] != float64(300) || demand["capacity"].([]any)[0] != float64(0) {
		t.Fatalf("expected role conflict against remaining capacity: %v", demand)
	}
	f.want(f.call(first, "POST", "/resource-bookings", book), 403, "booking is admin managed")
	f.want(f.call(admin, "DELETE", "/resource-directory?kind=role&id="+role, nil), 409, "booked role is in use")
	for _, raw := range got.body["bookings"].([]any) {
		f.want(f.call(admin, "DELETE", "/resource-bookings/"+raw.(map[string]any)["id"].(string), nil), 204, "remove booking")
	}
	f.want(f.call(admin, "DELETE", "/resource-directory?kind=role&id="+role, nil), 204, "remove unused role")
}

func TestWorkloadPerformance(t *testing.T) {
	if os.Getenv("PMS_PERF") != "1" {
		t.Skip("set PMS_PERF=1 for the workload benchmark")
	}
	f := newFixture(t)
	admin := f.person("admin")
	space, second := f.space("Benchmark"), f.space("Second placement")
	f.exec(`INSERT INTO users(email,first_name,last_name,password_hash,user_type,status)
 SELECT 'load-'||n||'@example.test','Resource',n::text,'x','regular','active' FROM generate_series(1,50)n`)
	f.exec(`WITH created AS (
 INSERT INTO items(type,title,created_by,status_id,effort_minutes,start_date,due_date)
 SELECT 'task','Load '||n,$1,(SELECT id FROM statuses WHERE grp='active' LIMIT 1),481,'2026-11-16','2026-11-27' FROM generate_series(1,2000)n
 RETURNING id), placements AS (
 INSERT INTO item_locations(item_id,space_id,rank) SELECT created.id,loc.id,'A' FROM created CROSS JOIN (VALUES($2::uuid),($3::uuid))loc(id)),
 members AS (SELECT id,row_number() OVER (ORDER BY id) AS n FROM users WHERE email::text LIKE 'load-%'),
 ranked AS (SELECT id,row_number() OVER (ORDER BY id) AS n FROM created)
 INSERT INTO item_assignees(item_id,user_id)
 SELECT ranked.id,members.id FROM ranked JOIN members ON members.n=(ranked.n%50)+1 OR members.n=((ranked.n+1)%50)+1`, admin.id, space, second)
	samples := []time.Duration{}
	for i := range 22 {
		start := time.Now()
		got := f.call(admin, "GET", "/workload?from=2026-11-16&to=2026-11-29", nil)
		f.want(got, 200, "workload benchmark")
		if len(got.body["tasks"].([]any)) != 2000 {
			t.Fatal("benchmark must include all 2,000 visible tasks")
		}
		if i >= 2 {
			samples = append(samples, time.Since(start))
		}
	}
	slices.Sort(samples)
	t.Logf("50 people, 2,000 tasks, 4,000 placements, 20 samples: p95=%v", samples[18])
	if samples[18] >= 1500*time.Millisecond {
		t.Fatal("workload exceeded local 1.5s p95 budget")
	}
}

func TestBrowserWorkload(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("requires Chrome and PostgreSQL")
	}
	f := newFixture(t)
	admin := f.person("admin")
	f.exec("UPDATE users SET first_name='Capacity',last_name='Reviewer' WHERE id=$1", admin.id)
	space := f.space("Workload pilot")
	task := f.queryID(`INSERT INTO items(type,title,created_by,status_id,effort_minutes,start_date,due_date)
 VALUES('task','Review scope',$1,(SELECT id FROM statuses WHERE grp='active' LIMIT 1),600,'2026-11-17','2026-11-17') RETURNING id::text`, admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", task, space)
	runBrowserFixture(t, f, []string{"workload-live.spec.ts"}, "PMS_FIXTURE_WORKLOAD="+admin.cookie.String())
}

func TestUtilizationPlannedVersusActualMatchesRawData(t *testing.T) {
	f := newFixture(t)
	admin, one, two := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Util")
	f.share(space, "user", one.id, "editor")
	task := f.queryID(`INSERT INTO items(type,title,created_by,status_id,effort_minutes,start_date,due_date)
 VALUES('task','Task',$1,(SELECT id FROM statuses WHERE grp='active' LIMIT 1),300,'2026-11-16','2026-11-20') RETURNING id::text`, admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", task, space)
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES($1,$2)", task, one.id)
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES($1,$2,'2026-11-17',120),($1,$2,'2026-11-18',60),($1,$2,'2026-12-30',500)", task, one.id)
	path := "/utilization?from=2026-11-16&to=2026-11-20"
	got := f.call(admin, "GET", path, nil)
	f.want(got, 200, "utilization")
	var row map[string]any
	for _, raw := range got.items() {
		if raw["id"] == one.id {
			row = raw
		}
	}
	// 5 workdays x 8 h = 2400 min capacity; planned = task effort; actual = the two entries inside the range only.
	if row == nil || row["capacity"] != float64(2400) || row["planned"] != float64(300) || row["actual"] != float64(180) || row["planned_percent"] != 12.5 || row["actual_percent"] != 7.5 {
		t.Fatalf("utilization row: %v", row)
	}
	mine := f.call(two, "GET", path, nil)
	f.want(mine, 200, "own row only")
	if len(mine.items()) != 1 || mine.items()[0]["id"] != two.id {
		t.Fatalf("a regular user sees only themselves: %v", mine.body)
	}
	f.want(f.call(admin, "GET", "/utilization?from=2026-11-20&to=2026-11-16", nil), 422, "bad range")
}

func TestBaselinesFreezeDatesAndCannotBeChanged(t *testing.T) {
	f := newFixture(t)
	admin, viewer, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Baselines")
	f.share(space, "user", viewer.id, "read")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','P',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", project, space)
	task := f.queryID("INSERT INTO items(type,title,created_by,start_date,due_date) VALUES ('task','T',$1,'2026-11-16','2026-11-20') RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'A')", task, space, project)
	path := "/items/" + project + "/baselines"
	f.want(f.call(viewer, "POST", path, map[string]any{"name": "Plan"}), 403, "read cannot baseline")
	f.want(f.call(outsider, "POST", path, map[string]any{"name": "Plan"}), 404, "hidden project")
	f.want(f.call(admin, "POST", path, map[string]any{"name": " "}), 422, "empty name")
	made := f.call(admin, "POST", path, map[string]any{"name": "Plan"})
	f.want(made, 201, "baseline")
	if made.body["item_count"] != float64(1) {
		t.Fatalf("one dated item: %v", made.body)
	}
	f.exec("UPDATE items SET start_date='2026-11-18',due_date='2026-11-27' WHERE id=$1", task)
	v := f.call(viewer, "GET", "/baselines/"+made.body["id"].(string)+"/variance", nil)
	f.want(v, 200, "variance")
	row := v.items()[0]
	if row["baseline_due"] != "2026-11-20" || row["due"] != "2026-11-27" || row["start_variance_days"] != float64(2) || row["due_variance_days"] != float64(7) {
		t.Fatalf("variance: %v", row)
	}
	f.want(f.call(outsider, "GET", "/baselines/"+made.body["id"].(string)+"/variance", nil), 404, "hidden")
	f.want(f.call(viewer, "GET", path, nil), 200, "list")
	for _, q := range []string{"UPDATE baselines SET name='x'", "DELETE FROM baseline_items", "UPDATE baseline_items SET due_date=NULL", "TRUNCATE baselines"} {
		if _, err := f.pool.Exec(f.ctx, q); err == nil {
			t.Fatalf("%s must be rejected", q)
		}
	}
	for i := 0; i < 9; i++ {
		f.want(f.call(admin, "POST", path, map[string]any{"name": "More"}), 201, "up to ten")
	}
	f.want(f.call(admin, "POST", path, map[string]any{"name": "Eleventh"}), 422, "limit")
}
