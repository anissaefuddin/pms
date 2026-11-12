package httpserver_test

import (
	"net/http"
	"net/url"
	"sort"
	"testing"
	"time"
)

func TestItemLists(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("List")
	parent := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Parent',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", parent, space)
	active := f.queryID("SELECT id::text FROM statuses WHERE name='New'")
	completed := f.queryID("SELECT id::text FROM statuses WHERE name='Completed'")
	f.exec(`
WITH inserted AS (
 INSERT INTO items(type,title,created_by,status_id,due_date)
 VALUES ('task','Zulu',$1,$2,NULL),('task','Alpha',$1,$3,'2026-11-12')
 RETURNING id,title
)
INSERT INTO item_locations(item_id,space_id,parent_item_id,rank)
SELECT id,$4,$5,CASE WHEN title='Zulu' THEN 'U' ELSE 'b' END FROM inserted`, admin.id, active, completed, space, parent)
	f.exec("INSERT INTO item_assignees(item_id,user_id) SELECT id,$1 FROM items WHERE title='Alpha'", admin.id)

	root := f.call(admin, http.MethodGet, "/spaces/"+space+"/items", nil)
	if root.status != 200 || len(root.items()) != 1 || root.items()[0]["id"] != parent || root.items()[0]["child_count"] != float64(2) {
		t.Fatalf("root list: %d %v", root.status, root.body)
	}
	children := "/items/" + parent + "/children"
	r := f.call(admin, http.MethodGet, children+"?type=task&sort=due_date&limit=1", nil)
	if r.status != 200 || len(r.items()) != 1 || r.items()[0]["title"] != "Alpha" {
		t.Fatalf("first page: %d %v", r.status, r.body)
	}
	cursor, ok := r.body["next_cursor"].(string)
	if !ok || cursor == "" {
		t.Fatalf("missing cursor: %v", r.body)
	}
	r = f.call(admin, http.MethodGet, children+"?type=task&sort=due_date&limit=1&cursor="+url.QueryEscape(cursor), nil)
	if r.status != 200 || len(r.items()) != 1 || r.items()[0]["title"] != "Zulu" || r.body["next_cursor"] != nil {
		t.Fatalf("second page: %d %v", r.status, r.body)
	}
	r = f.call(admin, http.MethodGet, children+"?type=task&status_group=completed&assignee=me&sort=status", nil)
	if r.status != 200 || len(r.items()) != 1 || r.items()[0]["title"] != "Alpha" {
		t.Fatalf("filtered children: %d %v", r.status, r.body)
	}
	r = f.call(admin, http.MethodGet, children+"?assignee=none&sort=title", nil)
	if r.status != 200 || len(r.items()) != 1 || r.items()[0]["title"] != "Zulu" {
		t.Fatalf("unassigned children: %d %v", r.status, r.body)
	}
}

func TestItemListFiveHundred(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Large list")
	f.exec(`
WITH batch AS (
 INSERT INTO items(type,title,created_by)
 SELECT 'task','Item '||n,$1 FROM generate_series(1,500) n RETURNING id,seq
)
INSERT INTO item_locations(item_id,space_id,rank)
SELECT id,$2,lpad(seq::text,12,'0') FROM batch`, admin.id, space)
	path := "/spaces/" + space + "/items?limit=200"
	seen := map[string]bool{}
	cursor := ""
	for range 3 {
		r := f.call(admin, http.MethodGet, path+"&cursor="+url.QueryEscape(cursor), nil)
		if r.status != 200 {
			t.Fatalf("large list: %d %v", r.status, r.body)
		}
		for _, item := range r.items() {
			id := item["id"].(string)
			if seen[id] {
				t.Fatalf("duplicate across pages: %s", id)
			}
			seen[id] = true
		}
		cursor, _ = r.body["next_cursor"].(string)
	}
	if len(seen) != 500 || cursor != "" {
		t.Fatalf("collected %d items, cursor=%q", len(seen), cursor)
	}
	samples := make([]time.Duration, 20)
	for i := range samples {
		start := time.Now()
		r := f.call(admin, http.MethodGet, path, nil)
		samples[i] = time.Since(start)
		if r.status != 200 || len(r.items()) != 200 {
			t.Fatalf("sample %d: status=%d items=%d", i, r.status, len(r.items()))
		}
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	t.Logf("500 items, 200/page, 20 local HTTP requests: p95=%s", samples[18])
}
