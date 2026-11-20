package httpserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
)

// postItem sends a create from a goroutine without t.Fatal, returning the status and item id.
func postItem(t *testing.T, base string, p *person, body map[string]any) (int, string) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/items", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", publicURL)
	req.AddCookie(p.cookie)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, ""
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	id, _ := out["id"].(string)
	return res.StatusCode, id
}

// R6 characterization: without an Idempotency-Key the server treats every request as new. A double submit that reaches
// the server twice (concurrent, or a retry after a timeout whose response was lost) stores two items.
func TestCreateDuplicatesWithoutIdempotencyKey(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Reliability")
	f.share(space, "user", admin.id, "full")
	body := map[string]any{"type": "task", "title": "Concurrent twice", "locations": []map[string]any{{"space_id": space}}}

	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i := range statuses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			statuses[i], _ = postItem(t, f.srv.URL, admin, body)
		}(i)
	}
	wg.Wait()
	if statuses[0] != 201 || statuses[1] != 201 {
		t.Fatalf("both concurrent requests: %v", statuses)
	}
	if n := f.queryID("SELECT count(*)::text FROM items WHERE title='Concurrent twice'"); n != "2" {
		t.Fatalf("characterization: each request creates an item, got %s", n)
	}

	// A retry after a lost response is the same request again: another item.
	if code, _ := postItem(t, f.srv.URL, admin, body); code != 201 {
		t.Fatalf("retry: %d", code)
	}
	if n := f.queryID("SELECT count(*)::text FROM items WHERE title='Concurrent twice'"); n != "3" {
		t.Fatalf("retry characterization: got %s", n)
	}
}

// R6 custom location: one item can be placed in several locations (Spaces and parents). Each location must read back.
func TestCreateInSeveralCustomLocations(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	first, second := f.space("Custom A"), f.space("Custom B")
	f.share(first, "user", admin.id, "full")
	f.share(second, "user", admin.id, "full")
	parent := f.call(admin, http.MethodPost, "/items", map[string]any{
		"type": "project", "title": "Parent B", "locations": []map[string]any{{"space_id": second}},
	})
	if parent.status != 201 {
		t.Fatalf("parent: %d %v", parent.status, parent.body)
	}
	pid := parent.body["id"].(string)
	created := f.call(admin, http.MethodPost, "/items", map[string]any{
		"type": "task", "title": "Multi location", "locations": []map[string]any{
			{"space_id": first},
			{"space_id": second, "parent_item_id": pid},
		},
	})
	if created.status != 201 {
		t.Fatalf("create in two locations: %d %v", created.status, created.body)
	}
	locs, _ := created.body["locations"].([]any)
	if len(locs) != 2 {
		t.Fatalf("both locations must be returned: %v", created.body["locations"])
	}
	id := created.body["id"].(string)
	read := f.call(admin, http.MethodGet, "/items/"+id, nil)
	if read.status != 200 || len(read.body["locations"].([]any)) != 2 {
		t.Fatalf("readback: %d %v", read.status, read.body["locations"])
	}
	for _, space := range []string{first, second} {
		list := f.call(admin, http.MethodGet, "/spaces/"+space+"/items", nil)
		found := false
		for _, it := range list.items() {
			if it["id"] == id {
				found = true
			}
		}
		if space == second {
			// Under a parent the item is a child, so the Space root list may not show it: check the parent's children.
			found = false
			ch := f.call(admin, http.MethodGet, "/items/"+pid+"/children", nil)
			for _, it := range ch.items() {
				if it["id"] == id {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("the item must be visible in Space %s", space)
		}
	}
}

// Option B: the client may send the item id (UUIDv7). A retry with the same id stores nothing new.
func TestCreateWithClientIDIsIdempotent(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	other := f.person("regular")
	space := f.space("Client id")
	f.share(space, "user", admin.id, "full")
	f.share(space, "user", other.id, "full")
	const v7 = "018f4d3c-9a6b-7c10-8000-00000000abcd"
	body := map[string]any{"id": v7, "type": "task", "title": "Client id task", "locations": []map[string]any{{"space_id": space}}}

	first := f.call(admin, http.MethodPost, "/items", body)
	if first.status != 201 || first.body["id"] != v7 {
		t.Fatalf("create with client id: %d %v", first.status, first.body)
	}
	retry := f.call(admin, http.MethodPost, "/items", body)
	if retry.status != 409 {
		t.Fatalf("retry with the same id must be 409 (already exists), got %d %v", retry.status, retry.body)
	}
	if n := f.queryID("SELECT count(*)::text FROM items WHERE title='Client id task'"); n != "1" {
		t.Fatalf("a retry must not store a second item, got %s", n)
	}
	// Another user cannot take over an id that exists.
	steal := f.call(other, http.MethodPost, "/items", map[string]any{"id": v7, "type": "task", "title": "Stolen", "locations": []map[string]any{{"space_id": space}}})
	if steal.status != 409 {
		t.Fatalf("an id owned by someone else must be 409, got %d %v", steal.status, steal.body)
	}
	if n := f.queryID("SELECT count(*)::text FROM items WHERE title='Stolen'"); n != "0" {
		t.Fatalf("a refused id must store nothing, got %s", n)
	}
	// Only UUIDv7 is accepted; anything else is a validation error.
	if r := f.call(admin, http.MethodPost, "/items", map[string]any{"id": "4b4e5d2a-9f00-4c11-9a00-00000000beef", "type": "task", "title": "v4", "locations": []map[string]any{{"space_id": space}}}); r.status != 422 {
		t.Fatalf("a non-v7 id must be 422 (validation), got %d %v", r.status, r.body)
	}
	if r := f.call(admin, http.MethodPost, "/items", map[string]any{"id": "not-a-uuid", "type": "task", "title": "bad", "locations": []map[string]any{{"space_id": space}}}); r.status != 422 {
		t.Fatalf("a malformed id must be 422 (validation), got %d %v", r.status, r.body)
	}
	// Without an id the create works as before.
	if r := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": "No id", "locations": []map[string]any{{"space_id": space}}}); r.status != 201 {
		t.Fatalf("create without id: %d %v", r.status, r.body)
	}
}
