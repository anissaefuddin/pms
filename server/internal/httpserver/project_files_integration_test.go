package httpserver_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestProjectFiles(t *testing.T) {
	f := newFixture(t)
	admin, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	open, hidden := f.space("Open"), f.space("Hidden")
	f.share(open, "user", reader.id, "read")
	f.share(open, "user", admin.id, "full") // A1.2: the admin manages the project's files in both Spaces as an explicit member
	f.share(hidden, "user", admin.id, "full")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Project',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, open)
	visible := customFieldItem(f, admin, open, "Visible")
	secret := customFieldItem(f, admin, hidden, "Secret")
	for _, id := range []string{visible, secret} {
		f.exec("UPDATE item_locations SET parent_item_id=$1 WHERE item_id=$2", project, id)
	}
	for i, id := range []string{project, visible, secret} {
		f.exec(`INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256)
VALUES (gen_random_uuid(),$1,$2,$3,'text/plain',1,$4,decode(repeat('00',32),'hex'))`, id, admin.id, []string{"p.txt", "v.txt", "s.txt"}[i], []string{"k1", "k2", "k3"}[i])
	}
	f.exec(`INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256,deleted_at)
VALUES (gen_random_uuid(),$1,$2,'gone.txt','text/plain',1,'k4',decode(repeat('00',32),'hex'),now())`, visible, admin.id)
	path := "/items/" + project + "/files"
	for _, tc := range []struct {
		who  *person
		want int
	}{{admin, 3}, {reader, 2}} {
		got := f.call(tc.who, "GET", path, nil)
		f.want(got, 200, "list project files")
		if len(got.items()) != tc.want {
			t.Fatalf("want %d files, got %+v", tc.want, got.items())
		}
	}
	f.want(f.call(outsider, "GET", path, nil), 404, "no access to the project")
}

// The Files view of a folder or project: where each file sits, counts that only include visible files, and server-side filtering.
func TestFilesListIsCountedFilteredAndPaged(t *testing.T) {
	f := newFixture(t)
	admin, reader := f.person("admin"), f.person("regular")
	open, hidden := f.space("Open"), f.space("Hidden")
	f.share(open, "user", reader.id, "read")
	f.share(open, "user", admin.id, "full")
	f.share(hidden, "user", admin.id, "full")
	item := func(kind, title, space, parent string) string {
		id := f.queryID("INSERT INTO items(type,title,created_by) VALUES ($1::item_type,$2,$3) RETURNING id::text", kind, title, admin.id)
		var under any
		if parent != "" {
			under = parent
		}
		f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'U')", id, space, under)
		return id
	}
	folder := item("folder", "Asset Integrity", open, "")
	alpha := item("project", "Alpha", open, folder)
	task := item("task", "Data Collection", open, alpha)
	beta := item("project", "Beta", open, folder)
	secret := item("project", "Secret", hidden, folder)
	n := 0
	attach := func(owner, name, link string) {
		n++
		var url any
		if link != "" {
			url = link
		}
		f.exec(`INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256,link_url,created_at)
VALUES (gen_random_uuid(),$1,$2,$3,'text/plain',1,$4,decode(repeat('00',32),'hex'),$5,timestamptz '2026-11-01' + make_interval(mins => $6))`, owner, admin.id, name, fmt.Sprint("key", n), url, n)
	}
	attach(folder, "Standard.pdf", "")
	attach(folder, "Reference", "https://example.com/ref")
	attach(alpha, "Charter.docx", "")
	attach(task, "Drawing.pdf", "")
	attach(beta, "Contract.pdf", "")
	attach(secret, "Hidden.pdf", "")

	get := func(p *person, query string) reply {
		t.Helper()
		r := f.call(p, "GET", "/items/"+folder+"/files"+query, nil)
		f.want(r, 200, "files"+query)
		return r
	}
	names := func(r reply) string {
		var out []string
		for _, row := range r.items() {
			out = append(out, row["filename"].(string))
		}
		return strings.Join(out, ",")
	}
	all := get(reader, "")
	if got := fmt.Sprint(all.body["counts"]); got != "map[all:5 direct:2 projects:2 subitems:3]" {
		t.Fatalf("counts = %s", got)
	}
	if got := fmt.Sprint(get(admin, "").body["counts"]); got != "map[all:6 direct:2 projects:3 subitems:4]" {
		t.Fatalf("admin counts = %s", got)
	}
	if strings.Contains(fmt.Sprint(all.body), "Secret") {
		t.Fatalf("a hidden project must not be named: %v", all.body)
	}
	paths := map[string]string{}
	for _, row := range all.items() {
		paths[row["filename"].(string)] = fmt.Sprint(row["path"])
	}
	if paths["Standard.pdf"] != "[]" || paths["Charter.docx"] != "[Alpha]" || paths["Drawing.pdf"] != "[Alpha Data Collection]" {
		t.Fatalf("paths = %v", paths)
	}
	for query, want := range map[string]string{
		"?scope=direct":           "Reference,Standard.pdf",
		"?scope=subitems":         "Contract.pdf,Drawing.pdf,Charter.docx",
		"?q=PDF":                  "Contract.pdf,Drawing.pdf,Standard.pdf",
		"?kind=link":              "Reference",
		"?kind=file&scope=direct": "Standard.pdf",
		"?project=" + alpha:       "Drawing.pdf,Charter.docx",
		"?project=" + secret:      "",
		"?review=approved":        "",
		"?limit=2":                "Contract.pdf,Drawing.pdf",
		"?limit=2&offset=4":       "Standard.pdf",
		"?limit=2&offset=40":      "",
	} {
		if got := names(get(reader, query)); got != want {
			t.Errorf("%s: %q, want %q", query, got, want)
		}
	}
	page := get(reader, "?q=pdf&limit=1")
	if page.body["total"] != float64(3) || fmt.Sprint(page.body["counts"]) != "map[all:5 direct:2 projects:2 subitems:3]" {
		t.Fatalf("total follows the filter, counts do not: %v", page.body)
	}
	var projects []string
	for _, p := range all.body["projects"].([]any) {
		m := p.(map[string]any)
		projects = append(projects, fmt.Sprintf("%v:%v", m["title"], m["files"]))
	}
	if got := strings.Join(projects, ","); got != "Alpha:2,Beta:1" {
		t.Fatalf("projects = %s", got)
	}
	for _, bad := range []string{"?scope=x", "?kind=x", "?review=x", "?project=x", "?limit=0", "?limit=201", "?offset=-1", "?limit=a"} {
		f.want(f.call(reader, "GET", "/items/"+folder+"/files"+bad, nil), 422, bad)
	}
}

