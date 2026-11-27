package httpserver_test

import (
	"fmt"
	"slices"
	"testing"
	"time"
)

// RT-37: server-side hierarchical search returns the ancestor path of each hit and pages with offset,
// and the path never reveals ancestors the caller cannot view.
func TestSearchHierarchyPathAndOffset(t *testing.T) {
	f := newFixture(t)
	admin, member, limited := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Hier")
	f.share(space, "user", member.id, "read")
	item := func(kind, title string, parent *string) string {
		id := f.queryID("INSERT INTO items(type,title,created_by,scope_state) VALUES ($1,$2,$3,'internal') RETURNING id::text", kind, title, admin.id)
		f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'U')", id, space, parent)
		return id
	}
	root := item("project", "ITA root", nil)
	mid := item("folder", "Kurau Plant", &root)
	leaf := item("task", "HAZOP session", &mid)
	item("task", "HAZOP second", &mid)
	// limited user sees only the folder subtree, not the project above it
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,'user',$2,'read')", mid, limited.id)

	titles := func(v any) []string {
		var out []string
		for _, p := range v.([]any) {
			out = append(out, p.(map[string]any)["title"].(string))
		}
		return out
	}
	find := func(r reply, id string) map[string]any {
		for _, v := range r.body["items"].([]any) {
			if m := v.(map[string]any); m["id"] == id {
				return m
			}
		}
		t.Fatalf("hit %s missing: %v", id, r.body)
		return nil
	}
	r := f.call(member, "GET", "/search?q=hazop&space_id="+space+"&container_id="+root, nil)
	path, ok := find(r, leaf)["path"]
	if !ok {
		t.Fatalf("no path on hit: %v", r.body)
	}
	if got := titles(path); len(got) != 2 || got[0] != "ITA root" || got[1] != "Kurau Plant" {
		t.Fatalf("member path = %v", got)
	}
	r = f.call(limited, "GET", "/search?q=hazop", nil)
	if got := titles(find(r, leaf)["path"]); len(got) != 1 || got[0] != "Kurau Plant" {
		t.Fatalf("limited path leaks hidden ancestor: %v", got)
	}
	// paging: one hit per page, the second page holds the other hit
	first := f.call(member, "GET", "/search?q=hazop&limit=1", nil)
	second := f.call(member, "GET", "/search?q=hazop&limit=1&offset=1", nil)
	a, b := first.body["items"].([]any), second.body["items"].([]any)
	if len(a) != 1 || len(b) != 1 || a[0].(map[string]any)["id"] == b[0].(map[string]any)["id"] {
		t.Fatalf("offset paging: %v / %v", first.body, second.body)
	}
	if r := f.call(member, "GET", "/search?q=hazop&offset=-1", nil); r.status != 422 {
		t.Fatalf("negative offset: %d", r.status)
	}
}

// Benchmark of scoped hierarchical search (with ancestor paths) on 500, 1,000 and 10,000 items arranged
// as project > folder > subfolder > task. Logs p95; fails only on a loose budget.
func TestSearchHierarchyBenchmark(t *testing.T) {
	for _, n := range []int{500, 1000, 10000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			f := newFixture(t)
			admin, member := f.person("admin"), f.person("regular")
			space := f.space("Bench")
			f.share(space, "user", member.id, "read")
			root := f.queryID("INSERT INTO items(type,title,created_by,scope_state) VALUES ('project','Bench root',$1,'internal') RETURNING id::text", admin.id)
			f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", root, space)
			// 10 folders, each with sub-folders, the rest are leaf tasks spread under the sub-folders
			f.exec(`WITH folders AS (
  INSERT INTO items(type,title,created_by,scope_state) SELECT 'folder','Folder '||g,$1,'internal' FROM generate_series(1,10) g RETURNING id,title
), placed AS (
  INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) SELECT id,$2,$3,'U' FROM folders RETURNING item_id
) SELECT count(*) FROM placed`, admin.id, space, root)
			f.exec(`WITH subs AS (
  INSERT INTO items(type,title,created_by,scope_state) SELECT 'folder','Sub '||p.title||'.'||g,$1,'internal' FROM items p, generate_series(1,5) g WHERE p.title LIKE 'Folder %' RETURNING id,title
), placed AS (
  INSERT INTO item_locations(item_id,space_id,parent_item_id,rank)
  SELECT s.id,$2,(SELECT id FROM items WHERE title=split_part(substr(s.title,5),'.',1)),'U' FROM subs s RETURNING item_id
) SELECT count(*) FROM placed`, admin.id, space)
			f.exec(`WITH tasks AS (
  INSERT INTO items(type,title,created_by,scope_state) SELECT 'task','HAZOP item '||g,$1,'internal' FROM generate_series(1,$3::int) g RETURNING id,seq
) INSERT INTO item_locations(item_id,space_id,parent_item_id,rank)
SELECT t.id,$2,(SELECT id FROM items WHERE title LIKE 'Sub Folder %' ORDER BY title OFFSET (t.seq % 50) LIMIT 1),'U' FROM tasks t`, admin.id, space, n)
			f.exec("ANALYZE items")
			f.exec("ANALYZE item_locations")
			var durations []time.Duration
			for i := -3; i < 20; i++ {
				start := time.Now()
				r := f.call(member, "GET", "/search?q=hazop&limit=50&space_id="+space+"&container_id="+root, nil)
				if r.status != 200 || len(r.body["items"].([]any)) == 0 {
					t.Fatalf("search: %d %v", r.status, r.body)
				}
				if first := r.body["items"].([]any)[0].(map[string]any); first["path"] == nil {
					t.Fatalf("no path: %v", first)
				}
				if i >= 0 {
					durations = append(durations, time.Since(start))
				}
			}
			slices.Sort(durations)
			p95 := durations[len(durations)-2]
			t.Logf("%d tasks, scoped to project, limit 50 with ancestor paths: p50=%v p95=%v max=%v", n, durations[len(durations)/2], p95, durations[len(durations)-1])
			if p95 > 800*time.Millisecond {
				t.Fatalf("p95 %v over budget", p95)
			}
		})
	}
}
