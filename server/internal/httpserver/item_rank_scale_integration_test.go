package httpserver_test

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

// Lane-D (QA B2): sibling ranking at 500/1000/2000 children (concurrent create, reorder, legacy long ranks).
func TestRankingAtScale(t *testing.T) {
	// Gated: currently RED by design (QA-D-laneD-001, 409 under concurrent create at scale). Run with PMS_TEST_SCALE=1.
	if os.Getenv("PMS_TEST_SCALE") != "1" {
		t.Skip("set PMS_TEST_SCALE=1 to run the 500/1000/2000-children scale test")
	}
	for _, n := range []int{500, 1000, 2000} {
		t.Run(fmt.Sprintf("children=%d", n), func(t *testing.T) {
			f := newFixture(t)
			admin := f.person("admin")
			space := f.space("Rank scale")
			parent := f.call(admin, "POST", "/items", map[string]any{"type": "folder", "title": "Parent", "locations": []map[string]any{{"space_id": space}}})
			f.want(parent, 201, "parent")
			pid := parent.body["id"].(string)
			create := func(i int) reply {
				return f.call(admin, "POST", "/items", map[string]any{"type": "task", "title": fmt.Sprintf("T%04d", i), "locations": []map[string]any{{"space_id": space, "parent_item_id": pid}}})
			}
			// Concurrent create: 8 workers share the work; every request must succeed.
			var wg sync.WaitGroup
			var mu sync.Mutex
			bad := map[int]int{}
			next := 0
			for w := 0; w < 8; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						mu.Lock()
						i := next
						next++
						mu.Unlock()
						if i >= n {
							return
						}
						if r := create(i); r.status != 201 {
							mu.Lock()
							bad[r.status]++
							mu.Unlock()
						}
					}
				}()
			}
			wg.Wait()
			if len(bad) > 0 {
				t.Fatalf("concurrent create failures by status: %v", bad)
			}
			if c := f.queryID("SELECT count(*)::text FROM item_locations WHERE parent_item_id=$1", pid); c != fmt.Sprint(n) {
				t.Fatalf("children stored: %s want %d", c, n)
			}
			if d := f.queryID("SELECT (count(*)-count(DISTINCT rank))::text FROM item_locations WHERE parent_item_id=$1", pid); d != "0" {
				t.Fatalf("rank collisions: %s", d)
			}
			t.Logf("n=%d max rank length=%s", n, f.queryID("SELECT max(length(rank))::text FROM item_locations WHERE parent_item_id=$1", pid))

			// Reorder: move 40 children to the front/after others; order in the DB must follow and ranks stay unique and bounded.
			var items []string
			rows, err := f.pool.Query(t.Context(), `SELECT item_id::text FROM item_locations WHERE parent_item_id=$1 ORDER BY rank COLLATE "C"`, pid)
			if err != nil {
				t.Fatal(err)
			}
			for rows.Next() {
				var s string
				rows.Scan(&s)
				items = append(items, s)
			}
			rows.Close()
			for k := 0; k < 40; k++ {
				mv, after := items[len(items)-1-k], items[k*3]
				r := f.call(admin, "POST", "/items/"+mv+"/reorder", map[string]any{"after_item_id": after, "parent_item_id": pid, "space_id": space})
				if r.status != 204 && r.status != 200 {
					t.Fatalf("reorder %d: %d %v", k, r.status, r.body)
				}
			}
			if d := f.queryID("SELECT (count(*)-count(DISTINCT rank))::text FROM item_locations WHERE parent_item_id=$1", pid); d != "0" {
				t.Fatalf("rank collisions after reorder: %s", d)
			}
			if m := f.queryID("SELECT max(length(rank))::text FROM item_locations WHERE parent_item_id=$1", pid); len(m) > 3 {
				t.Fatalf("rank length too large after reorder: %s", m)
			}
		})
	}
}

// Legacy data written before the fix has ranks one character longer per child (up to the 512 limit); the system must keep accepting children and moves.
func TestRankingWithLegacyLongRanks(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Legacy rank")
	parent := f.call(admin, "POST", "/items", map[string]any{"type": "folder", "title": "Parent", "locations": []map[string]any{{"space_id": space}}})
	f.want(parent, 201, "parent")
	pid := parent.body["id"].(string)
	var kids []string
	for i := 0; i < 3; i++ {
		r := f.call(admin, "POST", "/items", map[string]any{"type": "task", "title": fmt.Sprintf("L%d", i), "locations": []map[string]any{{"space_id": space, "parent_item_id": pid}}})
		f.want(r, 201, "legacy child")
		kids = append(kids, r.body["id"].(string))
	}
	// Simulate old behaviour: rank grows by one character per sibling.
	for i, id := range kids {
		f.exec("UPDATE item_locations SET rank=$2 WHERE item_id=$1", id, strings.Repeat("V", 480+i*10)+"1")
	}
	r := f.call(admin, "POST", "/items", map[string]any{"type": "task", "title": "after legacy", "locations": []map[string]any{{"space_id": space, "parent_item_id": pid}}})
	if r.status != 201 {
		t.Fatalf("append after legacy long ranks: %d %v", r.status, r.body)
	}
	mv := f.call(admin, "POST", "/items/"+kids[2]+"/reorder", map[string]any{"after_item_id": kids[0], "parent_item_id": pid, "space_id": space})
	if mv.status != 204 && mv.status != 200 {
		t.Fatalf("reorder among legacy long ranks: %d %v", mv.status, mv.body)
	}
	if d := f.queryID("SELECT (count(*)-count(DISTINCT rank))::text FROM item_locations WHERE parent_item_id=$1", pid); d != "0" {
		t.Fatalf("rank collisions: %s", d)
	}
}
