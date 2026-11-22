package httpserver_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"
)

// Workspace Overview timeline and agenda (Batch C). Name prefix TestWorkspaceOverview.

func ovGetPath(f *fixture, p *person, path string) reply {
	f.t.Helper()
	return f.call(p, "GET", path, nil)
}

func ovChildOf(f *fixture, child, parent, space string) {
	f.t.Helper()
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'U')", child, space, parent)
}

func TestWorkspaceOverviewTimelineGroupsDueWork(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	member := f.person("regular")
	open := f.spaceWithoutAdmins("Open")
	f.share(open, "user", member.id, "read")
	due := func(n int) string { return "(now() AT TIME ZONE 'UTC')::date + " + strconv.Itoa(n) }

	project := ovItem(f, admin.id, "project", "P", "active", due(5), "", open)
	inside := ovItem(f, admin.id, "task", "T-inside", "active", due(6), "")
	ovChildOf(f, inside, project, open)
	ovItem(f, admin.id, "task", "T-unassigned", "active", due(7), "", open)
	ovItem(f, admin.id, "task", "T-far", "active", due(200), "", open)
	ovItem(f, admin.id, "task", "T-done", "completed", due(8), "", open)

	res := ovGetPath(f, member, "/overview/timeline")
	f.want(res, http.StatusOK, "timeline")
	spaces := res.body["spaces"].([]any)
	if len(spaces) != 1 {
		t.Fatalf("one Space with due work, got %d: %v", len(spaces), res.body)
	}
	s := spaces[0].(map[string]any)
	projects := s["projects"].([]any)
	if len(projects) != 1 || len(projects[0].(map[string]any)["tasks"].([]any)) != 1 {
		t.Fatalf("project P holds exactly its child task in range: %v", projects)
	}
	if u := s["unassigned_tasks"].([]any); len(u) != 1 || u[0].(map[string]any)["title"] != "T-unassigned" {
		t.Fatalf("only the unparented task is unassigned: %v", u)
	}

	// Out-of-range input is refused, not clamped.
	f.want(ovGetPath(f, member, "/overview/timeline?days=91"), http.StatusUnprocessableEntity, "range above 90")
	f.want(ovGetPath(f, member, "/overview/timeline?from=2026-13-01"), http.StatusUnprocessableEntity, "bad date")
	f.want(ovGetPath(f, member, "/overview/timeline?space_id="+open+"&days=0"), http.StatusUnprocessableEntity, "zero days")

	// A stranger with no Space sees nothing.
	stranger := f.person("regular")
	if n := len(ovGetPath(f, stranger, "/overview/timeline").body["spaces"].([]any)); n != 0 {
		t.Fatalf("no grant means no Space in the timeline, got %d", n)
	}
}

func TestWorkspaceOverviewAgendaOrderOverdueAndKind(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	member := f.person("regular")
	open := f.spaceWithoutAdmins("Open")
	f.share(open, "user", member.id, "read")
	due := func(n int) string { return "(now() AT TIME ZONE 'UTC')::date + " + strconv.Itoa(n) }

	ovItem(f, admin.id, "project", "P3", "active", due(3), "", open)
	ovItem(f, admin.id, "task", "T-today", "active", due(0), "", open)
	ovItem(f, admin.id, "task", "T-done", "completed", due(1), "", open)
	ovItem(f, admin.id, "task", "T-late", "active", due(-1), "", open)

	res := ovGetPath(f, member, "/overview/agenda")
	f.want(res, http.StatusOK, "agenda")
	items := res.body["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("today and +3 only (completed excluded, yesterday outside default range): %v", items)
	}
	if first := items[0].(map[string]any); first["title"] != "T-today" || first["overdue"] != false {
		t.Fatalf("ordered by date, today first: %v", first)
	}
	if n := res.body["overdue_count"].(float64); n != 1 {
		t.Fatalf("one past-due active task regardless of range, got %v", n)
	}

	// A range that starts yesterday shows the overdue task, flagged.
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	wide := ovGetPath(f, member, "/overview/agenda?from="+yesterday)
	f.want(wide, http.StatusOK, "agenda from yesterday")
	if n := len(wide.body["items"].([]any)); n != 3 {
		t.Fatalf("yesterday, today and +3: got %d", n)
	}
	if first := wide.body["items"].([]any)[0].(map[string]any); first["title"] != "T-late" || first["overdue"] != true {
		t.Fatalf("the past-due task comes first and is flagged: %v", first)
	}

	if n := len(ovGetPath(f, member, "/overview/agenda?type=task").body["items"].([]any)); n != 1 {
		t.Fatalf("type=task keeps only tasks today, got %d", n)
	}
	f.want(ovGetPath(f, member, "/overview/agenda?type=secret"), http.StatusUnprocessableEntity, "unknown type")
	f.want(ovGetPath(f, member, "/overview/agenda?days=32"), http.StatusUnprocessableEntity, "days above 31")
	f.want(ovGetPath(f, member, "/overview/agenda?limit=0"), http.StatusUnprocessableEntity, "limit zero")
}
