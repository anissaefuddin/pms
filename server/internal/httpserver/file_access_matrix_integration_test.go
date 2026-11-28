package httpserver_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

// AC-12 / Workstream B (FILE-01..06, FILE-08). Every request is checked against current membership and current grants, so a
// revoked member or a Workspace Admin without a grant gets not found, the same as a missing file. Positive controls come first.
func TestFileAccessFollowsCurrentCustomerAccess(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	client := f.createClient(admin, "FA-A")["id"].(string)
	otherClient := f.createClient(admin, "FA-B")["id"].(string)
	member, outsider, bareAdmin := f.person("regular"), f.person("regular"), f.person("admin")
	f.addMember(admin, client, admin)
	f.addMember(admin, client, member)
	f.addMember(admin, otherClient, outsider)
	space := f.scopedSpace("FA client", "client", client)
	f.share(space, "user", admin.id, "full")
	f.share(space, "user", member.id, "read")
	f.share(space, "user", outsider.id, "read") // a grant without membership of this Customer: must not work
	item := f.itemIn(space, "client", client, admin.id, "Spec with files")

	upload := func(p *person, name string, data []byte) string {
		t.Helper()
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, _ := form.CreateFormFile("file", name)
		part.Write(data)
		form.Close()
		req, _ := http.NewRequest(http.MethodPost, f.srv.URL+"/api/v1/items/"+item+"/attachments", &body)
		req.AddCookie(p.cookie)
		req.Header.Set("Origin", publicURL)
		req.Header.Set("Content-Type", form.FormDataContentType())
		res, err := f.srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("upload: status %d %v", res.StatusCode, out)
		}
		return out["id"].(string)
	}
	att := upload(admin, "spec.txt", []byte("confidential client text"))
	download := func(p *person) (int, string) {
		status, _, body := f.rawGet(p, "/attachments/"+att+"/download")
		return status, body
	}

	// FILE-01: same-client authorized access works (positive control).
	if status, body := download(member); status != http.StatusOK || !strings.Contains(body, "confidential client text") {
		t.Fatalf("FILE-01 same-client download: status %d", status)
	}
	// FILE-02: a person of another Customer, even with a grant on the Space, is denied.
	if status, body := download(outsider); status != http.StatusNotFound || strings.Contains(body, "confidential") {
		t.Fatalf("FILE-02 cross-client download: status %d", status)
	}
	// FILE-03: a Workspace Admin without membership or grant is denied.
	if status, body := download(bareAdmin); status != http.StatusNotFound || strings.Contains(body, "confidential") {
		t.Fatalf("FILE-03 admin without access: status %d", status)
	}
	// FILE-05: previews follow the same item check. Positive first, then denial for the outsider.
	if status, _, _ := f.rawGet(member, "/attachments/"+att+"/versions/1/preview"); status != http.StatusOK {
		t.Fatalf("FILE-05 preview for a member: status %d", status)
	}
	if status, _, _ := f.rawGet(outsider, "/attachments/"+att+"/versions/1/preview"); status != http.StatusNotFound {
		t.Fatalf("FILE-05 preview for an outsider: status %d", status)
	}
	// FILE-06: version history follows the item check too.
	if status, _, _ := f.rawGet(member, "/attachments/"+att+"/versions"); status != http.StatusOK {
		t.Fatalf("FILE-06 versions for a member: status %d", status)
	}
	if status, _, _ := f.rawGet(outsider, "/attachments/"+att+"/versions"); status != http.StatusNotFound {
		t.Fatalf("FILE-06 versions for an outsider: status %d", status)
	}
	// FILE-08: an export requested by a member is readable by that member, and stops being readable once access ends.
	export := f.call(member, http.MethodPost, "/spaces/"+space+"/excel-exports", map[string]any{})
	f.want(export, http.StatusAccepted, "FILE-08 export queued")
	exportPath := "/excel-exports/" + export.body["id"].(string)
	f.want(f.call(member, http.MethodGet, exportPath, nil), http.StatusOK, "FILE-08 requester reads the export before revocation")

	// FILE-04: revoked membership denies every later request, including the export and the preview.
	f.revokeMember(admin, client, member)
	if status, body := download(member); status != http.StatusNotFound || strings.Contains(body, "confidential") {
		t.Fatalf("FILE-04 revoked member download: status %d", status)
	}
	if status, _, _ := f.rawGet(member, "/attachments/"+att+"/versions/1/preview"); status != http.StatusNotFound {
		t.Fatalf("FILE-04 revoked member preview: status %d", status)
	}
	f.want(f.call(member, http.MethodGet, exportPath, nil), http.StatusNotFound, "FILE-08 export after revocation")
	if status, _, _ := f.rawGet(member, exportPath+"/file"); status != http.StatusNotFound {
		t.Fatalf("FILE-08 export file after revocation: status %d", status)
	}
}
