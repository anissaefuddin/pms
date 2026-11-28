package httpserver_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/previews"
	"github.com/anissaefuddin/pms/server/internal/pydiostore"
	"github.com/anissaefuddin/pms/server/internal/storage/cellstest"
	"github.com/google/uuid"
)

type storageFixture struct {
	*fixture
	cfg    config.Config
	admin  *person
	client string
	space  string
}

func newStorageFixture(t *testing.T) storageFixture {
	t.Helper()
	var cfg config.Config
	f := newFixtureWith(t, 1000, func(c *config.Config) { cfg = *c })
	admin := f.person("admin")
	client := f.createClient(admin, "ACME")["id"].(string)
	f.createClient(admin, "BETA")
	f.addMember(admin, client, admin)
	space := f.scopedSpace("Storage", "client", client)
	f.share(space, "user", admin.id, "full")
	return storageFixture{f, cfg, admin, client, space}
}

func (f storageFixture) settings(url string, version int, change map[string]any) map[string]any {
	body := map[string]any{"version": version, "enabled": true, "base_url": url, "root": "pms-files", "token": "cells-token",
		"customer_roots": map[string]any{}, "code_field_id": nil}
	for k, v := range change {
		body[k] = v
	}
	return body
}

func (f storageFixture) create(kind, title, parent string) string {
	location := map[string]any{"space_id": f.space}
	if parent != "" {
		location["parent_item_id"] = parent
	}
	r := f.call(f.admin, http.MethodPost, "/items", map[string]any{"type": kind, "title": title, "locations": []map[string]any{location}})
	f.want(r, http.StatusCreated, kind+" "+title)
	return r.body["id"].(string)
}

