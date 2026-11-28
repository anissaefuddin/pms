package httpserver_test

import (
	"net/http"
	"strings"
	"testing"
)

// B3-15: a person who can see only Client A gets nothing about Client B from any list, search, or overview endpoint. Each endpoint
// is read raw, so an array body is checked too. A positive control proves the endpoint works for Client A's own item.
func TestCrossClientReadsDoNotDiscloseHiddenItems(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	clientA := f.createClient(admin, "CR-A")["id"].(string)
	clientB := f.createClient(admin, "CR-B")["id"].(string)
	userA := f.person("regular")
	f.addMember(admin, clientA, admin)
	f.addMember(admin, clientB, admin)
	f.addMember(admin, clientA, userA)
	spaceA := f.scopedSpace("Alpha workspace", "client", clientA)
	spaceB := f.scopedSpace("Bravo workspace", "client", clientB)
	f.share(spaceA, "user", admin.id, "full")
	f.share(spaceA, "user", userA.id, "full")
	f.share(spaceB, "user", admin.id, "full")
	visible := f.itemIn(spaceA, "client", clientA, userA.id, "Alpha visible task")
	hidden := f.itemIn(spaceB, "client", clientB, admin.id, "Bravo secret task")
	// Overdue and active, so overview, agenda, and workload would count it if they leaked it.
	f.exec("UPDATE items SET type='task', due_date='2020-01-01' WHERE id=ANY($1::uuid[])", []string{visible, hidden})

	paths := []string{
		"/search?q=secret",
		"/search?q=Bravo",
		"/overview",
		"/overview/tasks",
		"/overview/agenda?from=2020-01-01&days=30",
		"/overview/timeline?from=2020-01-01&days=30",
		"/workload?from=2020-01-01&to=2020-01-31",
		"/me/todo",
		"/me/shared",
	}
	for _, path := range paths {
		status, _, body := f.rawGet(userA, path)
		if status != http.StatusOK {
			t.Fatalf("GET %s: status %d", path, status)
		}
		for _, leak := range []string{"Bravo secret task", "Bravo workspace", clientB, hidden} {
			if strings.Contains(body, leak) {
				t.Fatalf("GET %s disclosed %q to a Client A reader", path, leak)
			}
		}
	}
	// Positive control: the same overdue query finds Client A's own item, so the endpoints work and the leak check is meaningful.
	if status, _, body := f.rawGet(userA, "/search?q=Alpha"); status != http.StatusOK || !strings.Contains(body, visible) {
		t.Fatalf("the Client A reader must find their own item: status %d", status)
	}
}
