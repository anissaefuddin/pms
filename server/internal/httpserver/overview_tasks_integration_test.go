package httpserver_test

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Workspace Overview Tasks tab: one list across Spaces, with filters that combine. Name prefix TestWorkspaceOverviewTasks.

func ovTaskTitles(t *testing.T, f *fixture, p *person, query string) []string {
	t.Helper()
	res := f.call(p, "GET", "/overview/tasks"+query, nil)
	f.want(res, http.StatusOK, "tasks "+query)
	var titles []string
	for _, item := range res.items() {
		titles = append(titles, item["title"].(string))
	}
	sort.Strings(titles)
	return titles
}

func TestWorkspaceOverviewTasksFilters(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	member := f.person("regular")
	alpha := f.spaceWithoutAdmins("Alpha")
	beta := f.spaceWithoutAdmins("Beta")
	f.share(alpha, "user", member.id, "read")
	f.share(beta, "user", member.id, "read")
	due := func(n int) string { return "(now() AT TIME ZONE 'UTC')::date + " + strconv.Itoa(n) }

	overdue := ovItem(f, admin.id, "task", "Overdue report", "active", due(-3), "", alpha)
	ovItem(f, admin.id, "task", "Beta launch", "active", due(5), "", beta)
	ovItem(f, admin.id, "task", "Shipped report", "completed", due(-1), "now()", alpha)
	ovItem(f, admin.id, "task", "Far away", "deferred", due(200), "", alpha)
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", overdue, member.id)
	ovItem(f, admin.id, "task", "Undated", "active", "", "", beta) // DEF-005: no due date is not overdue, and not an error

	all := []string{"Beta launch", "Far away", "Overdue report", "Shipped report", "Undated"}
	if got := ovTaskTitles(t, f, member, ""); strings.Join(got, "|") != strings.Join(all, "|") {
		t.Fatalf("all tasks = %v, want %v", got, all)
	}
	if got := ovTaskTitles(t, f, member, "?space_id="+alpha); strings.Join(got, "|") != "Far away|Overdue report|Shipped report" {
		t.Fatalf("one Space = %v", got)
	}
	if got := ovTaskTitles(t, f, member, "?space_id="+alpha+"&space_id="+beta); len(got) != 5 {
		t.Fatalf("two Spaces = %v", got)
	}
	if got := ovTaskTitles(t, f, member, "?status=completed"); strings.Join(got, "|") != "Shipped report" {
		t.Fatalf("status completed = %v", got)
	}
	if got := ovTaskTitles(t, f, member, "?status=completed&status=deferred"); len(got) != 2 {
		t.Fatalf("two statuses = %v", got)
	}
	if got := ovTaskTitles(t, f, member, "?preset=overdue"); strings.Join(got, "|") != "Overdue report" {
		t.Fatalf("overdue preset = %v", got)
	}
	if got := ovTaskTitles(t, f, member, "?preset=completed_week"); strings.Join(got, "|") != "Shipped report" {
		t.Fatalf("completed this week = %v", got)
	}
	if got := ovTaskTitles(t, f, member, "?assignee_id="+member.id); strings.Join(got, "|") != "Overdue report" {
		t.Fatalf("assignee = %v", got)
	}
	today := time.Now().UTC()
	from, to := today.AddDate(0, 0, -10).Format(time.DateOnly), today.AddDate(0, 0, 10).Format(time.DateOnly)
	if got := ovTaskTitles(t, f, member, "?due_from="+from+"&due_to="+to); len(got) != 3 {
		t.Fatalf("due range = %v", got)
	}
	if got := ovTaskTitles(t, f, member, "?q=REPORT"); strings.Join(got, "|") != "Overdue report|Shipped report" {
		t.Fatalf("title search = %v", got)
	}
	for _, bad := range []string{"?status=bogus", "?preset=nope", "?space_id=not-a-uuid", "?due_from=2026-13-40", "?limit=0"} {
		if res := f.call(member, "GET", "/overview/tasks"+bad, nil); res.status != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d, want 422", bad, res.status)
		}
	}
}
