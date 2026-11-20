package httpserver_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
)

// E2E-PAR-09 (API level): approval is pinned to a file version and an external reviewer sees only that version.
func TestFileApprovalPinnedToVersionWithExternalReviewer(t *testing.T) {
	f := newFixture(t)
	owner, reviewer, regularOutsider := f.person("admin"), f.person("external"), f.person("regular")
	space := f.space("Review")
	f.share(space, "user", owner.id, "full") // A1.2: the owner creates and uploads files as an explicit member
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Design',$1) RETURNING id::text", owner.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	upload := func(p *person, path, name, data string) map[string]any {
		t.Helper()
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, _ := form.CreateFormFile("file", name)
		part.Write([]byte(data))
		form.Close()
		req, _ := http.NewRequest("POST", f.srv.URL+"/api/v1"+path, &body)
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
		if res.StatusCode != 201 {
			t.Fatalf("upload %s: %d %v", path, res.StatusCode, out)
		}
		return out
	}
	get := func(p *person, path string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1"+path, nil)
		req.AddCookie(p.cookie)
		res, err := f.srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	att := upload(owner, "/items/"+item+"/attachments", "logo.txt", "v1 bytes")["id"].(string)
	vpath := "/attachments/" + att + "/versions"
	upload(owner, vpath, "logo.txt", "v2 bytes")

	// Without a pin an external user has no access at all (404 everywhere).
	f.want(f.call(reviewer, "GET", "/items/"+item+"/approvals", nil), 404, "reviewer has no item access")
	if s, _ := get(reviewer, vpath+"/1/download"); s != 404 {
		t.Fatalf("unpinned download: %d", s)
	}
	pin := map[string]any{"description": "Review logo", "approver_ids": []string{reviewer.id}, "attachment_id": att, "attachment_version": 1}
	// A regular user cannot be pinned without item access (only external reviewers are exempt); bad pins are 422.
	f.want(f.call(owner, "POST", "/items/"+item+"/approvals", map[string]any{"approver_ids": []string{regularOutsider.id}, "attachment_id": att, "attachment_version": 1}), 422, "regular without access")
	f.want(f.call(owner, "POST", "/items/"+item+"/approvals", map[string]any{"approver_ids": []string{reviewer.id}, "attachment_id": att, "attachment_version": 9}), 422, "unknown version")
	f.want(f.call(owner, "POST", "/items/"+item+"/approvals", map[string]any{"approver_ids": []string{reviewer.id}, "attachment_id": att}), 422, "half pin")
	// A plain item approval still needs item access for the external user.
	f.want(f.call(owner, "POST", "/items/"+item+"/approvals", map[string]any{"approver_ids": []string{reviewer.id}}), 422, "item approval needs view")

	created := f.call(owner, "POST", "/items/"+item+"/approvals", pin)
	f.want(created, 201, "pin approval to v1")
	if created.body["attachment_version"] != float64(1) || created.body["attachment_id"] != att {
		t.Fatalf("pin not reported: %v", created.body)
	}
	id := created.body["id"].(string)

	if s, b := get(reviewer, vpath+"/1/download"); s != 200 || b != "v1 bytes" {
		t.Fatalf("reviewer v1: %d %q", s, b)
	}
	if s, _ := get(reviewer, vpath+"/2/download"); s != 404 {
		t.Fatalf("reviewer must not read v2: %d", s)
	}
	if s, _ := get(reviewer, "/attachments/"+att+"/download"); s != 404 {
		t.Fatalf("reviewer must not read the current file: %d", s)
	}
	f.want(f.call(reviewer, "GET", "/items/"+item, nil), 404, "reviewer cannot open the item")
	f.want(f.call(reviewer, "DELETE", "/attachments/"+att, nil), 404, "reviewer cannot delete")
	reviews := f.call(reviewer, "GET", "/file-reviews", nil)
	if reviews.status != 200 || len(reviews.items()) != 1 || reviews.items()[0]["version"] != float64(1) || reviews.items()[0]["filename"] != "logo.txt" {
		t.Fatalf("file reviews: %d %v", reviews.status, reviews.body)
	}
	f.want(f.call(regularOutsider, "PUT", "/approvals/"+id+"/decision", map[string]any{"decision": "approved"}), 404, "stranger cannot decide")
	f.want(f.call(reviewer, "PUT", "/approvals/"+id+"/decision", map[string]any{"decision": "approved", "comment": "ok"}), 200, "reviewer decides")

	// A new version does not move the approval: it stays on v1, approved; v2 has no approval.
	list := f.call(owner, "GET", "/items/"+item+"/approvals", nil).items()
	if len(list) != 1 || list[0]["attachment_version"] != float64(1) || list[0]["status"] != "approved" {
		t.Fatalf("approval after v2: %v", list)
	}
	upload(owner, vpath, "logo.txt", "v3 bytes")
	if n := f.queryID("SELECT attachment_version::text FROM approvals WHERE id=$1", id); n != "1" {
		t.Fatalf("pin moved to v%s", n)
	}
	// Removing the reviewer from the approval revokes the file access.
	f.want(f.call(owner, "DELETE", "/approvals/"+id, nil), 204, "withdraw")
	if s, _ := get(reviewer, vpath+"/1/download"); s != 404 {
		t.Fatalf("access must end with the approval: %d", s)
	}
}
