package httpserver_test

import "testing"

func TestProjectFiles(t *testing.T) {
	f := newFixture(t)
	admin, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	open, hidden := f.space("Open"), f.space("Hidden")
	f.share(open, "user", reader.id, "read")
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
