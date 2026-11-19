package httpserver_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/previews"
	"github.com/anissaefuddin/pms/server/internal/storage"
	"github.com/google/uuid"
)

// QA-D-lane4-001/002: when pages cannot be rendered (no pdftoppm), the page URL answers 404, never 500, and the preview reports "skipped".
func TestPDFPreviewWithoutRendererIsSkippedAndPageIs404(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no pdftoppm on PATH
	f := newFixture(t)
	owner := f.person("admin")
	space := f.space("Docs")
	f.share(space, "user", owner.id, "full") // A1.2: the owner creates the item and its attachments as an explicit member
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Doc',$1) RETURNING id::text", owner.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("file", "sample.pdf")
	part.Write([]byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF\n"))
	form.Close()
	req, _ := http.NewRequest("POST", f.srv.URL+"/api/v1/items/"+item+"/attachments", &body)
	req.AddCookie(owner.cookie)
	req.Header.Set("Origin", publicURL)
	req.Header.Set("Content-Type", form.FormDataContentType())
	res, err := f.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatalf("upload: %d %v", res.StatusCode, out)
	}
	id := out["id"].(string)
	if err := previews.Generate(f.ctx, f.pool, storage.LocalStorage{Dir: f.files}, uuid.MustParse(id), 1); err != nil {
		t.Fatal(err)
	}
	v := "/attachments/" + id + "/versions/1/"
	if p := f.call(owner, "GET", v+"preview", nil); p.status != 200 || p.body["status"] != "skipped" {
		t.Fatalf("preview: %+v", p)
	}
	if p := f.call(owner, "GET", v+"preview/1", nil); p.status != 404 {
		t.Fatalf("page of a skipped preview: %d, want 404 (%v)", p.status, p.body)
	}
	// And before any preview row exists at all.
	f.exec("DELETE FROM attachment_previews")
	if p := f.call(owner, "GET", v+"preview/1", nil); p.status != 404 {
		t.Fatalf("page without preview row: %d, want 404 (%v)", p.status, p.body)
	}
}
