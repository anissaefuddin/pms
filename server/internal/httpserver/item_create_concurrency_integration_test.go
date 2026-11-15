package httpserver_test

import (
	"net/http"
	"sync"
	"testing"
)

// QA-D-lane3-002: independent creates under one parent must all succeed, not fail with a serialization 409.
func TestConcurrentCreatesUnderOneParentAllSucceed(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Work")
	parent := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "folder", "title": "Parent", "locations": []map[string]any{{"space_id": space}}})
	f.want(parent, 201, "parent")
	pid := parent.body["id"].(string)
	const n = 12
	got := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i] = f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": "T", "locations": []map[string]any{{"space_id": space, "parent_item_id": pid}}}).status
		}()
	}
	wg.Wait()
	for i, s := range got {
		if s != 201 {
			t.Fatalf("create %d: status %d, all of %v must be 201", i, s, got)
		}
	}
}
