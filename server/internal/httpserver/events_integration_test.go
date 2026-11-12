package httpserver_test

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEventReplayRespectsCurrentAccess(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	privateSpace, publicSpace := f.space("Private"), f.space("Visible")
	f.share(privateSpace, "user", member.id, "read")
	f.share(publicSpace, "user", member.id, "read")
	makeItem := func(space string) string {
		id := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Original',$1) RETURNING id::text", admin.id)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
		return id
	}
	privateItem, publicItem := makeItem(privateSpace), makeItem(publicSpace)
	connect := func(cursor string) (*bufio.Scanner, func()) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		t.Cleanup(cancel)
		req, err := http.NewRequestWithContext(ctx, "GET", f.srv.URL+"/api/v1/events", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(member.cookie)
		req.Header.Set("Last-Event-ID", cursor)
		res, err := f.srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { res.Body.Close() })
		if res.StatusCode != 200 || res.Header.Get("Content-Type") != "text/event-stream" {
			t.Fatalf("stream response: %d %v", res.StatusCode, res.Header)
		}
		return bufio.NewScanner(res.Body), func() { cancel(); res.Body.Close() }
	}
	initial, closeInitial := connect("")
	if !initial.Scan() || !strings.HasPrefix(initial.Text(), "id: ") {
		t.Fatalf("initial cursor: %q %v", initial.Text(), initial.Err())
	}
	cursor := strings.TrimPrefix(initial.Text(), "id: ")
	closeInitial()
	patch := func(item string) {
		t.Helper()
		res := f.call(admin, "PATCH", "/items/"+item, map[string]any{"title": "Changed"}, "If-Match", `"1"`)
		if res.status != 200 {
			t.Fatalf("patch: %+v", res)
		}
	}
	patch(privateItem)
	f.exec("DELETE FROM shares WHERE target_id=$1 AND principal_id=$2", privateSpace, member.id)
	patch(publicItem)

	replayed, closeReplay := connect(cursor)
	defer closeReplay()
	for replayed.Scan() {
		line := replayed.Text()
		if strings.Contains(line, privateItem) || strings.Contains(line, privateSpace) {
			t.Fatal("replay included an item whose access was revoked")
		}
		if strings.Contains(line, publicItem) && strings.Contains(line, publicSpace) {
			break
		}
	}
	if !strings.Contains(replayed.Text(), publicItem) {
		t.Fatalf("authorized event missing: %v", replayed.Err())
	}
	closeReplay()

	// A direct share can see the removal without learning the containing Space ID.
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,'user',$2,'read')", privateItem, member.id)
	deleted, closeDeleted := connect("")
	if res := f.call(admin, "DELETE", "/items/"+privateItem, nil); res.status != 204 {
		t.Fatalf("delete: %+v", res)
	}
	for deleted.Scan() {
		if strings.HasPrefix(deleted.Text(), "data: ") && strings.Contains(deleted.Text(), privateItem) {
			break
		}
	}
	if !strings.Contains(deleted.Text(), privateItem) || strings.Contains(deleted.Text(), privateSpace) {
		t.Fatalf("direct-share deletion event: %q, %v", deleted.Text(), deleted.Err())
	}
	closeDeleted()

	resynced, closeResync := connect("previous-process:0")
	defer closeResync()
	for resynced.Scan() {
		if resynced.Text() == "event: resync" {
			return
		}
	}
	t.Fatalf("resync missing: %v", resynced.Err())
}

func TestEventHeartbeat(t *testing.T) {
	if os.Getenv("PMS_TEST_SSE_HEARTBEAT") != "1" {
		t.Skip("set PMS_TEST_SSE_HEARTBEAT=1 to check the 25-second interval")
	}
	f := newFixture(t)
	member := f.person("regular")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", f.srv.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(member.cookie)
	res, err := f.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		if scanner.Text() == ": heartbeat" {
			return
		}
	}
	t.Fatalf("heartbeat missing: %v", scanner.Err())
}
