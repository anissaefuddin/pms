package httpserver_test

import (
	"fmt"
	"net/http"
	"testing"
)

// Workspace Overview (see docs/reviews/LENS_WORKSPACE_OVERVIEW_IMPLEMENTATION.md). Name prefix TestWorkspaceOverview.

// ovStatus returns the first status of a group in the default workflow.
func ovStatus(f *fixture, grp string) string {
	f.t.Helper()
	return f.queryID(fmt.Sprintf("SELECT id::text FROM statuses WHERE grp='%s'::status_group ORDER BY position LIMIT 1", grp))
}

// ovItem inserts a Task or Project with the given status group, due date expression, and completed-at expression (SQL).
func ovItem(f *fixture, creator, typ, title, grp, due, completed string, spaces ...string) string {
	f.t.Helper()
	dueExpr, doneExpr := "NULL", "NULL"
	if due != "" {
		dueExpr = due
	}
	if completed != "" {
		doneExpr = completed
	}
	id := f.queryID(fmt.Sprintf(
		"INSERT INTO items(type,title,created_by,status_id,due_date,completed_at) VALUES ('%s',$1,$2,$3,%s,%s) RETURNING id::text",
		typ, dueExpr, doneExpr), title, creator, ovStatus(f, grp))
	for _, s := range spaces {
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, s)
	}
	return id
}

func ovGet(f *fixture, p *person, query string) reply {
	f.t.Helper()
	return f.call(p, "GET", "/overview"+query, nil)
}

func ovKPI(f *fixture, res reply, key string) float64 {
	f.t.Helper()
	kpis, ok := res.body["kpis"].(map[string]any)
	if !ok {
		f.t.Fatalf("no kpis in %v", res.body)
	}
	v, ok := kpis[key].(float64)
	if !ok {
		f.t.Fatalf("kpi %s missing in %v", key, kpis)
	}
	return v
}

func ovSpaceIDs(res reply) []string {
	var out []string
	rows, _ := res.body["spaces"].([]any)
	for _, r := range rows {
		out = append(out, r.(map[string]any)["id"].(string))
	}
	return out
}

func TestWorkspaceOverviewAccessCountsAndTimezone(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	member := f.person("regular")
	viewer := f.person("viewer")
	multi := f.person("regular")
	open := f.spaceWithoutAdmins("Open")
	closed := f.spaceWithoutAdmins("Closed")
	f.share(open, "user", member.id, "read")
	f.share(open, "user", viewer.id, "read")
	f.share(open, "user", multi.id, "full")
	f.share(closed, "user", multi.id, "full")
	f.exec("UPDATE users SET timezone='Asia/Jakarta' WHERE id IN ($1,$2,$3)", member.id, viewer.id, multi.id)

	// Open: an active project, an overdue task (yesterday in Jakarta), a task due today, a task due tomorrow,
	// a task completed now, and a task completed long ago.
	ovItem(f, admin.id, "project", "P1", "active", "", "", open)
	ovItem(f, admin.id, "task", "T-overdue", "active", "(now() AT TIME ZONE 'Asia/Jakarta')::date - 1", "", open)
	ovItem(f, admin.id, "task", "T-today", "active", "(now() AT TIME ZONE 'Asia/Jakarta')::date", "", open)
	ovItem(f, admin.id, "task", "T-tomorrow", "active", "(now() AT TIME ZONE 'Asia/Jakarta')::date + 1", "", open)
	ovItem(f, admin.id, "task", "T-done-now", "completed", "", "now()", open)
	ovItem(f, admin.id, "task", "T-done-old", "completed", "", "now() - interval '40 days'", open)
	// Closed: an active project and an overdue task that only the multi user can see.
	ovItem(f, admin.id, "project", "P2", "active", "", "", closed)
	ovItem(f, admin.id, "task", "T-closed-overdue", "active", "(now() AT TIME ZONE 'Asia/Jakarta')::date - 3", "", closed)
	// A project in both Spaces must be counted once for the multi user.
	ovItem(f, admin.id, "project", "P3", "active", "", "", open, closed)

	// Admin with no grant sees no Space and no counts.
	adminRes := ovGet(f, admin, "")
	f.want(adminRes, http.StatusOK, "admin overview")
	if ovKPI(f, adminRes, "accessible_spaces") != 0 || ovKPI(f, adminRes, "active_projects") != 0 {
		t.Fatalf("an admin without a grant must see nothing: %v", adminRes.body)
	}

	// Member with a read share on Open: counts that Space only, in their own time zone.
	memberRes := ovGet(f, member, "")
	f.want(memberRes, http.StatusOK, "member overview")
	if ovKPI(f, memberRes, "accessible_spaces") != 1 {
		t.Fatalf("member sees one Space: %v", memberRes.body)
	}
	if got := ovKPI(f, memberRes, "active_projects"); got != 2 {
		t.Fatalf("member active projects = %v, want 2 (P1 and P3)", got)
	}
	if got := ovKPI(f, memberRes, "overdue_tasks"); got != 1 {
		t.Fatalf("member overdue = %v, want 1 (yesterday in Jakarta only)", got)
	}
	if got := ovKPI(f, memberRes, "completed_this_week"); got != 1 {
		t.Fatalf("member completed this week = %v, want 1", got)
	}
	if ovKPI(f, memberRes, "accessible_spaces") != 1 || len(ovSpaceIDs(memberRes)) != 1 {
		t.Fatalf("member must not see the closed Space")
	}
	if at := memberRes.body["kpis"].(map[string]any)["at_risk_projects"].(map[string]any)["status"]; at != "NOT_AVAILABLE" {
		t.Fatalf("at-risk must be NOT_AVAILABLE, got %v", at)
	}

	// Viewer (read on Open) gets the same numbers as the member: read is the ceiling for what they see.
	viewerRes := ovGet(f, viewer, "")
	f.want(viewerRes, http.StatusOK, "viewer overview")
	if ovKPI(f, viewerRes, "active_projects") != ovKPI(f, memberRes, "active_projects") {
		t.Fatalf("viewer and member see the same Open Space numbers")
	}

	// Multi user sees both Spaces; the project in both is counted once.
	multiRes := ovGet(f, multi, "")
	f.want(multiRes, http.StatusOK, "multi overview")
	if got := ovKPI(f, multiRes, "active_projects"); got != 3 {
		t.Fatalf("multi active projects = %v, want 3 (P1, P2, P3 once)", got)
	}
	if got := ovKPI(f, multiRes, "overdue_tasks"); got != 2 {
		t.Fatalf("multi overdue = %v, want 2", got)
	}
	if got := len(ovSpaceIDs(multiRes)); got != 2 {
		t.Fatalf("multi lists two Spaces, got %d", got)
	}
}

