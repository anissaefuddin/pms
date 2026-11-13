package httpserver_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/analytics"
)

// TestReportTemplatesProduceTheRightNumbers builds one fixture where every template has a known answer, creates a
// report from each template through the API and compares the result with the hand-counted raw data.
func TestReportTemplatesProduceTheRightNumbers(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	shared := f.space("Template data")
	f.share(shared, "user", member.id, "read")
	active := f.queryID("SELECT id::text FROM statuses WHERE grp='active' ORDER BY name LIMIT 1")
	done := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' ORDER BY name LIMIT 1")
	activeName := f.queryID("SELECT name FROM statuses WHERE id=$1", active)
	doneName := f.queryID("SELECT name FROM statuses WHERE id=$1", done)
	item := func(kind, title, status, due string) string {
		id := f.queryID(`INSERT INTO items(type,title,created_by,status_id,due_date) VALUES ($1::item_type,$2,$3,$4::uuid,NULLIF($5,'')::date) RETURNING id::text`, kind, title, admin.id, status, due)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, shared)
		return id
	}
	today := f.queryID("SELECT current_date::text")
	past := f.queryID("SELECT (current_date - 60)::text")
	far := f.queryID("SELECT (current_date + 400)::text")
	thisMonth := item("task", "Due this month", active, today)
	overdue := item("task", "Overdue", active, past)
	item("task", "Unassigned and far away", active, far)
	finished := item("task", "Finished long ago", done, past)
	item("project", "Running project", active, "")
	item("project", "Closed project", done, "")
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", thisMonth, member.id)
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", overdue, admin.id)
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", finished, member.id)
	lastMonth := f.queryID("SELECT (date_trunc('month', current_date) - interval '1 day')::date::text")
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,$3,480)", thisMonth, member.id, today)
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,$3,240)", overdue, admin.id, today)
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,$3,1000)", thisMonth, member.id, lastMonth) // outside the period
	workdays := f.queryID(`SELECT count(*)::text FROM generate_series(date_trunc('month', current_date), date_trunc('month', current_date) + interval '1 month - 1 day', interval '1 day') d
WHERE extract(dow FROM d) BETWEEN 1 AND 5`)
	var capacity float64
	fmt.Sscan(workdays, &capacity)
	capacity *= 8 * 60 // the default schedule: Monday to Friday, 8 hours
	percent := func(minutes float64) float64 { return math.Round(minutes/capacity*1000) / 10 }

	// The template list is the single source: every definition must be valid and have a unique key.
	list := f.call(admin, "GET", "/report-templates", nil)
	f.want(list, 200, "list templates")
	if got := len(list.items()); got != len(analytics.Templates) || got != 7 {
		t.Fatalf("templates = %d, want 7", got)
	}
	for _, tpl := range analytics.Templates {
		if err := tpl.Query.Validate(); err != nil {
			t.Fatalf("template %s is invalid: %v", tpl.Key, err)
		}
	}

	run := func(who *person, key string) map[string]float64 {
		t.Helper()
		created := f.call(who, "POST", "/reports", map[string]any{"name": "From " + key, "template": key})
		f.want(created, 201, "create from "+key)
		result := f.call(who, "GET", "/reports/"+created.body["id"].(string)+"/result", nil)
		f.want(result, 200, "run "+key)
		return rowsOf(reply{body: result.body["result"].(map[string]any)})
	}
	same := func(name string, got, want map[string]float64) {
		t.Helper()
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
	same("active tasks by assignee", run(admin, "active_tasks_by_assignee"), map[string]float64{member.id: 1, admin.id: 1, "": 1})
	same("overdue", run(admin, "overdue"), map[string]float64{admin.id: 1}) // the completed old task is not overdue
	same("due this month", run(admin, "due_this_month"), map[string]float64{member.id: 1})
	same("unassigned", run(admin, "unassigned"), map[string]float64{shared: 1})
	same("weekly project status", run(admin, "weekly_project_status"), map[string]float64{activeName: 1, doneName: 1})
	same("time spent", run(admin, "time_spent"), map[string]float64{member.id: 480, admin.id: 240})
	same("utilization", run(admin, "utilization"), map[string]float64{member.id: percent(480), admin.id: percent(240)})
	// Rights apply to the person who opens the report: the member does not see the admin's overdue task.
	same("overdue as member", run(member, "overdue"), map[string]float64{admin.id: 1}) // read access on the Space shows it
	f.exec("DELETE FROM shares WHERE principal_id=$1", member.id)
	same("overdue without access", run(member, "overdue"), map[string]float64{})

	// The utilization average is the mean of the people listed.
	total := f.call(admin, "GET", "/reports/"+f.queryID("SELECT id::text FROM reports WHERE name='From utilization'")+"/result", nil).body["result"].(map[string]any)["total"].(float64)
	if want := math.Round((percent(480)+percent(240))/2*10) / 10; math.Abs(total-want) > 0.11 {
		t.Errorf("utilization total = %v, want about %v", total, want)
	}

	// A template and a definition cannot be mixed; unknown templates are rejected; a blank report still works.
	f.want(f.call(admin, "POST", "/reports", map[string]any{"name": "x", "template": "nope"}), 422, "unknown template")
	f.want(f.call(admin, "POST", "/reports", map[string]any{"name": "x", "template": "overdue", "definition": map[string]any{"layout": "table", "query": map[string]any{"reporting_on": "tasks", "metric": "count"}}}), 422, "template with definition")
	f.want(f.call(admin, "POST", "/reports", map[string]any{"name": "Blank"}), 201, "report from scratch")

	// Utilization is only defined per person and over a period.
	query := func(q map[string]any) reply { return f.call(admin, "POST", "/analytics/query", q) }
	f.want(query(map[string]any{"reporting_on": "time_entries", "metric": "utilization", "group_by": "item", "filters": map[string]any{"entry_this_month": true}}), 422, "utilization by item")
	f.want(query(map[string]any{"reporting_on": "time_entries", "metric": "utilization", "group_by": "user"}), 422, "utilization without a period")
	f.want(query(map[string]any{"reporting_on": "tasks", "metric": "count", "filters": map[string]any{"entry_this_month": true}}), 422, "entry period on tasks")
	f.want(query(map[string]any{"reporting_on": "time_entries", "metric": "minutes", "filters": map[string]any{"due_this_month": true}}), 422, "due period on time entries")
	f.want(query(map[string]any{"reporting_on": "time_entries", "metric": "utilization", "group_by": "user", "filters": map[string]any{"entry_from": "2026-05-01", "entry_to": "2026-04-01"}}), 422, "reversed period")
	f.exec("DELETE FROM work_schedule_exceptions")
	f.exec("UPDATE work_schedules SET working_days='{0}'")
	if r := query(map[string]any{"reporting_on": "time_entries", "metric": "utilization", "group_by": "user", "filters": map[string]any{"entry_from": "2026-05-04", "entry_to": "2026-05-08"}}); r.status != 422 {
		t.Fatalf("a period without working days = %d, want 422", r.status)
	}
}
