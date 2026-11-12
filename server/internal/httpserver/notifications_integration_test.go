package httpserver_test

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNotificationWorkflow(t *testing.T) {
	f := newFixture(t)
	admin, mentioned, follower, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Inbox")
	f.share(space, "user", mentioned.id, "read")
	f.share(space, "user", follower.id, "read")
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Design review',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	for _, p := range []*person{admin, mentioned, follower, outsider} {
		f.exec("INSERT INTO item_followers(item_id,user_id) VALUES ($1,$2)", item, p.id)
	}
	group := f.group("Reviewers", mentioned, outsider)
	doc := func(mentionType, id, label string) map[string]any {
		return map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{
			map[string]any{"type": "mention", "attrs": map[string]string{"kind": mentionType, "id": id, "label": label}},
		}}}}
	}
	comments := "/items/" + item + "/comments"
	body := map[string]any{"body": doc("group", group, "Reviewers")}
	created := f.call(admin, "POST", comments, body)
	if created.status != 201 {
		t.Fatalf("comment: %+v", created)
	}
	checkInbox := func(p *person, kind string) string {
		t.Helper()
		inbox := f.call(p, "GET", "/notifications", nil)
		if inbox.status != 200 || len(inbox.items()) != 1 || inbox.items()[0]["kind"] != kind || inbox.body["unread_count"] != float64(1) {
			t.Fatalf("inbox %s: %+v", kind, inbox)
		}
		return inbox.items()[0]["id"].(string)
	}
	mentionID := checkInbox(mentioned, "mention")
	checkInbox(follower, "comment")
	for _, p := range []*person{admin, outsider} {
		if got := f.call(p, "GET", "/notifications", nil); got.status != 200 || len(got.items()) != 0 {
			t.Fatalf("unexpected inbox: %+v", got)
		}
	}
	comment := "/comments/" + created.body["id"].(string)
	if got := f.call(admin, "PATCH", comment, body, "If-Match", `"1"`); got.status != 200 {
		t.Fatalf("edit unchanged mention: %+v", got)
	}
	checkInbox(mentioned, "mention")
	if jobs := f.queryID("SELECT count(*)::text FROM river_job WHERE kind='send_notification_email'"); jobs != "1" {
		t.Fatalf("want only the mention email, got %s", jobs)
	}
	target := "/notifications/" + mentionID
	if got := f.call(follower, "PATCH", target, map[string]bool{"read": true}); got.status != 404 {
		t.Fatalf("patch another recipient: %+v", got)
	}
	if got := f.call(mentioned, "PATCH", target, map[string]bool{"read": true}); got.status != 200 || got.body["read_at"] == nil {
		t.Fatalf("read notification: %+v", got)
	}
	if got := f.call(mentioned, "GET", "/notifications?unread_only=true", nil); got.status != 200 || len(got.items()) != 0 {
		t.Fatalf("unread filter: %+v", got)
	}
	path := "/items/" + item
	status := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' LIMIT 1")
	edit := map[string]any{"assignee_ids": []string{mentioned.id, outsider.id}, "status_id": status, "description": doc("user", follower.id, "Follower")}
	if got := f.call(admin, "PATCH", path, edit, "If-Match", `"1"`); got.status != 200 {
		t.Fatalf("notify item edit: %+v", got)
	}
	if got := f.call(admin, "PATCH", path, edit, "If-Match", `"1"`); got.status != 409 {
		t.Fatalf("stale mutation: %+v", got)
	}
	if n := f.queryID("SELECT count(*)::text FROM notifications WHERE item_id=$1 AND kind='assigned' AND recipient_id=$2", item, mentioned.id); n != "1" {
		t.Fatalf("assignment count: %s", n)
	}
	if n := f.queryID("SELECT count(*)::text FROM notifications WHERE item_id=$1 AND kind='status_changed' AND recipient_id=$2", item, follower.id); n != "1" {
		t.Fatalf("status count: %s", n)
	}
	if n := f.queryID("SELECT count(*)::text FROM notifications WHERE item_id=$1 AND kind='mention' AND recipient_id=$2", item, follower.id); n != "1" {
		t.Fatalf("description mention count: %s", n)
	}
	if got := f.call(admin, "PUT", path+"/shares", map[string]any{"shares": []any{map[string]string{
		"principal_type": "user", "principal_id": outsider.id, "role": "read",
	}}}, "If-Match", `"2"`); got.status != 200 {
		t.Fatalf("share: %+v", got)
	}
	checkInbox(outsider, "shared")
	if got := f.call(follower, "POST", "/notifications/read-all", nil); got.status != 204 {
		t.Fatalf("read all: %+v", got)
	}
	if got := f.call(follower, "GET", "/notifications?limit=1", nil); got.status != 200 || got.body["unread_count"] != float64(0) || got.body["next_cursor"] == nil {
		t.Fatalf("read count or paging: %+v", got)
	}
	if got := f.call(follower, "POST", "/notifications/archive-all", nil); got.status != 204 {
		t.Fatalf("archive all: %+v", got)
	}
	if got := f.call(follower, "GET", "/notifications", nil); got.status != 200 || len(got.items()) != 0 {
		t.Fatalf("archive all not reflected: %+v", got)
	}
	f.exec("DELETE FROM shares WHERE target_id=$1 AND principal_id=$2", space, mentioned.id)
	if got := f.call(mentioned, "GET", "/notifications", nil); got.status != 200 || len(got.items()) != 0 || got.body["unread_count"] != float64(0) {
		t.Fatalf("revoked inbox: %+v", got)
	}
	if got := f.call(mentioned, "PATCH", target, map[string]bool{"archived": true}); got.status != 404 {
		t.Fatalf("revoked mutation: %+v", got)
	}
}

func TestNotificationReplayChecksCurrentAccess(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	hidden, visible := f.space("Revoked"), f.space("Still visible")
	f.share(hidden, "user", member.id, "read")
	f.share(visible, "user", member.id, "read")
	connect := func(cursor string) (*bufio.Scanner, func()) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", f.srv.URL+"/api/v1/events", nil)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		req.AddCookie(member.cookie)
		req.Header.Set("Last-Event-ID", cursor)
		res, err := f.srv.Client().Do(req)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		return bufio.NewScanner(res.Body), func() { cancel(); res.Body.Close() }
	}
	ready, closeReady := connect("")
	if !ready.Scan() || !strings.HasPrefix(ready.Text(), "id: ") {
		closeReady()
		t.Fatalf("initial cursor: %q", ready.Text())
	}
	cursor := strings.TrimPrefix(ready.Text(), "id: ")
	closeReady()
	notify := func(space string) string {
		t.Helper()
		item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Review',$1) RETURNING id::text", admin.id)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
		res := f.call(admin, "PATCH", "/items/"+item, map[string]any{"assignee_ids": []string{member.id}}, "If-Match", `"1"`)
		if res.status != 200 {
			t.Fatalf("assignment: %+v", res)
		}
		return f.queryID("SELECT id::text FROM notifications WHERE item_id=$1", item)
	}
	revoked := notify(hidden)
	f.exec("DELETE FROM shares WHERE target_id=$1 AND principal_id=$2", hidden, member.id)
	allowed := notify(visible)
	replayed, closeReplay := connect(cursor)
	defer closeReplay()
	for replayed.Scan() {
		if strings.Contains(replayed.Text(), revoked) {
			t.Fatal("replay returned a notification for an inaccessible Item")
		}
		if strings.Contains(replayed.Text(), allowed) {
			return
		}
	}
	t.Fatalf("authorized notification missing: %v", replayed.Err())
}