func TestWorkspaceOverviewRevokeArchiveFilterAndPagination(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	member := f.person("regular")
	one := f.spaceWithoutAdmins("Alpha")
	two := f.spaceWithoutAdmins("Beta")
	f.share(one, "user", member.id, "read")
	f.share(two, "user", member.id, "read")
	ovItem(f, admin.id, "project", "P", "active", "", "", one)
	ovItem(f, admin.id, "project", "Q", "active", "", "", two)

	// Pagination: two Spaces, page size one.
	first := ovGet(f, member, "?limit=1&sort=name")
	f.want(first, http.StatusOK, "first page")
	if first.body["next_cursor"] == nil || first.body["total"] != 2.0 {
		t.Fatalf("first page must carry a cursor and total 2: %v", first.body)
	}
	second := ovGet(f, member, "?limit=1&sort=name&cursor="+first.body["next_cursor"].(string))
	f.want(second, http.StatusOK, "second page")
	if s1, s2 := ovSpaceIDs(first), ovSpaceIDs(second); len(s1) != 1 || len(s2) != 1 || s1[0] == s2[0] {
		t.Fatalf("pages must not repeat a Space: %v %v", s1, s2)
	}
	if second.body["next_cursor"] != nil {
		t.Fatalf("the last page has no cursor")
	}

	// Archived Spaces are hidden by default and shown only when asked for, still subject to access.
	f.exec("UPDATE spaces SET archived_at=now() WHERE id=$1", two)
	if n := len(ovSpaceIDs(ovGet(f, member, ""))); n != 1 {
		t.Fatalf("an archived Space is hidden by default, got %d", n)
	}
	if n := len(ovSpaceIDs(ovGet(f, member, "?include_archived=true"))); n != 2 {
		t.Fatalf("include_archived shows it, got %d", n)
	}
	f.exec("UPDATE spaces SET archived_at=NULL WHERE id=$1", two)

	// Invisible Space filter: 404, not an empty list that reveals the Space exists.
	stranger := f.person("regular")
	f.want(ovGet(f, stranger, "?space_id="+one), http.StatusNotFound, "invisible Space filter")

	// Bad parameters are refused.
	f.want(ovGet(f, member, "?sort=secret"), http.StatusUnprocessableEntity, "unknown sort")
	f.want(ovGet(f, member, "?limit=500"), http.StatusUnprocessableEntity, "limit above maximum")
	f.want(ovGet(f, member, "?cursor=%21%21%21"), http.StatusUnprocessableEntity, "bad cursor")

	// Revoke: the next request no longer lists the Space, with no cache in between.
	f.exec("DELETE FROM shares WHERE target_type='space' AND target_id=$1 AND principal_id=$2", one, member.id)
	after := ovGet(f, member, "")
	f.want(after, http.StatusOK, "after revoke")
	if n := len(ovSpaceIDs(after)); n != 1 {
		t.Fatalf("a revoked Space leaves the overview at once, got %d", n)
	}
	for _, id := range ovSpaceIDs(after) {
		if id == one {
			t.Fatalf("the revoked Space must not appear")
		}
	}
	f.want(ovGet(f, member, "?space_id="+one), http.StatusNotFound, "revoked Space filter")
	if got := ovKPI(f, after, "active_projects"); got != 1 {
		t.Fatalf("revoked Space's project is no longer counted, got %v", got)
	}
}