// A folder may keep a Files view; Timelog and Chart stay project views.
func TestFoldersMayHaveAFilesView(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Views")
	f.share(space, "user", admin.id, "full")
	folder := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('folder','Docs',$1) RETURNING id::text", admin.id)
	task := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Task',$1) RETURNING id::text", admin.id)
	for _, id := range []string{folder, task} {
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
	}
	create := func(item, kind string) reply {
		return f.call(admin, "POST", "/views", map[string]any{"scope_item_id": item, "type": kind, "name": kind, "visibility": "public"})
	}
	f.want(create(folder, "files"), 201, "files on a folder")
	f.want(create(folder, "chart"), 422, "chart on a folder")
	f.want(create(folder, "timelog"), 422, "timelog on a folder")
	f.want(create(task, "files"), 404, "a task has no views")
}

// The Space Files view gathers every item of one Space, judged by what the viewer may do in that Space.
func TestSpaceFiles(t *testing.T) {
	f := newFixture(t)
	admin, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	open, other := f.space("Open"), f.space("Other")
	f.share(open, "user", reader.id, "read")
	f.share(open, "user", admin.id, "full")
	f.share(other, "user", admin.id, "full")
	item := func(kind, title, space, parent string) string {
		id := f.queryID("INSERT INTO items(type,title,created_by) VALUES ($1::item_type,$2,$3) RETURNING id::text", kind, title, admin.id)
		var under any
		if parent != "" {
			under = parent
		}
		f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'U')", id, space, under)
		return id
	}
	folder := item("folder", "Contracts", open, "")
	alpha := item("project", "Alpha", open, folder)
	task := item("task", "Collect", open, alpha)
	loose := item("task", "Loose", open, "")
	elsewhere := item("project", "Elsewhere", other, "")
	gone := item("task", "Gone", open, "")
	n := 0
	attach := func(owner, name string) {
		n++
		f.exec(`INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256,created_at)
VALUES (gen_random_uuid(),$1,$2,$3,'text/plain',1,$4,decode(repeat('00',32),'hex'),timestamptz '2026-11-01' + make_interval(mins => $5))`, owner, admin.id, name, fmt.Sprint("space-key", n), n)
	}
	attach(folder, "folder.txt")
	attach(task, "task.txt")
	attach(loose, "loose.txt")
	attach(elsewhere, "elsewhere.txt")
	attach(gone, "gone.txt")
	f.exec("UPDATE items SET deleted_at=now() WHERE id=$1", gone)

	r := f.call(reader, "GET", "/spaces/"+open+"/files", nil)
	f.want(r, 200, "space files")
	var got []string
	for _, row := range r.items() {
		got = append(got, fmt.Sprintf("%v@%v", row["filename"], row["path"]))
	}
	if s := strings.Join(got, " "); s != "loose.txt@[Loose] task.txt@[Contracts Alpha Collect] folder.txt@[Contracts]" {
		t.Fatalf("rows = %s", s)
	}
	if s := fmt.Sprint(r.body["counts"]); s != "map[all:3 direct:0 projects:1 subitems:3]" {
		t.Fatalf("counts = %s", s)
	}
	if s := f.call(reader, "GET", "/spaces/"+open+"/files?project="+alpha+"&q=task&limit=1", nil); s.body["total"] != float64(1) {
		t.Fatalf("filtered = %v", s.body)
	}
	f.want(f.call(reader, "GET", "/spaces/"+open+"/files?limit=0", nil), 422, "bad page")
	f.want(f.call(outsider, "GET", "/spaces/"+open+"/files", nil), 404, "no access to the Space")
	f.want(f.call(reader, "GET", "/spaces/"+other+"/files", nil), 404, "another Space")
	f.want(f.call(reader, "GET", "/spaces/not-an-id/files", nil), 404, "malformed id")
	f.want(f.call(admin, "POST", "/views", map[string]any{"scope_space_id": open, "type": "files", "name": "Files", "visibility": "public"}), 201, "files view on a Space")
	f.want(f.call(admin, "POST", "/views", map[string]any{"scope_space_id": open, "type": "chart", "name": "Chart", "visibility": "public"}), 422, "chart on a Space")
}

