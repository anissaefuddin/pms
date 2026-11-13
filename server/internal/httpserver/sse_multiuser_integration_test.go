package httpserver_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// sseClient is one browser's EventSource: it reads frames in the background and keeps them in order.
type sseClient struct {
	frames chan sseFrame
	cancel context.CancelFunc
}

type sseFrame struct {
	id, event string
	data      map[string]any
	at        time.Time
}

func connectSSE(t *testing.T, f *fixture, p *person, lastID string) *sseClient {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	req, _ := http.NewRequestWithContext(ctx, "GET", f.srv.URL+"/api/v1/events", nil)
	req.AddCookie(p.cookie)
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	res, err := f.srv.Client().Do(req)
	if err != nil || res.StatusCode != 200 {
		cancel()
		t.Fatalf("connect: %v %v", res, err)
	}
	c := &sseClient{frames: make(chan sseFrame, 4096), cancel: cancel}
	go func() {
		defer res.Body.Close()
		scanner := bufio.NewScanner(res.Body)
		var cur sseFrame
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "id: "):
				cur.id = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "event: "):
				cur.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &cur.data)
			case line == "":
				if cur.event != "" {
					cur.at = time.Now()
					c.frames <- cur
				}
				cur = sseFrame{}
			}
		}
	}()
	t.Cleanup(cancel)
	return c
}

// next returns the next frame that is not the "ready" handshake, or fails after the deadline.
func (c *sseClient) next(t *testing.T, wait time.Duration) sseFrame {
	t.Helper()
	for {
		select {
		case fr := <-c.frames:
			if fr.event != "ready" {
				return fr
			}
		case <-time.After(wait):
			t.Fatalf("no event within %s", wait)
		}
	}
}

func (c *sseClient) ready(t *testing.T) string {
	t.Helper()
	select {
	case fr := <-c.frames:
		if fr.event != "ready" {
			t.Fatalf("first frame %q, want ready", fr.event)
		}
		return fr.id
	case <-time.After(3 * time.Second):
		t.Fatal("no ready frame")
		return ""
	}
}

func offset(t *testing.T, id string) int {
	t.Helper()
	_, n, _ := strings.Cut(id, ":")
	v, err := strconv.Atoi(n)
	if err != nil {
		t.Fatalf("bad event id %q", id)
	}
	return v
}

// S7.1.2 (STAB-05): two signed-in users, one edits and one watches. Status and comment changes reach the watcher
// within 3 seconds, in order and exactly once; after a dropped connection the missed events are replayed exactly
// once from Last-Event-ID; an id from another server run, or one older than the history, asks for a full reload.
func TestSSETwoUsersLatencyOrderReconnectAndResync(t *testing.T) {
	f := newFixture(t)
	editor, watcher := f.person("admin"), f.person("regular")
	space := f.space("Live")
	f.share(space, "user", watcher.id, "read")
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Live item',$1) RETURNING id::text", editor.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", item, space)
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
	if len(statuses) < 3 {
		t.Fatalf("statuses: %v", statuses)
	}
	client := connectSSE(t, f, watcher, "")
	client.ready(t)

	// Status and comment changes by another user arrive within the 3 s target.
	version := 1
	patch := func(status string) time.Time {
		t.Helper()
		start := time.Now()
		got := f.call(editor, "PATCH", "/items/"+item, map[string]any{"status_id": status}, "If-Match", `"`+strconv.Itoa(version)+`"`)
		if got.status != 200 {
			t.Fatalf("patch: %+v", got)
		}
		version++
		return start
	}
	start := patch(statuses[1])
	fr := client.next(t, 3*time.Second)
	if fr.event != "item.updated" || fr.data["id"] != item || fr.at.Sub(start) > 3*time.Second {
		t.Fatalf("status event: %+v latency %s", fr, fr.at.Sub(start))
	}
	statusLatency := fr.at.Sub(start)
	start = time.Now()
	comment := map[string]any{"body": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "hi"}}}}}}
	f.want(f.call(editor, "POST", "/items/"+item+"/comments", comment), 201, "comment")
	var sawComment bool
	var commentLatency time.Duration
	for range 3 { // a comment also bumps the item; accept either order but require the comment event
		fr = client.next(t, 3*time.Second)
		if fr.event == "comment.created" {
			sawComment, commentLatency = true, fr.at.Sub(start)
			break
		}
	}
	if !sawComment || commentLatency > 3*time.Second {
		t.Fatalf("comment event missing or slow: %v %s", sawComment, commentLatency)
	}
	t.Logf("latency: status %s, comment %s", statusLatency, commentLatency)

	// Order and duplicates over a burst: ids rise strictly, no id repeats.
	for i := range 20 {
		patch(statuses[i%3])
	}
	seen, last, lastID := map[string]bool{}, 0, ""
	for range 20 {
		fr := client.next(t, 3*time.Second)
		if seen[fr.id] {
			t.Fatalf("duplicate event id %s", fr.id)
		}
		seen[fr.id] = true
		if n := offset(t, fr.id); n <= last {
			t.Fatalf("event %s out of order after %d", fr.id, last)
		} else {
			last, lastID = n, fr.id
		}
	}

	// Reconnect: the connection drops, three events happen meanwhile, the replay returns exactly those three.
	client.cancel()
	for i := range 3 {
		patch(statuses[i%3])
	}
	again := connectSSE(t, f, watcher, lastID)
	var replayed []int
	for range 3 {
		replayed = append(replayed, offset(t, again.next(t, 3*time.Second).id))
	}
	for i := 1; i < len(replayed); i++ {
		if replayed[i] != replayed[i-1]+1 {
			t.Fatalf("replay not contiguous: %v", replayed)
		}
	}
	if replayed[0] != offset(t, lastID)+1 {
		t.Fatalf("replay starts at %d, want %d", replayed[0], offset(t, lastID)+1)
	}
	select {
	case extra := <-again.frames:
		t.Fatalf("replay delivered an extra event: %+v", extra)
	case <-time.After(300 * time.Millisecond):
	}
	again.cancel()

	// An id from another server run (different epoch) or from the future cannot be trusted: the client must reload everything.
	for _, bad := range []string{"other-epoch:5", "garbage"} {
		c := connectSSE(t, f, watcher, bad)
		if fr := c.next(t, 3*time.Second); fr.event != "resync" {
			t.Fatalf("Last-Event-ID %q answered %q, want resync", bad, fr.event)
		}
		c.cancel()
	}
}
