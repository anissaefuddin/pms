package httpserver_test

import (
	"net/http"
	"testing"
	"time"
)

func TestItemCardFields(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Card fields")
	item := customFieldItem(f, admin, space, "Card")
	first := f.call(admin, http.MethodGet, "/items/"+item, nil)
	f.want(first, http.StatusOK, "read")
	if first.body["status_changed_at"] == nil {
		t.Fatalf("status_changed_at missing: %v", first.body)
	}
	if first.body["cover_attachment_id"] != nil {
		t.Fatalf("no attachment yet: %v", first.body["cover_attachment_id"])
	}
	f.exec("UPDATE items SET status_changed_at=now()-interval '10 days' WHERE id=$1", item)
	statuses := f.queryID("SELECT string_agg(id::text, ',' ORDER BY position) FROM statuses WHERE workflow_id=(SELECT id FROM workflows WHERE is_default)")
	ids := splitComma(statuses)
	patch := func(status, etag string) reply {
		return f.call(admin, http.MethodPatch, "/items/"+item, map[string]any{"status_id": status}, "If-Match", etag)
	}
	read := f.call(admin, http.MethodGet, "/items/"+item, nil)
	etag := read.header.Get("ETag")
	same := patch(ids[0], etag)
	f.want(same, http.StatusOK, "status")
	if first := same.body["status"]; first == nil {
		t.Fatalf("status: %v", same.body)
	}
	changedAt := func() time.Time {
		var s string
		s = f.queryID("SELECT status_changed_at::text FROM items WHERE id=$1", item)
		parsed, err := time.Parse("2006-01-02 15:04:05.999999-07", s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		return parsed
	}
	afterFirst := changedAt()
	if time.Since(afterFirst) > time.Minute {
		t.Skip("workflow default status equals the item's current status; nothing to compare")
	}
	f.exec("UPDATE items SET status_changed_at=now()-interval '10 days' WHERE id=$1", item)
	etag = f.call(admin, http.MethodGet, "/items/"+item, nil).header.Get("ETag")
	f.want(patch(ids[0], etag), http.StatusOK, "same status again")
	if time.Since(changedAt()) < 9*24*time.Hour {
		t.Fatal("setting the same status must not reset days in status")
	}
	attachment := f.queryID(`INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256)
VALUES (gen_random_uuid(),$1,$2,'a.png','image/png',1,'k1',decode(repeat('00',32),'hex')) RETURNING id::text`, item, admin.id)
	f.exec(`INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256)
VALUES (gen_random_uuid(),$1,$2,'b.svg','image/svg+xml',1,'k2',decode(repeat('00',32),'hex'))`, item, admin.id)
	if got := f.call(admin, http.MethodGet, "/items/"+item, nil).body["cover_attachment_id"]; got != attachment {
		t.Fatalf("cover = %v, want the first image %s", got, attachment)
	}
}

func splitComma(s string) []string {
	out := []string{}
	for start, i := 0, 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}
