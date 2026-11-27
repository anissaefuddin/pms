package httpserver_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/housekeeping"
	"github.com/anissaefuddin/pms/server/internal/storage"
)

func TestEmptyRecycleBinPurgesOnlyWhatActorMayDelete(t *testing.T) {
	f := newFixture(t)
	admin, reader := f.person("admin"), f.person("regular")
	a, b := f.space("Primary"), f.space("Alternate")
	f.share(a, "user", reader.id, "read")
	f.share(a, "user", admin.id, "full") // A1.2: the admin deletes items in both Spaces as their explicit manager
	f.share(b, "user", admin.id, "full")
	mk := func(kind, title string) string {
		return f.queryID("INSERT INTO items(type,title,created_by,scope_state) VALUES ($1,$2,$3,'internal') RETURNING id::text", kind, title, admin.id)
	}
	place := func(item, space string, parent *string) {
		f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'A')", item, space, parent)
	}
	parent, child, multi, gone := mk("project", "Purge parent"), mk("task", "Purge child"), mk("task", "Purge multi"), mk("task", "Purge other")
	place(parent, a, nil)
	place(child, a, &parent)
	place(multi, a, &parent)
	place(multi, b, nil)
	place(gone, b, nil)
	key := "aa/bb/aabbccdd-1111-4222-8333-444455556666"
	if err := os.MkdirAll(filepath.Join(f.files, "aa", "bb"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.files, key), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.exec(`INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256)
VALUES (gen_random_uuid(),$1,$2,'a.bin','application/octet-stream',1,$3,decode(repeat('00',32),'hex'))`, child, admin.id, key)
	f.exec("INSERT INTO comments(id,item_id,author_id,body,body_text) VALUES (gen_random_uuid(),$1,$2,'{}','c')", child, admin.id)
	for _, id := range []string{parent, gone} {
		if got := f.call(admin, "DELETE", "/items/"+id, nil); got.status != 204 {
			t.Fatalf("delete %s: %+v", id, got)
		}
	}
	// A reader sees the parent in the bin (read access) but may not delete it; nothing is purged.
	if got := f.call(reader, "DELETE", "/recycle-bin", nil); got.status != 200 || got.body["deleted"] != float64(0) {
		t.Fatalf("reader empty: %+v", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM items WHERE id=ANY($1::uuid[])", []string{parent, child, gone}); got != "3" {
		t.Fatalf("reader purged items: %s", got)
	}
	if got := f.call(admin, "DELETE", "/recycle-bin", nil); got.status != 200 || got.body["deleted"] != float64(2) {
		t.Fatalf("admin empty: %+v", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM items WHERE id=ANY($1::uuid[])", []string{parent, child, gone}); got != "0" {
		t.Fatalf("items left: %s", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1", child); got != "0" {
		t.Fatalf("comments left: %s", got)
	}
	// The live item that sat in two places keeps its other location and loses the dead one.
	if got := f.queryID("SELECT count(*)::text FROM item_locations WHERE item_id=$1 AND space_id=$2 AND parent_item_id IS NULL AND deleted_root_id IS NULL", multi, b); got != "1" {
		t.Fatalf("multi location: %s", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM item_locations WHERE item_id=$1", multi); got != "1" {
		t.Fatalf("multi dead location kept: %s", got)
	}
	if got := f.call(admin, "GET", "/recycle-bin", nil); got.status != 200 || len(got.items()) != 0 {
		t.Fatalf("bin not empty: %+v", got)
	}
	// No orphans: housekeeping removes the file whose row went with the item.
	removed, err := housekeeping.SweepFiles(f.ctx, f.pool, storage.LocalStorage{Dir: f.files}, -time.Minute)
	if err != nil || removed != 1 {
		t.Fatalf("sweep removed %d, err %v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(f.files, key)); !os.IsNotExist(err) {
		t.Fatalf("file still there: %v", err)
	}
}
