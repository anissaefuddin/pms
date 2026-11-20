package httpserver_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strconv"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/previews"
	"github.com/anissaefuddin/pms/server/internal/storage"
	"github.com/google/uuid"
)

// S11.1.4: compare shows two versions of one file; access is checked per side and mixed files are rejected.
func TestCompareTwoVersionsOfOneFile(t *testing.T) {
	f := newFixture(t)
	owner, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Compare")
	f.share(space, "user", owner.id, "full") // A1.2: the owner creates the item and its attachments as an explicit member
	f.share(space, "user", reader.id, "read")
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Logo',$1) RETURNING id::text", owner.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	upload := func(path, name string, data []byte) string {
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
		id, _ := out["id"].(string)
		return id
	}
	a := upload("/items/"+item+"/attachments", "a.png", pngBytes(40, 20))
	upload("/attachments/"+a+"/versions", "a2.png", pngBytes(60, 30))
	b := upload("/items/"+item+"/attachments", "b.png", pngBytes(10, 10))
	store := storage.LocalStorage{Dir: f.files}
	for _, v := range []struct {
		id string
		n  int
	}{{a, 1}, {a, 2}, {b, 1}} {
		if err := previews.Generate(f.ctx, f.pool, store, uuid.MustParse(v.id), v.n); err != nil {
			t.Fatal(err)
		}
	}
	q := func(la string, lv int, ra string, rv int) string {
		return "/attachment-compare?left_attachment=" + la + "&left_version=" + strconv.Itoa(lv) + "&right_attachment=" + ra + "&right_version=" + strconv.Itoa(rv)
	}
	got := f.call(reader, "GET", q(a, 1, a, 2), nil)
	width := func(side string) any {
		return got.body[side].(map[string]any)["pages"].([]any)[0].(map[string]any)["width"]
	}
	if got.status != 200 || width("left") != float64(40) || width("right") != float64(60) {
		t.Fatalf("compare: %+v", got)
	}
	if s := f.call(outsider, "GET", q(a, 1, a, 2), nil).status; s != 404 {
		t.Fatalf("outsider compare: %d", s)
	}
	if s := f.call(reader, "GET", q(a, 1, a, 3), nil).status; s != 404 {
		t.Fatalf("missing version: %d", s)
	}
	if s := f.call(reader, "GET", q(a, 1, b, 1), nil).status; s != 422 {
		t.Fatalf("versions of different files: %d", s)
	}
	// A file the caller cannot see must not leak as 422 either.
	hidden := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Hidden',$1) RETURNING id::text", owner.id)
	private := f.space("Private")
	f.share(private, "user", owner.id, "full") // A1.2: the owner places the hidden item in its own private Space
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", hidden, private)
	c := upload("/items/"+hidden+"/attachments", "c.png", pngBytes(5, 5))
	if s := f.call(reader, "GET", q(a, 1, c, 1), nil).status; s != 404 {
		t.Fatalf("hidden file: %d", s)
	}
}
