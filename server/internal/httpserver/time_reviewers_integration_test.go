package httpserver_test

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// DEF-042: whoever may review a Task's time is told, however far up their Full access was given; nobody else is.
func TestTimesheetSubmissionReachesProjectLevelReviewers(t *testing.T) {
	f := newFixture(t)
	member, spaceLead, projectLead, twice, reader, outsider, revoked := f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.spaceWithoutAdmins("Reviewers")
	for p, role := range map[*person]string{member: "editor", spaceLead: "full", twice: "full", reader: "read", revoked: "editor"} {
		f.share(space, "user", p.id, role)
	}
	node := func(kind, title, parent string) string {
		id := f.queryID("INSERT INTO items(type,title,created_by) VALUES ($1,$2,$3) RETURNING id::text", kind, title, spaceLead.id)
		if parent == "" {
			f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", id, space)
		} else {
			f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'A')", id, space, parent)
		}
		return id
	}
	project := node("project", "P", "")
	task := node("task", "T", node("task", "Parent", node("folder", "F", project)))
	f.share2(project, projectLead.id, "full")
	f.share2(project, twice.id, "full")
	f.share2(project, revoked.id, "full")
	f.exec("DELETE FROM shares WHERE target_id=$1 AND principal_id=$2", project, revoked.id)
	told := func(p *person) string {
		return f.queryID("SELECT count(*)::text FROM notifications WHERE recipient_id=$1 AND kind='timesheet_submitted'", p.id)
	}
	submit := func(date string) {
		f.want(f.call(member, "POST", "/time-entries/submit", map[string]any{"from": date, "to": date}), 200, "submit")
	}
	made := f.call(member, "POST", "/time-entries", map[string]any{"item_id": task, "date": "2026-11-02", "minutes": 60})
	f.want(made, 201, "log")
	submit("2026-11-02")
	for name, c := range map[string]struct {
		p    *person
		want string
	}{"Full on the Space": {spaceLead, "1"}, "Full on the project": {projectLead, "1"}, "Full on both": {twice, "1"}, "Read": {reader, "0"},
		"no grant": {outsider, "0"}, "revoked": {revoked, "0"}, "the author": {member, "0"}} {
		if got := told(c.p); got != c.want {
			t.Errorf("%s: %s notices, want %s", name, got, c.want)
		}
	}
	// The notice leads somewhere the reviewer may go, and a rejected entry sent again tells them again.
	id := made.body["id"].(string)
	review := f.call(projectLead, "GET", "/time-entries/review", nil)
	if review.status != 200 || len(review.items()) != 1 {
		t.Fatalf("project reviewer's queue: %v", review.body)
	}
	f.want(f.call(projectLead, "POST", "/time-entries/"+id+"/review", map[string]any{"decision": "rejected", "note": "again"}, "If-Match", `"2"`), 200, "reject")
	submit("2026-11-02")
	if got := told(projectLead); got != "2" {
		t.Errorf("resubmission: %s", got)
	}
}

// Measurement for finding C, run by hand: PMS_PERF=1 go test -run TimesheetSubmitCost -v
func TestTimesheetSubmitCost(t *testing.T) {
	if os.Getenv("PMS_PERF") != "1" {
		t.Skip("set PMS_PERF=1 to measure")
	}
	for _, n := range []int{1, 10, 50, 100} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			f := newFixture(t)
			member := f.person("regular")
			space := f.spaceWithoutAdmins("Cost")
			f.share(space, "user", member.id, "editor")
			for i := range 20 { // 20 people with Full: every one is a reviewer
				p := f.person("regular")
				f.share(space, "user", p.id, "full")
				_ = i
			}
			for i := range n { // one entry on each of n Tasks: the worst case
				task := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task',$1,$2) RETURNING id::text", fmt.Sprint("t", i), member.id)
				f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,$3)", task, space, fmt.Sprintf("A%04d", i))
				f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,'2026-11-02',30)", task, member.id)
			}
			db := f.queryID("SELECT current_database()")
			f.exec(fmt.Sprintf(`ALTER DATABASE %q SET log_statement='all'`, db))
			f.pool.Reset() // new sessions pick the setting up
			f.exec(fmt.Sprintf("SELECT 'MARK-start-%d'", n))
			start := time.Now()
			f.want(f.call(member, "POST", "/time-entries/submit", map[string]any{"from": "2026-11-02", "to": "2026-11-02"}), 200, "submit")
			took := time.Since(start)
			f.exec(fmt.Sprintf("SELECT 'MARK-end-%d'", n))
			f.exec(fmt.Sprintf(`ALTER DATABASE %q RESET log_statement`, db))
			t.Logf("MEASURE entries=%d latency=%s", n, took.Round(time.Millisecond))
		})
	}
}
