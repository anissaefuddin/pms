package httpserver_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

// Track A: the per-parent advisory lock taken by item create must not deadlock, leak, or block other parents.

func lockFixture(t *testing.T) (f *fixture, admin *person, space string, mk func(parent, title string) reply) {
	f = newFixture(t)
	admin = f.person("admin")
	space = f.space("Work")
	mk = func(parent, title string) reply {
		loc := map[string]any{"space_id": space}
		if parent != "" {
			loc["parent_item_id"] = parent
		}
		return f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": title, "locations": []map[string]any{loc}})
	}
	return
}

func folder(f *fixture, admin *person, space, title string) string {
	r := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "folder", "title": title, "locations": []map[string]any{{"space_id": space}}})
	f.want(r, 201, "folder "+title)
	return r.body["id"].(string)
}

// Creates, reorders and moves under one parent at the same time: no 5xx, no rank collision.
func TestConcurrentCreateReorderAndMoveShareOneParent(t *testing.T) {
	f, admin, space, mk := lockFixture(t)
	parent, other := folder(f, admin, space, "P"), folder(f, admin, space, "Q")
	seed := make([]string, 4)
	for i := range seed {
		r := mk(parent, "seed")
		f.want(r, 201, "seed")
		seed[i] = r.body["id"].(string)
	}
	var wg sync.WaitGroup
	bad := make(chan string, 200)
	run := func(fn func() reply) {
		defer wg.Done()
		if r := fn(); r.status >= 500 {
			bad <- r.body["detail"].(string)
		}
	}
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go run(func() reply { return mk(parent, "c") })
	}
	for i := 0; i < 6; i++ {
		i := i
		wg.Add(2)
		go run(func() reply {
			return f.call(admin, "POST", "/items/"+seed[i%4]+"/reorder", map[string]any{"space_id": space, "parent_item_id": parent, "after_item_id": seed[(i+1)%4]})
		})
		go run(func() reply {
			return f.call(admin, "PUT", "/items/"+seed[i%4]+"/locations", map[string]any{"locations": []map[string]any{{"space_id": space, "parent_item_id": other}}})
		})
	}
	wg.Wait()
	close(bad)
	for d := range bad {
		t.Fatalf("server error under mixed concurrency: %s", d)
	}
	if n := f.queryID("SELECT count(*)::text FROM (SELECT parent_item_id,rank FROM item_locations WHERE space_id=$1 AND deleted_root_id IS NULL GROUP BY 1,2 HAVING count(*)>1) x", space); n != "0" {
		t.Fatalf("%s duplicate (parent, rank) pairs", n)
	}
}

// Creates that place one item under two parents, in opposite orders, never deadlock (keys are locked in sorted order).
func TestMultiLocationCreatesDoNotDeadlock(t *testing.T) {
	f, admin, space, _ := lockFixture(t)
	a, b := folder(f, admin, space, "A"), folder(f, admin, space, "B")
	var wg sync.WaitGroup
	codes := make(chan int, 40)
	for i := 0; i < 40; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			first, second := a, b
			if i%2 == 1 {
				first, second = b, a
			}
			codes <- f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": "multi", "locations": []map[string]any{
				{"space_id": space, "parent_item_id": first}, {"space_id": space, "parent_item_id": second}}}).status
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("multi-location creates deadlocked")
	}
	close(codes)
	for c := range codes {
		if c != 201 {
			t.Fatalf("multi-location create: %d", c)
		}
	}
}

// A create whose request is cancelled while it waits for the lock gives the lock back: a later create succeeds.
func TestCancelledCreateReleasesParentLock(t *testing.T) {
	f, admin, space, mk := lockFixture(t)
	parent := folder(f, admin, space, "P")
	holder, err := f.pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := holder.Exec(context.Background(), "SELECT pg_advisory_lock(hashtextextended($1,0))", "rank:"+space+":"+parent); err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			_, _ = holder.Exec(context.Background(), "SELECT pg_advisory_unlock(hashtextextended($1,0))", "rank:"+space+":"+parent)
			holder.Release()
		}
	}
	defer release()
	// While the lock is held elsewhere, a create to the same parent waits; a create to another parent does not.
	other := folder(f, admin, space, "Q")
	start := time.Now()
	if r := mk(other, "free"); r.status != 201 || time.Since(start) > 5*time.Second {
		t.Fatalf("a different parent must not wait: %d after %v", r.status, time.Since(start))
	}
	waiting := make(chan reply, 1)
	go func() { waiting <- mk(parent, "waits") }()
	select {
	case r := <-waiting:
		t.Fatalf("create under a locked parent must wait, got %d", r.status)
	case <-time.After(1500 * time.Millisecond):
	}
	release()
	select {
	case r := <-waiting:
		if r.status != 201 {
			t.Fatalf("waiting create after release: %d %v", r.status, r.body)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("create never resumed after the lock was released")
	}
	if r := mk(parent, "after"); r.status != 201 {
		t.Fatalf("lock leaked: %d", r.status)
	}
}
