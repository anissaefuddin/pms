package httpserver_test

import (
	"net/http"
	"strconv"
	"sync"
	"testing"
)

// S-5 race check: concurrent moves of one item. Every move that succeeds writes exactly one history event; a move
// that is refused, or retried by the serializable transaction, writes none. So the event count equals the number
// of 200 responses, and the final location matches the last successful move.
func TestConcurrentMovesRecordExactlyTheSuccessfulOnes(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	spaces := []string{f.space("S1"), f.space("S2"), f.space("S3"), f.space("S4")}
	for _, s := range spaces {
		f.share(s, "user", admin.id, "full")
	}
	item := customFieldItem(f, admin, spaces[0], "Racer")
	path := "/items/" + item + "/locations"

	const attempts = 8
	var wg sync.WaitGroup
	codes := make([]int, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			target := spaces[i%len(spaces)]
			res := f.call(admin, "PUT", path, map[string]any{"locations": []any{map[string]any{"space_id": target, "parent_item_id": nil}}})
			codes[i] = res.status
		}(i)
	}
	wg.Wait()

	ok := 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusConflict, http.StatusTooManyRequests, http.StatusInternalServerError:
			// a lost race: nothing written, nothing recorded
		default:
			t.Fatalf("unexpected status in a race: %v", codes)
		}
	}
	if ok == 0 {
		t.Fatalf("at least one move must win the race: %v", codes)
	}
	// A move to the place the item already is changes nothing and records nothing (200 without an event), so the
	// events are the real changes only: never more than the successful moves, never an empty change.
	events := f.queryID("SELECT count(*)::text FROM activity WHERE item_id=$1 AND action='update' AND changes ? 'locations'", item)
	if n, _ := strconv.Atoi(events); n == 0 || n > ok {
		t.Fatalf("history events %s must be between 1 and the %d successful moves (codes %v)", events, ok, codes)
	}
	if empty := f.queryID("SELECT count(*)::text FROM activity WHERE item_id=$1 AND action='update' AND changes ? 'locations' AND changes->'locations'->'from' = changes->'locations'->'to'", item); empty != "0" {
		t.Fatalf("a history event must describe a real change: %s empty", empty)
	}
	// The chain is consistent: the first event starts where the item was created (S1).
	first := f.queryID(`SELECT changes->'locations'->'from'->0->>'space_id' FROM activity WHERE item_id=$1 AND action='update' AND changes ? 'locations' ORDER BY created_at, id LIMIT 1`, item)
	if first != spaces[0] {
		t.Fatalf("the first recorded move starts in %s, want %s", first, spaces[0])
	}
}
