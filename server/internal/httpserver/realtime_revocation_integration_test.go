package httpserver_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"
)

// B3-16: a live stream is re-authorized for each event. A member who is still subscribed receives changes to a Customer item while
// the membership is active, and nothing after it is revoked, even though the old share row remains stored.
func TestRealtimeStreamStopsAfterMembershipRevoked(t *testing.T) {
	f := newFixture(t)
	admin, watcher := f.person("admin"), f.person("regular")
	client := f.createClient(admin, "RT-A")["id"].(string)
	f.addMember(admin, client, admin)
	f.addMember(admin, client, watcher)
	space := f.scopedSpace("RT client", "client", client)
	f.share(space, "user", admin.id, "full")
	f.share(space, "user", watcher.id, "read")
	item := f.itemIn(space, "client", client, admin.id, "Live client item")
	statuses := []string{}
	rows, err := f.pool.Query(f.ctx, "SELECT id::text FROM statuses ORDER BY position LIMIT 2")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		statuses = append(statuses, id)
	}
	rows.Close()
	if len(statuses) < 2 {
		t.Fatalf("need two statuses: %v", statuses)
	}

	stream := connectSSE(t, f, watcher, "")
	stream.ready(t)
	version := 1
	change := func(status string) {
		t.Helper()
		got := f.call(admin, http.MethodPatch, "/items/"+item, map[string]any{"status_id": status}, "If-Match", `"`+strconv.Itoa(version)+`"`)
		if got.status != http.StatusOK {
			t.Fatalf("change status: %d", got.status)
		}
		version++
	}

	change(statuses[0])
	stream.next(t, 5*time.Second) // positive control: the active member receives the change

	f.revokeMember(admin, client, watcher)
	change(statuses[1])
	select {
	case fr := <-stream.frames:
		if fr.event != "ready" {
			t.Fatalf("a revoked member received a live event: %s", fr.event)
		}
	case <-time.After(2 * time.Second):
	}
}
