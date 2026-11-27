package httpserver_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// PROMPT #028 (carried B3 verification, Portfolio): a Portfolio timeline returns only projects the caller may view. A request that also
// names another Customer's project is refused whole, so it reveals nothing about that project. Membership alone and a Workspace Admin
// without grants receive no protected project content. A revoked member receives nothing.
func TestPortfolioReturnsOnlyAuthorizedProjects(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	clientA := f.createClient(admin, "PF-A")["id"].(string)
	clientB := f.createClient(admin, "PF-B")["id"].(string)
	member, outsiderB, bareAdmin := f.person("regular"), f.person("regular"), f.person("admin")
	f.addMember(admin, clientA, admin)
	f.addMember(admin, clientB, admin)
	f.addMember(admin, clientA, member)
	f.addMember(admin, clientB, outsiderB)
	spaceA := f.scopedSpace("PF workspace A", "client", clientA)
	spaceB := f.scopedSpace("PF workspace B", "client", clientB)
	f.share(spaceA, "user", admin.id, "full")
	f.share(spaceA, "user", member.id, "read")
	f.share(spaceB, "user", admin.id, "full")
	f.share(spaceB, "user", outsiderB.id, "read")
	projA := f.itemIn(spaceA, "client", clientA, admin.id, "Alpha portfolio project")
	projB := f.itemIn(spaceB, "client", clientB, admin.id, "Bravo portfolio project")
	f.exec("UPDATE items SET type='project', start_date='2026-01-05', due_date='2026-03-01' WHERE id = ANY($1::uuid[])", []string{projA, projB})

	// timeline returns the body of a 200 response, or "" when the request is refused.
	timeline := func(p *person, ids ...string) (int, string) {
		t.Helper()
		res := f.call(p, http.MethodPost, "/portfolio/timeline", map[string]any{"project_ids": ids})
		raw, _ := json.Marshal(res.body)
		return res.status, string(raw)
	}

	// Positive control: the member sees their own Customer's project.
	if status, body := timeline(member, projA); status != http.StatusOK || !strings.Contains(body, "Alpha portfolio project") {
		t.Fatalf("the member must see their own project (status %d): %s", status, body)
	}
	// A request that also names another Customer's project is refused whole: no partial list and no hint of the other project.
	if status, body := timeline(member, projA, projB); status == http.StatusOK || strings.Contains(body, "Bravo") || strings.Contains(body, projB) {
		t.Fatalf("a mixed request must be refused whole (status %d): %s", status, body)
	}
	// Another Customer's member cannot use a requested id of Customer A.
	if status, body := timeline(outsiderB, projA); status == http.StatusOK || strings.Contains(body, "Alpha") || strings.Contains(body, projA) {
		t.Fatalf("another Customer's project leaked through a requested id (status %d): %s", status, body)
	}
	// A Workspace Admin without grants receives no project content for either Customer.
	if status, body := timeline(bareAdmin, projA, projB); status == http.StatusOK || strings.Contains(body, "portfolio project") {
		t.Fatalf("a Workspace Admin without grants received project content (status %d): %s", status, body)
	}
	// Revocation: the member's access ends and the same request stops returning the project.
	f.revokeMember(admin, clientA, member)
	if status, body := timeline(member, projA); status == http.StatusOK || strings.Contains(body, "Alpha portfolio project") {
		t.Fatalf("a revoked member still receives the project in Portfolio (status %d): %s", status, body)
	}
}
