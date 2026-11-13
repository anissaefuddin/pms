package httpserver_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/config"
)

// S11.1.6: the same permission and scanning rules apply when files live in an S3-compatible store, and links are attachments too.
func TestAttachmentsOnExternalStorageAndLinks(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	s3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch data, ok := objects[r.URL.Path]; r.Method {
		case "PUT":
			objects[r.URL.Path], _ = io.ReadAll(r.Body)
		case "DELETE":
			delete(objects, r.URL.Path)
			w.WriteHeader(204)
		default:
			if !ok {
				w.WriteHeader(404)
				return
			}
			http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
		}
	}))
	defer s3.Close()
	f := newFixtureWith(t, 1000, func(c *config.Config) {
		c.Storage, c.S3URL, c.S3Region, c.S3AccessKey, c.S3SecretKey = "s3", s3.URL+"/pms", "us-east-1", "AK", "SK"
	})
	owner, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Remote")
	f.share(space, "user", reader.id, "read")
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Spec',$1) RETURNING id::text", owner.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	upload := func(p *person, name string, data []byte) (int, map[string]any) {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, _ := form.CreateFormFile("file", name)
		part.Write(data)
		form.Close()
		req, _ := http.NewRequest("POST", f.srv.URL+"/api/v1/items/"+item+"/attachments", &body)
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
		return res.StatusCode, out
	}
	download := func(p *person, path string) (int, string) {
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
	count := func() int { mu.Lock(); defer mu.Unlock(); return len(objects) }

	status, a := upload(owner, "note.txt", []byte("remote bytes"))
	if status != 201 || count() != 1 {
		t.Fatalf("upload: %d %v objects=%d", status, a, count())
	}
	id := a["id"].(string)
	if s, b := download(reader, "/attachments/"+id+"/download"); s != 200 || b != "remote bytes" {
		t.Fatalf("reader download: %d %q", s, b)
	}
	if s, _ := download(outsider, "/attachments/"+id+"/download"); s != 404 {
		t.Fatalf("outsider download: %d", s)
	}
	if s, _ := upload(reader, "x.txt", []byte("no")); s != 403 {
		t.Fatalf("read-only upload: %d", s)
	}
	// Scanning still runs and a refused file leaves no object behind: executable, and a ZIP that hides one.
	if s, _ := upload(owner, "run.sh", []byte("#!/bin/sh\n")); s != 422 {
		t.Fatalf("executable: %d", s)
	}
	var z bytes.Buffer
	zw := zip.NewWriter(&z)
	w, _ := zw.Create("tool.exe")
	w.Write([]byte("MZ......"))
	zw.Close()
	if s, _ := upload(owner, "pack.zip", z.Bytes()); s != 422 || count() != 1 {
		t.Fatalf("zip with executable: %d objects=%d", s, count())
	}
	if s, _ := upload(owner, "big.txt", bytes.Repeat([]byte("a"), 2<<20)); s != 413 || count() != 1 {
		t.Fatalf("too large: %d objects=%d", s, count())
	}

	// Links.
	link := func(p *person, body string) (int, map[string]any) {
		req, _ := http.NewRequest("POST", f.srv.URL+"/api/v1/items/"+item+"/attachment-links", bytes.NewBufferString(body))
		req.AddCookie(p.cookie)
		req.Header.Set("Origin", publicURL)
		req.Header.Set("Content-Type", "application/json")
		res, err := f.srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out
	}
	status, l := link(owner, `{"url":"https://example.com/doc","title":"Design doc"}`)
	if status != 201 || l["link_url"] != "https://example.com/doc" || l["filename"] != "Design doc" || l["version"] != float64(1) {
		t.Fatalf("link: %d %v", status, l)
	}
	for body, want := range map[string]int{`{"url":"javascript:alert(1)"}`: 422, `{"url":"https://u:p@example.com/"}`: 422, `{"url":""}`: 422, `{}`: 422} {
		if s, _ := link(owner, body); s != want {
			t.Fatalf("%s: %d", body, s)
		}
	}
	if s, _ := link(reader, `{"url":"https://example.com/x"}`); s != 403 {
		t.Fatalf("read-only link: %d", s)
	}
	if s, _ := link(outsider, `{"url":"https://example.com/x"}`); s != 404 {
		t.Fatalf("outsider link: %d", s)
	}
	lid := l["id"].(string)
	if s, _ := download(reader, "/attachments/"+lid+"/download"); s != 422 {
		t.Fatalf("link download: %d", s)
	}
	if got := f.call(reader, "GET", "/items/"+item+"/attachments", nil).items(); len(got) != 2 || got[1]["link_url"] != "https://example.com/doc" || got[0]["link_url"] != nil {
		t.Fatalf("list: %v", got)
	}
	if got := f.call(reader, "GET", "/attachments/"+lid+"/versions", nil); got.status != 200 || len(got.items()) != 1 {
		t.Fatalf("link versions: %+v", got)
	}
	if s, _ := download(owner, "/attachments/"+lid+"/versions/1/download"); s != 422 {
		t.Fatalf("link version download: %d", s)
	}
}
