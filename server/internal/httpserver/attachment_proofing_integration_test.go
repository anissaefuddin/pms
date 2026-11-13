package httpserver_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/previews"
	"github.com/anissaefuddin/pms/server/internal/storage"
	"github.com/google/uuid"
)

func pngBytes(w, h int) []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)))
	return b.Bytes()
}

// S11.1.3: previews are per version and annotations stay on the version they were pinned to.
func TestProofingAnnotationsStayOnTheirVersion(t *testing.T) {
	f := newFixture(t)
	owner, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Proofing")
	f.share(space, "user", reader.id, "read")
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Logo',$1) RETURNING id::text", owner.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	upload := func(path, name string, data []byte) map[string]any {
		t.Helper()
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, _ := form.CreateFormFile("file", name)
		part.Write(data)
		form.Close()
		req, _ := http.NewRequest("POST", f.srv.URL+"/api/v1"+path, &body)
		req.AddCookie(owner.cookie)
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
			t.Fatalf("upload %s: %d %v", name, res.StatusCode, out)
		}
		return out
	}
	id := upload("/items/"+item+"/attachments", "logo.png", pngBytes(40, 20))["id"].(string)
	upload("/attachments/"+id+"/versions", "logo2.png", pngBytes(2000, 500)) // wider than MaxEdge: must be scaled
	store := storage.LocalStorage{Dir: f.files}
	aid := uuid.MustParse(id)
	for v := 1; v <= 2; v++ { // the queue is not running in tests; run the job body directly
		if err := previews.Generate(f.ctx, f.pool, store, aid, v); err != nil {
			t.Fatal(err)
		}
	}
	v := "/attachments/" + id + "/versions/"
	p1, p2 := f.call(reader, "GET", v+"1/preview", nil), f.call(reader, "GET", v+"2/preview", nil)
	pages := func(r any) []any { return r.(map[string]any)["pages"].([]any) }
	if p1.status != 200 || p1.body["status"] != "ready" || pages(p1.body)[0].(map[string]any)["width"] != float64(40) {
		t.Fatalf("v1 preview: %+v", p1)
	}
	if w := pages(p2.body)[0].(map[string]any)["width"]; w != float64(previews.MaxEdge) {
		t.Fatalf("v2 preview not bounded: %v", w)
	}
	if s := f.call(outsider, "GET", v+"1/preview", nil).status; s != 404 {
		t.Fatalf("outsider preview: %d", s)
	}
	req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1"+v+"1/preview/1", nil)
	req.AddCookie(reader.cookie)
	res, _ := f.srv.Client().Do(req)
	img, err := png.Decode(res.Body)
	res.Body.Close()
	if err != nil || res.Header.Get("Content-Type") != "image/png" || img.Bounds().Dx() != 40 {
		t.Fatalf("preview page: %v %v", err, res.Header)
	}
	if f.call(reader, "GET", v+"1/preview/2", nil).status != 404 {
		t.Fatal("page 2 does not exist")
	}

	// Annotations: pinned to v1 only.
	pin := map[string]any{"page": 1, "x": 0.25, "y": 0.5, "body": "Logo is too close to the edge"}
	if s := f.call(reader, "POST", v+"1/annotations", pin).status; s != 403 {
		t.Fatalf("read-only user annotated: %d", s)
	}
	if s := f.call(outsider, "POST", v+"1/annotations", pin).status; s != 404 {
		t.Fatalf("outsider annotated: %d", s)
	}
	for _, bad := range []map[string]any{{"x": 1.5, "y": 0, "body": "x"}, {"x": 0, "y": 0, "body": "  "}, {"x": 0, "y": 0, "page": 0, "body": "x"}, {"y": 0, "body": "x"}} {
		if s := f.call(owner, "POST", v+"1/annotations", bad).status; s != 422 {
			t.Fatalf("invalid annotation %v: %d", bad, s)
		}
	}
	made := f.call(owner, "POST", v+"1/annotations", pin)
	if made.status != 201 || made.body["version"] != float64(1) || made.body["x"] != 0.25 {
		t.Fatalf("create: %+v", made)
	}
	if s := f.call(owner, "POST", v+"3/annotations", pin).status; s != 404 {
		t.Fatalf("annotation on a missing version: %d", s)
	}
	if got := f.call(reader, "GET", v+"1/annotations", nil).items(); len(got) != 1 || got[0]["body"] != pin["body"] {
		t.Fatalf("v1 annotations: %v", got)
	}
	if got := f.call(reader, "GET", v+"2/annotations", nil).items(); len(got) != 0 {
		t.Fatalf("annotation leaked to v2: %v", got)
	}
	upload("/attachments/"+id+"/versions", "logo3.png", pngBytes(10, 10)) // a newer version changes nothing for v1
	if got := f.call(reader, "GET", v+"1/annotations", nil).items(); len(got) != 1 {
		t.Fatalf("v1 annotations after v3: %v", got)
	}
	one := v + "1/annotations/" + made.body["id"].(string)
	if s := f.call(reader, "PATCH", one, map[string]any{"resolved": true}).status; s != 403 {
		t.Fatalf("read-only resolve: %d", s)
	}
	if got := f.call(owner, "PATCH", one, map[string]any{"resolved": true}); got.status != 200 || got.body["resolved"] != true {
		t.Fatalf("resolve: %+v", got)
	}
	if s := f.call(owner, "DELETE", v+"2/annotations/"+made.body["id"].(string), nil).status; s != 404 {
		t.Fatalf("delete through the wrong version: %d", s)
	}
	if s := f.call(owner, "DELETE", one, nil).status; s != 204 {
		t.Fatalf("delete: %d", s)
	}
	// Non-image types are skipped, not failed.
	txt := upload("/items/"+item+"/attachments", "notes.txt", []byte("hello"))["id"].(string)
	if err := previews.Generate(f.ctx, f.pool, store, uuid.MustParse(txt), 1); err != nil {
		t.Fatal(err)
	}
	if got := f.call(owner, "GET", "/attachments/"+txt+"/versions/1/preview", nil); got.body["status"] != "skipped" {
		t.Fatalf("text preview: %+v", got)
	}
}
