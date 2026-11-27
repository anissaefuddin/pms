package httpserver_test

import (
	"net/http"
	"testing"
	"time"
)

// PROMPT 034: the consolidated monitor lists only projects the caller may view, with summary and options derived from that set.
// A Customer the caller is not authorised for is absent from rows, filters, counts and overdue figures alike. Revocation applies at once.
func TestPortfolioMonitorShowsOnlyAuthorisedCustomers(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	custA := f.createClient(admin, "MON-A")["id"].(string)
	custB := f.createClient(admin, "MON-B")["id"].(string)
	memberA, memberB, manager, outsider := f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular")
	f.addMember(admin, custA, memberA)
	f.addMember(admin, custB, memberB)
	f.addMember(admin, custA, manager)
	f.addMember(admin, custB, manager)

	spaceA := f.call(memberA, "POST", "/spaces", map[string]any{"name": "Mon A", "scope_state": "client", "client_id": custA})
	f.want(spaceA, http.StatusCreated, "Customer A space")
	spaceB := f.call(memberB, "POST", "/spaces", map[string]any{"name": "Mon B", "scope_state": "client", "client_id": custB})
	f.want(spaceB, http.StatusCreated, "Customer B space")
	sidA, sidB := spaceA.body["id"].(string), spaceB.body["id"].(string)
	f.share(sidA, "user", manager.id, "read")
	f.share(sidB, "user", manager.id, "read")

	projA := f.call(memberA, "POST", "/items", map[string]any{"type": "project", "title": "Mon project A", "owner_id": memberA.id,
		"locations": []map[string]any{{"space_id": sidA}}})
	f.want(projA, http.StatusCreated, "project A")
	projB := f.call(memberB, "POST", "/items", map[string]any{"type": "project", "title": "Mon project B", "locations": []map[string]any{{"space_id": sidB}}})
	f.want(projB, http.StatusCreated, "project B without a Project Manager")
	idA, idB := projA.body["id"].(string), projB.body["id"].(string)

	// An overdue active task in project A, due well before today.
	overdue := time.Now().AddDate(0, 0, -10).Format("2006-01-02")
	task := f.call(memberA, "POST", "/items", map[string]any{"type": "task", "title": "Late work", "due_date": overdue, "start_date": overdue,
		"locations": []map[string]any{{"space_id": sidA, "parent_item_id": idA}}})
	f.want(task, http.StatusCreated, "overdue task")

	monitor := func(p *person, query string) map[string]any {
		t.Helper()
		r := f.call(p, "GET", "/portfolio/projects"+query, nil)
		f.want(r, http.StatusOK, "monitor "+query)
		return r.body
	}
	titles := func(body map[string]any) map[string]bool {
		out := map[string]bool{}
		for _, raw := range body["items"].([]any) {
			out[raw.(map[string]any)["title"].(string)] = true
		}
		return out
	}

	// Customer A member: only A's project, its overdue task counted, and no B data anywhere.
	a := monitor(memberA, "")
	if got := titles(a); !got["Mon project A"] || got["Mon project B"] {
		t.Fatalf("Customer A member must see only project A: %v", got)
	}
	if n := a["summary"].(map[string]any)["total"].(float64); n != 1 {
		t.Fatalf("summary total for Customer A member = %v, want 1", n)
	}
	if od := a["summary"].(map[string]any)["overdue_tasks"].(float64); od != 1 {
		t.Fatalf("overdue tasks for Customer A member = %v, want 1", od)
	}
	for _, c := range a["options"].(map[string]any)["customers"].([]any) {
		if c.(map[string]any)["code"] == "MON-B" {
			t.Fatal("a Customer the member does not belong to appears in the filter options")
		}
	}

	// Manager with membership and read shares in both: both projects, both Customers offered.
	m := monitor(manager, "")
	if got := titles(m); !got["Mon project A"] || !got["Mon project B"] {
		t.Fatalf("the manager must see both authorised projects: %v", got)
	}
	if n := len(m["options"].(map[string]any)["customers"].([]any)); n != 2 {
		t.Fatalf("the manager must be offered both Customers, got %d", n)
	}

	// Customer filter and Unassigned are applied to the authorised set.
	byA := monitor(manager, "?customer="+custA)
	if got := titles(byA); !got["Mon project A"] || got["Mon project B"] {
		t.Fatalf("customer filter must narrow to Customer A: %v", got)
	}
	unassigned := monitor(manager, "?owner=none")
	if got := titles(unassigned); !got["Mon project B"] || got["Mon project A"] {
		t.Fatalf("owner=none must list the project without a Project Manager only: %v", got)
	}
	if n := unassigned["summary"].(map[string]any)["unassigned"].(float64); n != 1 {
		t.Fatalf("summary must count the unassigned project even when filtered: %v", n)
	}

	// Missing progress and health are null, not invented values.
	for _, row := range m["items"].([]any) {
		r := row.(map[string]any)
		if r["title"] == "Mon project B" && (r["progress_percent"] != nil || r["health"] != nil) {
			t.Fatalf("a project without progress or health must show null: %v", r)
		}
	}

	// A user with no membership sees nothing.
	if n := len(monitor(outsider, "")["items"].([]any)); n != 0 {
		t.Fatalf("an outsider must see no projects, got %d", n)
	}

	// Revocation takes effect at once: the manager loses Customer B and its project.
	f.revokeMember(admin, custB, manager)
	after := monitor(manager, "")
	if got := titles(after); got["Mon project B"] || !got["Mon project A"] {
		t.Fatalf("after revoking Customer B the manager must lose project B: %v", got)
	}
	if got := titles(monitor(memberB, "")); !got["Mon project B"] {
		t.Fatal("the Customer B member must still see project B")
	}
	f.want(f.call(manager, "GET", "/items/"+idB, nil), http.StatusNotFound, "revoked membership denies the project directly")
}

