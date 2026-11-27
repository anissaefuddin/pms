package httpserver_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// PROMPT #028 (carried B3 verification, reports): a report result contains only the values of Spaces the reader may view. A report
// that names another Customer's Space gives a Customer A reader that Space's values neither in rows nor in totals.
func TestReportResultExcludesAnotherCustomersValues(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	clientA := f.createClient(admin, "RR-A")["id"].(string)
	clientB := f.createClient(admin, "RR-B")["id"].(string)
	member := f.person("regular")
	f.addMember(admin, clientA, admin)
	f.addMember(admin, clientB, admin)
	f.addMember(admin, clientA, member)
	spaceA := f.scopedSpace("RR workspace A", "client", clientA)
	spaceB := f.scopedSpace("RR workspace B", "client", clientB)
	f.share(spaceA, "user", admin.id, "full")
	f.share(spaceA, "user", member.id, "read")
	f.share(spaceB, "user", admin.id, "full")
	itemA := f.itemIn(spaceA, "client", clientA, admin.id, "Alpha reported task")
	itemB := f.itemIn(spaceB, "client", clientB, admin.id, "Bravo reported task")
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,current_date,45)", itemA, admin.id)
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,current_date,999)", itemB, admin.id)

	report := f.call(admin, http.MethodPost, "/reports", map[string]any{"name": "Two customers", "space_id": spaceA, "sharing": "space", "definition": map[string]any{
		"layout": "table", "query": map[string]any{"reporting_on": "time_entries", "metric": "minutes", "group_by": "item", "space_ids": []string{spaceA, spaceB}}}})
	f.want(report, http.StatusCreated, "report over two Customers")
	id := report.body["id"].(string)
	f.want(f.call(admin, http.MethodGet, "/reports/"+id+"/result", nil), http.StatusOK, "admin reads both Customers' values")

	status, _, body := f.rawGet(member, "/reports/"+id+"/result")
	if status != http.StatusOK {
		t.Fatalf("the member reads their Customer's report result: status %d", status)
	}
	if strings.Contains(body, "Bravo reported task") || strings.Contains(body, itemB) || strings.Contains(body, "999") {
		t.Fatalf("the report result discloses another Customer's value: %s", body)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "Alpha reported task") {
		t.Fatalf("the member must see their own Customer's row: %s", body)
	}
}