func (f storageFixture) upload(path, name string, data []byte) (int, map[string]any) {
	f.t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("file", name)
	part.Write(data)
	form.Close()
	req, _ := http.NewRequest("POST", f.srv.URL+"/api/v1"+path, &body)
	req.AddCookie(f.admin.cookie)
	req.Header.Set("Origin", publicURL)
	req.Header.Set("Content-Type", form.FormDataContentType())
	res, err := f.srv.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func (f storageFixture) download(path string) (int, string) {
	f.t.Helper()
	req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1"+path, nil)
	req.AddCookie(f.admin.cookie)
	res, err := f.srv.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestPydioStorageSettingsAreCheckedEncryptedAndAdministratorOnly(t *testing.T) {
	f := newStorageFixture(t)
	g := cellstest.New(t, "cells-token", "pms-files", "cust-acme")
	member := f.person("regular")
	for _, call := range []struct{ method, path string }{{"GET", "/pydio/storage"}, {"PUT", "/pydio/storage"}, {"POST", "/pydio/storage/test"}} {
		f.want(f.call(member, call.method, call.path, f.settings(g.URL, 0, nil)), http.StatusForbidden, "a regular user "+call.method)
	}
	for name, change := range map[string]map[string]any{
		"http":                  {"base_url": "http://files.example.com"},
		"no token":              {"token": ""},
		"dot root":              {"root": "pms-files/.."},
		"space in root":         {"root": "pms files"},
		"unknown customer":      {"customer_roots": map[string]any{"NOPE": "cust-x"}},
		"overlapping customers": {"customer_roots": map[string]any{"ACME": "cust-acme", "BETA": "cust-acme/beta"}},
		"unknown code field":    {"code_field_id": uuid.NewString()},
	} {
		if r := f.call(f.admin, "PUT", "/pydio/storage", f.settings(g.URL, 0, change)); r.status != 422 {
			t.Fatalf("%s: %d %v", name, r.status, r.body)
		}
	}
	// A setting that cannot write is never saved.
	for name, change := range map[string]map[string]any{"wrong token": {"token": "nope"}, "folder without permission": {"root": "locked"},
		"customer folder without permission": {"customer_roots": map[string]any{"ACME": "locked"}}} {
		r := f.call(f.admin, "PUT", "/pydio/storage", f.settings(g.URL, 0, change))
		if r.status != 422 || r.body["code"] != "pydio_storage_check_failed" {
			t.Fatalf("%s: %d %v", name, r.status, r.body)
		}
	}
	f.want(f.call(f.admin, "POST", "/pydio/storage/test", nil), http.StatusNotFound, "nothing saved")
	saved := f.call(f.admin, "PUT", "/pydio/storage", f.settings(g.URL, 0, map[string]any{"customer_roots": map[string]any{"ACME": "/cust-acme/"}}))
	f.want(saved, http.StatusOK, "save")
	if saved.body["token_set"] != true || saved.body["version"] != float64(1) || strings.Contains(fmt.Sprint(saved.body), "cells-token") ||
		fmt.Sprint(saved.body["customer_roots"]) != "map[ACME:cust-acme]" {
		t.Fatalf("view: %v", saved.body)
	}
	if len(g.Keys()) != 0 {
		t.Fatalf("the check must clean up: %v", g.Keys())
	}
	if stored := f.queryID("SELECT token_enc FROM pydio_storage"); !strings.HasPrefix(stored, "v1:") || strings.Contains(stored, "cells-token") {
		t.Fatalf("the token must be encrypted: %q", stored)
	}
	f.want(f.call(f.admin, "POST", "/pydio/storage/test", nil), http.StatusNoContent, "test")
	f.want(f.call(f.admin, "PUT", "/pydio/storage", f.settings(g.URL, 0, nil)), http.StatusConflict, "stale version")
	moved := f.settings("https://elsewhere.example.com", 1, map[string]any{"token": ""})
	f.want(f.call(f.admin, "PUT", "/pydio/storage", moved), 422, "a saved token never goes to a new address")
	f.want(f.call(f.admin, "PUT", "/pydio/storage", f.settings(g.URL, 1, map[string]any{"token": ""})), http.StatusOK, "same address keeps the token")
	if audit := f.queryID("SELECT string_agg(details::text,' ') FROM audit_events WHERE action='pydio.storage_configured'"); strings.Contains(audit, "cells-token") {
		t.Fatalf("token in the audit log: %s", audit)
	}
}

func TestPydioStorageKeepsEachFileWhereItWasWritten(t *testing.T) {
	f := newStorageFixture(t)
	g := cellstest.New(t, "cells-token", "pms-files", "cust-acme", "special")
	field := f.call(f.admin, "POST", "/spaces/"+f.space+"/custom-fields", map[string]any{"name": "Project Code", "type": "text"})
	f.want(field, http.StatusCreated, "code field")
	codeID := field.body["id"].(string)
	project := f.create("project", "Parent", "")
	version := f.queryID("SELECT version::text FROM items WHERE id=$1", project)
	f.want(f.call(f.admin, "PATCH", "/items/"+project, map[string]any{"fields": map[string]any{codeID: "PRJ A/1"}}, "If-Match", fmt.Sprintf("%q", version)), http.StatusOK, "set code")
	task := f.create("task", "Task", project)
	sub := f.create("project", "Sub", project)
	subTask := f.create("task", "Sub task", sub)
	loose := f.create("task", "Loose", "")
	seq := func(item string) string { return f.queryID("SELECT seq::text FROM items WHERE id=$1", item) }

	_, before := f.upload("/items/"+task+"/attachments", "old.txt", []byte("local bytes"))
	old := before["id"].(string)
	f.want(f.call(f.admin, "PUT", "/pydio/storage", f.settings(g.URL, 0, map[string]any{"code_field_id": codeID})), http.StatusOK, "turn on")

	key := func(attachment string) string {
		return f.queryID("SELECT storage_backend||' '||storage_key FROM attachments WHERE id=$1", attachment)
	}
	put := func(item, name string) string {
		t.Helper()
		status, out := f.upload("/items/"+item+"/attachments", name, []byte("pydio "+name))
		if status != 201 {
			t.Fatalf("upload %s: %d %v", name, status, out)
		}
		return out["id"].(string)
	}
	folderOf := map[string]string{
		task:    "pms-files/ACME/PRJ-A-1_" + seq(project),
		subTask: "pms-files/ACME/P" + seq(sub), // a subproject has a folder of its own, not inside its parent
		loose:   "pms-files/ACME/_items/" + seq(loose),
	}
	var first string
	for item, folder := range folderOf {
		id := put(item, "Report (1).docx") // a type local storage refuses: Pydio allows every type by default
		if got := key(id); !strings.HasPrefix(got, "pydio "+folder+"/"+id+"/") || !strings.HasSuffix(got, "/Report-1.docx") {
			t.Fatalf("key of %s = %q, want folder %q", item, got, folder)
		}
		if item == task {
			first = id
		}
	}
	if s, body := f.download("/attachments/" + first + "/download"); s != 200 || body != "pydio Report (1).docx" {
		t.Fatalf("download through PMS: %d %q", s, body)
	}
	if s, body := f.download("/attachments/" + old + "/download"); s != 200 || body != "local bytes" {
		t.Fatalf("a local file stays readable: %d %q", s, body)
	}
	// A new version is a new file; the old one is untouched.
	if s, out := f.upload("/attachments/"+first+"/versions", "Report (1).docx", []byte("second")); s != 201 {
		t.Fatalf("version: %d %v", s, out)
	}
	if s, body := f.download("/attachments/" + first + "/versions/1/download"); s != 200 || body != "pydio Report (1).docx" {
		t.Fatalf("version 1: %d %q", s, body)
	}
	// A local file can get a Pydio version.
	if s, out := f.upload("/attachments/"+old+"/versions", "old.txt", []byte("now in pydio")); s != 201 {
		t.Fatalf("version of a local file: %d %v", s, out)
	}
	for path, want := range map[string]string{"/versions/1/download": "local bytes", "/download": "now in pydio"} {
		if s, body := f.download("/attachments/" + old + path); s != 200 || body != want {
			t.Fatalf("mixed %s: %d %q", path, s, body)
		}
	}

	// Executables stay refused; the extension list and the size limit of the Pydio tab apply.
	if s, _ := f.upload("/items/"+task+"/attachments", "run.sh", []byte("#!/bin/sh\n")); s != 422 {
		t.Fatalf("executable: %d", s)
	}
	f.want(f.call(f.admin, "PUT", "/pydio/settings", pydioSettings(0, map[string]any{"customer_roots": map[string]any{}, "max_upload_mb": 1, "allowed_extensions": []string{"pdf"}})), http.StatusOK, "limits")
	if s, out := f.upload("/items/"+task+"/attachments", "note.txt", []byte("x")); s != 422 || out["code"] != "file_type_blocked" {
		t.Fatalf("type outside the list: %d %v", s, out)
	}
	if s, _ := f.upload("/items/"+task+"/attachments", "big.pdf", bytes.Repeat([]byte("a"), 2<<20)); s != 413 {
		t.Fatalf("above the Pydio limit: %d", s)
	}
	list := f.call(f.admin, "GET", "/items/"+task+"/attachments", nil)
	if list.body["max_upload_bytes"] != float64(1<<20) {
		t.Fatalf("advertised limit: %v", list.body["max_upload_bytes"])
	}
	f.want(f.call(f.admin, "DELETE", "/pydio/settings", nil), http.StatusNoContent, "default limits")

	// Customer folder, then an administrator's project folder, which wins.
	f.want(f.call(f.admin, "PUT", "/pydio/storage", f.settings(g.URL, 1, map[string]any{"code_field_id": codeID, "customer_roots": map[string]any{"ACME": "cust-acme/files", "BETA": "cust-beta"}, "enabled": false})), http.StatusOK, "customer folders")
	place := func(item, folder, source string) {
		t.Helper()
		r := f.call(f.admin, "GET", "/items/"+item+"/pydio-folder", nil)
		f.want(r, http.StatusOK, "folder")
		if r.body["folder"] != folder || r.body["source"] != source {
			t.Fatalf("folder of %s = %v, want %s (%s)", item, r.body, folder, source)
		}
	}
	place(task, "cust-acme/files/PRJ-A-1_"+seq(project), "customer")
	f.want(f.call(f.admin, "PUT", "/items/"+project+"/pydio-folder", map[string]any{"folder": "cust-beta/x"}), 422, "another Customer's folder")
	f.want(f.call(f.admin, "PUT", "/items/"+project+"/pydio-folder", map[string]any{"folder": "pms-files"}), 422, "the storage root itself")
	f.want(f.call(f.admin, "PUT", "/items/"+task+"/pydio-folder", map[string]any{"folder": "special/a"}), 422, "only a project")
	f.want(f.call(f.admin, "PUT", "/items/"+project+"/pydio-folder", map[string]any{"folder": "special/a"}), http.StatusOK, "override")
	f.want(f.call(f.admin, "PUT", "/items/"+sub+"/pydio-folder", map[string]any{"folder": "special/a/inside"}), 422, "inside another project's folder")
	place(task, "special/a", "project")
	place(subTask, "cust-acme/files/P"+seq(sub), "customer")
	member := f.person("regular")
	f.addMember(f.admin, f.client, member)
	f.share(f.space, "user", member.id, "full")
	f.want(f.call(member, "PUT", "/items/"+project+"/pydio-folder", map[string]any{"folder": "special/b"}), http.StatusForbidden, "administrators only")

	// Off: new files are local again, Pydio files stay readable.
	back := put(task, "after.txt")
	if got := key(back); !strings.HasPrefix(got, "local ") {
		t.Fatalf("storage off must write locally: %q", got)
	}
	if s, _ := f.download("/attachments/" + first + "/download"); s != 200 {
		t.Fatalf("pydio file while off: %d", s)
	}

	// On and unreachable: the upload fails and nothing is recorded or written elsewhere.
	f.want(f.call(f.admin, "PUT", "/pydio/storage", f.settings(g.URL, 2, map[string]any{"code_field_id": codeID})), http.StatusOK, "on again")
	image := put(task, "plan.txt")
	if got := key(image); !strings.HasPrefix(got, "pydio special/a/") {
		t.Fatalf("project folder: %q", got)
	}
	status, picture := f.upload("/items/"+task+"/attachments", "logo.png", pngBytes(40, 20))
	if status != 201 {
		t.Fatalf("image: %d %v", status, picture)
	}
	worker := previews.Worker{Pool: f.pool, Config: f.cfg}
	if err := worker.Generate(f.ctx, uuid.MustParse(picture["id"].(string)), 1); err != nil {
		t.Fatal(err)
	}
	if got := f.queryID("SELECT status FROM attachment_previews WHERE attachment_id=$1", picture["id"]); got != "ready" {
		t.Fatalf("preview of a Pydio file: %s", got)
	}
	count := f.queryID("SELECT count(*)::text FROM attachment_versions")
	g.Set(true, false)
	if s, out := f.upload("/items/"+task+"/attachments", "lost.txt", []byte("x")); s != http.StatusBadGateway || out["code"] != "storage_unavailable" {
		t.Fatalf("gateway down: %d %v", s, out)
	}
	if s, _ := f.download("/attachments/" + first + "/download"); s != http.StatusBadGateway {
		t.Fatalf("download while down: %d", s)
	}
	if got := f.queryID("SELECT count(*)::text FROM attachment_versions"); got != count {
		t.Fatalf("a failed upload must record nothing: %s -> %s", count, got)
	}
}

// Unresolved ownership is quarantined: nobody reaches such an item through the API. Should its folder ever be asked for, it is
// not the internal one, so the files can be found and sorted out.
func TestPydioStorageKeepsUnresolvedOwnershipApart(t *testing.T) {
	f := newStorageFixture(t)
	g := cellstest.New(t, "cells-token", "pms-files", "cust-acme")
	f.want(f.call(f.admin, "PUT", "/pydio/storage", f.settings(g.URL, 0, map[string]any{"customer_roots": map[string]any{"ACME": "cust-acme"}})), http.StatusOK, "turn on")
	space := f.scopedSpace("Legacy", "internal", "")
	f.share(space, "user", f.admin.id, "full")
	item := func(kind, scope string) (string, string) {
		id := f.queryID("INSERT INTO items(type,title,created_by,scope_state) VALUES ($1::item_type,$2,$3,$4) RETURNING id::text", kind, kind+" "+scope, f.admin.id, scope)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
		return id, f.queryID("SELECT seq::text FROM items WHERE id=$1", id)
	}
	internal, internalSeq := item("project", "internal")
	unresolved, unresolvedSeq := item("project", "unresolved")
	loose, looseSeq := item("task", "unresolved")
	saved, _, err := pydiostore.Load(f.ctx, f.pool)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{
		internal:   "pms-files/_internal/P" + internalSeq,
		unresolved: "pms-files/_unresolved/P" + unresolvedSeq,
		loose:      "pms-files/_unresolved/_items/" + looseSeq,
	} {
		place, err := pydiostore.FolderOf(f.ctx, f.pool, saved, uuid.MustParse(id))
		if err != nil || place.Folder != want || place.Source != "default" {
			t.Errorf("folder = %+v %v, want %s", place, err, want)
		}
	}
	f.want(f.call(f.admin, "GET", "/items/"+unresolved+"/pydio-folder", nil), http.StatusNotFound, "quarantined, also for an administrator")
	if status, out := f.upload("/items/"+unresolved+"/attachments", "a.txt", []byte("x")); status != http.StatusNotFound || len(g.Keys()) != 0 {
		t.Fatalf("no upload reaches an unresolved item: %d %v %v", status, out, g.Keys())
	}
}
