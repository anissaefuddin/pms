package httpserver_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
)

func TestAttachmentVersionsAreImmutableAndPolicyChecked(t *testing.T) {
	f := newFixture(t)
	owner, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Versions")
	f.share(space, "user", reader.id, "read")
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Spec',$1) RETURNING id::text", owner.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	send := func(p *person, method, path, name string, data []byte) (int, map[string]any) {
		t.Helper()
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, _ := form.CreateFormFile("file", name)
		part.Write(data)
		form.Close()
		req, _ := http.NewRequest(method, f.srv.URL+"/api/v1"+path, &body)
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
	status, created := send(owner, "POST", "/items/"+item+"/attachments", "spec.txt", []byte("draft one"))
	if status != 201 || created["version"] != float64(1) {
		t.Fatalf("v1: %d %v", status, created)
	}
	id := created["id"].(string)
	vpath := "/attachments/" + id + "/versions"

	if s, _ := send(reader, "POST", vpath, "spec.txt", []byte("sneaky")); s != 403 {
		t.Fatalf("read-only user uploaded a version: %d", s)
	}
	if s, _ := send(outsider, "POST", vpath, "spec.txt", []byte("sneaky")); s != 404 {
		t.Fatalf("outsider upload: %d", s)
	}
	if s, _ := send(owner, "POST", vpath, "run.sh", []byte("#!/bin/sh\n")); s != 422 {
		t.Fatalf("executable version: %d", s)
	}
	status, v2 := send(owner, "POST", vpath, "spec-final.txt", []byte("final text, longer"))
	if status != 201 || v2["version"] != float64(2) || v2["filename"] != "spec-final.txt" || v2["id"] != id {
		t.Fatalf("v2: %d %v", status, v2)
	}

	list := f.call(reader, "GET", vpath, nil)
	vs := list.items()
	if list.status != 200 || len(vs) != 2 || vs[0]["version"] != float64(2) || vs[0]["is_current"] != true || vs[1]["is_current"] != false {
		t.Fatalf("versions: %+v", list)
	}
	if got := f.call(outsider, "GET", vpath, nil); got.status != 404 {
		t.Fatalf("outsider list: %+v", got)
	}
	// The old attachment list keeps its shape and now reports the newest version.
	if got := f.call(reader, "GET", "/items/"+item+"/attachments", nil).items(); len(got) != 1 || got[0]["filename"] != "spec-final.txt" || got[0]["version"] != float64(2) {
		t.Fatalf("attachment list: %v", got)
	}
	// Per-version download: reader may, outsider may not; v1 still has its own bytes and the default follows the newest.
	for _, c := range []struct {
		p      *person
		path   string
		status int
		body   string
	}{
		{reader, vpath + "/1/download", 200, "draft one"},
		{reader, vpath + "/2/download", 200, "final text, longer"},
		{reader, "/attachments/" + id + "/download", 200, "final text, longer"},
		{outsider, vpath + "/1/download", 404, ""},
		{reader, vpath + "/3/download", 404, ""},
		{reader, vpath + "/0/download", 404, ""},
	} {
		if s, b := download(c.p, c.path); s != c.status || (c.status == 200 && b != c.body) {
			t.Fatalf("%s: %d %q", c.path, s, b)
		}
	}
	// History cannot be rewritten.
	if _, err := f.pool.Exec(f.ctx, "UPDATE attachment_versions SET filename='x'"); err == nil {
		t.Fatal("version row was updated")
	}
	if _, err := f.pool.Exec(f.ctx, "DELETE FROM attachment_versions WHERE version=1"); err == nil {
		t.Fatal("version row was deleted")
	}
	// A deleted attachment hides its versions too.
	if got := f.call(owner, "DELETE", "/attachments/"+id, nil); got.status != 204 {
		t.Fatalf("delete: %+v", got)
	}
	if s, _ := download(owner, vpath+"/1/download"); s != 404 {
		t.Fatalf("deleted version download: %d", s)
	}
}