// Each row says what the caller may do with it, by the same rules the attachment endpoints apply.
func TestFileRowsCarryTheCallersRights(t *testing.T) {
	f := newFixture(t)
	full, contributor, reader := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Rights")
	f.share(space, "user", full.id, "full")
	f.share(space, "user", contributor.id, "limited")
	f.share(space, "user", reader.id, "read")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','P',$1) RETURNING id::text", full.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, space)
	attach := func(name string, uploader *person, link any) string {
		return f.queryID(`INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256,link_url)
VALUES (gen_random_uuid(),$1,$2,$3,'text/plain',1,$3,decode(repeat('00',32),'hex'),$4) RETURNING id::text`, project, uploader.id, name, link)
	}
	attach("theirs.txt", full, nil)
	own := attach("mine.txt", contributor, nil)
	attach("board", full, "https://example.com/board")
	for _, path := range []string{"/items/" + project + "/files", "/spaces/" + space + "/files"} {
		for _, tc := range []struct {
			who  *person
			want map[string]string // filename: delete,upload
		}{
			{full, map[string]string{"theirs.txt": "true,true", "mine.txt": "true,true", "board": "true,false"}},
			{contributor, map[string]string{"theirs.txt": "false,true", "mine.txt": "true,true", "board": "false,false"}},
			{reader, map[string]string{"theirs.txt": "false,false", "mine.txt": "false,false", "board": "false,false"}},
		} {
			got := f.call(tc.who, "GET", path, nil)
			f.want(got, 200, "list "+path)
			for _, row := range got.items() {
				name := row["filename"].(string)
				if rights := fmt.Sprintf("%v,%v", row["can_delete"], row["can_upload_version"]); rights != tc.want[name] {
					t.Errorf("%s %s: rights of %s = %s, want %s", tc.who.id, path, name, rights, tc.want[name])
				}
			}
			if len(got.items()) != 3 {
				t.Fatalf("rows: %v", got.items())
			}
		}
	}
	// The flag tells the truth: what it refuses, the endpoint refuses too.
	f.want(f.call(reader, "DELETE", "/attachments/"+own, nil), 403, "a reader cannot delete")
	f.want(f.call(contributor, "DELETE", "/attachments/"+own, nil), 204, "the uploader deletes their own file")
}