// Sort and filter parameters are validated, and a bad value is refused rather than ignored.
func TestPortfolioMonitorRefusesUnknownFilters(t *testing.T) {
	f := newFixture(t)
	user := f.person("regular")
	f.want(f.call(user, "GET", "/portfolio/projects?health=purple", nil), http.StatusUnprocessableEntity, "unknown health")
	f.want(f.call(user, "GET", "/portfolio/projects?status=lost", nil), http.StatusUnprocessableEntity, "unknown status")
	f.want(f.call(user, "GET", "/portfolio/projects?sort=secret", nil), http.StatusUnprocessableEntity, "unknown sort")
	empty := f.call(user, "GET", "/portfolio/projects", nil)
	f.want(empty, http.StatusOK, "the monitor for a user with no projects")
	// Empty lists are arrays, never null: the page maps over them.
	options := empty.body["options"].(map[string]any)
	if _, ok := options["customers"].([]any); !ok {
		t.Fatalf("options.customers must be an empty array, got %v", options["customers"])
	}
	if _, ok := options["owners"].([]any); !ok {
		t.Fatalf("options.owners must be an empty array, got %v", options["owners"])
	}
}

// Fail closed: a project in an unresolved legacy Space is not in anyone's monitor, the administrator's included.
func TestPortfolioMonitorExcludesUnresolvedOwnership(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	legacy := f.unresolvedSpace("Legacy monitor space")
	const item = "01a2aaaa-0000-7000-8000-00000000f00d"
	f.exec("INSERT INTO items(id,type,title,scope_state,created_by,version) VALUES ($1,'project','Legacy project','unresolved',$2,1)", item, admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, legacy)
	r := f.call(admin, "GET", "/portfolio/projects", nil)
	f.want(r, http.StatusOK, "monitor")
	for _, raw := range r.body["items"].([]any) {
		if raw.(map[string]any)["title"] == "Legacy project" {
			t.Fatal("an unresolved legacy project reached the monitor")
		}
	}
}
