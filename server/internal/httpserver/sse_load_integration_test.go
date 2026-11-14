package httpserver_test

import (
	"fmt"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

// S6.3.4 (NFR "Real-time"): at the size of a client (25 people, 10,000 items, every person watching), a status change or a comment by one
// person shows up for every other person within 3 seconds. p95 over all watchers and samples is what the target names.
func TestSSELatencyUnderClientLoad(t *testing.T) {
	f := newFixture(t)
	editor := f.person("admin")
	space := f.space("Busy")
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Hot item',$1) RETURNING id::text", editor.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", item, space)
	// Client scale: 10,000 items in the same space.
	f.exec(`WITH n AS (INSERT INTO items(type,title,created_by) SELECT 'task','Filler '||g,$1 FROM generate_series(1,10000) g RETURNING id)
INSERT INTO item_locations(item_id,space_id,rank) SELECT id,$2,'B'||lpad((row_number() OVER ())::text,6,'0') FROM n`, editor.id, space)

	const watchers = 24 // with the editor, 25 seats
	clients := make([]*sseClient, watchers)
	for i := range clients {
		p := f.person("regular")
		f.share(space, "user", p.id, "read")
		clients[i] = connectSSE(t, f, p, "")
		clients[i].ready(t)
	}
	statuses := []string{}
	rows, err := f.pool.Query(f.ctx, "SELECT id::text FROM statuses ORDER BY position LIMIT 3")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		statuses = append(statuses, id)
	}
	rows.Close()

	body := func(text string) map[string]any {
		return map[string]any{"body": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text}}}}}}
	}
	// waitAll collects, per watcher, the time its first frame of the wanted kind arrived.
	waitAll := func(event string, start time.Time) []time.Duration {
		t.Helper()
		out := make([]time.Duration, watchers)
		var wg sync.WaitGroup
		for i, c := range clients {
			wg.Add(1)
			go func() {
				defer wg.Done()
				deadline := time.After(10 * time.Second)
				for {
					select {
					case fr := <-c.frames:
						if fr.event == event && fr.data["id"] == item || fr.event == event && fr.data["item_id"] == item {
							out[i] = fr.at.Sub(start)
							return
						}
					case <-deadline:
						out[i] = -1
						return
					}
				}
			}()
		}
		wg.Wait()
		return out
	}

	var status, comment []time.Duration
	version := 1
	for i := range 15 {
		start := time.Now()
		f.want(f.call(editor, "PATCH", "/items/"+item, map[string]any{"status_id": statuses[(i+1)%3]}, "If-Match", `"`+strconv.Itoa(version)+`"`), 200, "patch")
		version++
		status = append(status, waitAll("item.updated", start)...)
		start = time.Now()
		f.want(f.call(editor, "POST", "/items/"+item+"/comments", body(fmt.Sprintf("note %d", i))), 201, "comment")
		comment = append(comment, waitAll("comment.created", start)...)
		// a comment bumps the item version; reread it so the next patch carries the right one
		cur := f.call(editor, "GET", "/items/"+item, nil)
		version = int(cur.body["version"].(float64))
		for _, c := range clients { // drop the item.updated the comment caused
			for len(c.frames) > 0 {
				<-c.frames
			}
		}
	}
	p95 := func(name string, samples []time.Duration) time.Duration {
		t.Helper()
		for _, d := range samples {
			if d < 0 {
				t.Fatalf("%s: a watcher never got the event", name)
			}
		}
		slices.Sort(samples)
		got := samples[len(samples)*95/100]
		t.Logf("%s: %d samples (%d watchers x %d changes), p50 %s, p95 %s, max %s", name, len(samples), watchers, len(samples)/watchers, samples[len(samples)/2], got, samples[len(samples)-1])
		return got
	}
	if got := p95("status", status); got > 3*time.Second {
		t.Fatalf("status p95 %s exceeds 3s", got)
	}
	if got := p95("comment", comment); got > 3*time.Second {
		t.Fatalf("comment p95 %s exceeds 3s", got)
	}
}
