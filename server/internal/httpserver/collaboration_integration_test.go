package httpserver_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/security"
)

func TestBrowserCollaboration(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	first, second := f.person("admin"), f.person("regular")
	space := f.space("Browser collaboration")
	f.share(space, "user", second.id, "editor")
	const password = "fixture-browser-password"
	hash, err := security.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	f.exec("UPDATE users SET password_hash=$2 WHERE id=$1", first.id, hash)
	f.exec("UPDATE users SET first_name='Pilot',last_name='Recipient' WHERE id=$1", second.id)
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Original title',$1) RETURNING id::text", first.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	f.exec("INSERT INTO item_followers(item_id,user_id) VALUES ($1,$2)", item, second.id)

	runBrowserFixture(t, f, []string{"collaboration-live.spec.ts", "pilot-live.spec.ts"},
		"PMS_FIXTURE_ITEM="+item,
		"PMS_FIXTURE_SPACE="+space,
		"PMS_FIXTURE_LOGIN_EMAIL="+f.queryID("SELECT email::text FROM users WHERE id=$1", first.id),
		"PMS_FIXTURE_LOGIN_PASSWORD="+password,
		"PMS_FIXTURE_FIRST_COOKIE="+first.cookie.String(),
		"PMS_FIXTURE_SECOND_COOKIE="+second.cookie.String(),
	)
}

func runBrowserFixture(t *testing.T, f *fixture, specs []string, env ...string) {
	t.Helper()
	f.exec(`UPDATE account SET settings=settings||'{"onboarding_done":true}'`) // the first-run wizard would cover the page under test
	args := append([]string{"run", "test:e2e", "--", "--workers=1"}, specs...)
	cmd := exec.CommandContext(t.Context(), "npm", args...)
	cmd.Dir = "../../../web"
	cmd.Env = append(os.Environ(),
		"PMS_FIXTURE_API="+f.srv.URL,
		"PMS_FIXTURE_ORIGIN="+publicURL,
	)
	cmd.Env = append(cmd.Env, env...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Playwright: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}

func TestCollaborationAccessAndAttachmentLifecycle(t *testing.T) {
	f := newFixture(t)
	owner, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Collaboration")
	f.share(space, "user", reader.id, "read")
	f.share(space, "user", owner.id, "full") // A1.2: the owner creates the item and its comments as an explicit member
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Notes',$1) RETURNING id::text", owner.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	path := "/items/" + item
	valid := map[string]any{"body": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Hello"}}}}}}
	comment := f.call(owner, "POST", path+"/comments", valid)
	if comment.status != 201 {
		t.Fatalf("create comment: %+v", comment)
	}
	commentPath := "/comments/" + comment.body["id"].(string)
	payload := []byte("Notes for tomorrow")
	upload := func(p *person, name string, data []byte, want int) string {
		t.Helper()
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, err := form.CreateFormFile("file", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", f.srv.URL+"/api/v1"+path+"/attachments", &body)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(p.cookie)
		req.Header.Set("Origin", publicURL)
		req.Header.Set("Content-Type", form.FormDataContentType())
		res, err := f.srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var result map[string]any
		if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != want {
			t.Fatalf("upload %s: got %d, want %d: %v", name, res.StatusCode, want, result)
		}
		code := map[int]string{413: "file_too_large", 422: "file_type_blocked"}[want]
		if code != "" && result["code"] != code {
			t.Fatalf("upload %s: code %v, want %s", name, result["code"], code)
		}
		id, _ := result["id"].(string)
		return id
	}
	id := upload(owner, "notes.txt", payload, 201)
	attachment := "/attachments/" + id
	sum := sha256.Sum256(payload)
	if got := f.queryID("SELECT encode(sha256,'hex') FROM attachments WHERE id=$1", id); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("stored hash: %s", got)
	}
	elf := []byte("\x7fELF\x00")
	upload(owner, "photo.png", elf, 422)
	var zipped bytes.Buffer
	archive := zip.NewWriter(&zipped)
	entry, err := archive.Create("photo.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(elf); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	upload(owner, "photos.zip", zipped.Bytes(), 422)
	upload(owner, "large.txt", bytes.Repeat([]byte("a"), 1<<20+1), 413)
	upload(outsider, "notes.txt", payload, 404)
	upload(reader, "notes.txt", payload, 403)
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"GET", path + "/comments", nil},
		{"POST", path + "/comments", valid},
		{"PATCH", commentPath, valid},
		{"DELETE", commentPath, nil},
		{"GET", path + "/attachments", nil},
		{"GET", attachment + "/download", nil},
		{"DELETE", attachment, nil},
	} {
		if got := f.call(outsider, tc.method, tc.path, tc.body, "If-Match", `"1"`); got.status != 404 {
			t.Errorf("%s %s: expected 404, got %+v", tc.method, tc.path, got)
		}
	}
	if got := f.call(reader, "GET", path+"/comments", nil); got.status != 200 || len(got.items()) != 1 {
		t.Fatalf("reader comments: %+v", got)
	}
	if got := f.call(reader, "GET", path+"/attachments", nil); got.status != 200 || len(got.items()) != 1 {
		t.Fatalf("reader attachments: %+v", got)
	}
	if got := f.call(reader, "DELETE", attachment, nil); got.status != 403 {
		t.Fatalf("reader delete: %+v", got)
	}
	req, err := http.NewRequest("GET", f.srv.URL+"/api/v1"+attachment+"/download", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(reader.cookie)
	res, err := f.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil || res.StatusCode != 200 || !bytes.Equal(data, payload) ||
		!strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment;") ||
		res.Header.Get("X-Content-Type-Options") != "nosniff" || res.Header.Get("Content-Security-Policy") != "sandbox" {
		t.Fatalf("download: %d %v %q %v", res.StatusCode, res.Header, data, err)
	}
	if got := f.call(owner, "DELETE", attachment, nil); got.status != 204 {
		t.Fatalf("delete attachment: %+v", got)
	}
	if got := f.call(reader, "GET", attachment+"/download", nil); got.status != 404 {
		t.Fatalf("download deleted attachment: %+v", got)
	}
}
