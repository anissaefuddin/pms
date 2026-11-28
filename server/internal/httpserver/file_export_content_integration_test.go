package httpserver_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/projectexcel"
)

// PROMPT #026 Workstream D (FILE-07): the workbook a person downloads contains the Customer A records they may read, and no Customer B
// title or identifier, even though this person can also read Customer B. The check reads the generated cells, not only the HTTP status.
func TestSpaceExportContainsOnlyTheRequestedCustomersRecords(t *testing.T) {
	f := newFixture(t)
	startExcelWorker(t, f)
	admin := f.person("admin")
	clientA := f.createClient(admin, "EX-A")["id"].(string)
	clientB := f.createClient(admin, "EX-B")["id"].(string)
	dual, outsider := f.person("regular"), f.person("regular")
	f.addMember(admin, clientA, dual)
	f.addMember(admin, clientB, dual)
	f.addMember(admin, clientA, admin)
	f.addMember(admin, clientB, admin)
	spaceA := f.scopedSpace("EX workspace A", "client", clientA)
	spaceB := f.scopedSpace("EX workspace B", "client", clientB)
	for _, s := range []string{spaceA, spaceB} {
		f.share(s, "user", admin.id, "full")
		f.share(s, "user", dual.id, "full")
	}
	f.share(spaceA, "user", outsider.id, "read") // a grant without membership of Customer A: no export
	alpha := f.itemIn(spaceA, "client", clientA, admin.id, "Alpha export row")
	bravo := f.itemIn(spaceB, "client", clientB, admin.id, "Bravo export row")
	_ = alpha

	// Positive control: the authorized export is created, completes, and is retrieved by its requester.
	created := f.call(dual, http.MethodPost, "/spaces/"+spaceA+"/excel-exports", map[string]any{})
	f.want(created, http.StatusAccepted, "dual member exports Customer A's Space")
	path := "/excel-exports/" + created.body["id"].(string)
	deadline := time.Now().Add(10 * time.Second)
	for f.call(dual, http.MethodGet, path, nil).body["status"] != "ready" {
		if time.Now().After(deadline) {
			t.Fatal("export not ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	req, _ := http.NewRequest(http.MethodGet, f.srv.URL+"/api/v1"+path+"/file", nil)
	req.AddCookie(dual.cookie)
	res, err := f.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	rows, err := projectexcel.Read(data)
	if err != nil {
		t.Fatalf("the export must be a readable workbook: %v", err)
	}
	cells := strings.ToLower(strings.Join(flatten(rows), "\n"))
	if !strings.Contains(cells, "alpha export row") {
		t.Fatalf("FILE-07: the Customer A record must be in the export")
	}
	if strings.Contains(cells, "bravo export row") || strings.Contains(cells, strings.ToLower(bravo)) || strings.Contains(cells, strings.ToLower(clientB)) {
		t.Fatalf("FILE-07: the export discloses a Customer B title or identifier")
	}

	// Without membership of Customer A, a grant alone gives no export of its Space.
	denied := f.call(outsider, http.MethodPost, "/spaces/"+spaceA+"/excel-exports", map[string]any{})
	if denied.status < 400 {
		t.Fatalf("FILE-07: a grant without membership must not produce an export (status %d)", denied.status)
	}
}

func flatten(rows [][]string) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r...)
	}
	return out
}
