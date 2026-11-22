package httpserver_test

import (
	"fmt"
	"slices"
	"testing"
)

func TestSidebarPreferencePersistsAndValidates(t *testing.T) {
	f := newFixture(t)
	member := f.person("regular")
	got := f.call(member, "GET", "/me", nil)
	if hidden, _ := got.body["sidebar_hidden"].([]any); len(hidden) != 0 {
		t.Fatalf("default must show everything: %v", got.body["sidebar_hidden"])
	}
	f.want(f.call(member, "PATCH", "/me", map[string]any{"sidebar_hidden": []string{"stream", "my_todo", "stream"}}), 200, "hide entries")
	hidden := f.call(member, "GET", "/me", nil).body["sidebar_hidden"].([]any)
	if len(hidden) != 2 || hidden[0] != "my_todo" || hidden[1] != "stream" {
		t.Fatalf("hidden = %v", hidden)
	}
	for _, bad := range []any{[]string{"search"}, []string{"inbox"}, "stream", nil} {
		f.want(f.call(member, "PATCH", "/me", map[string]any{"sidebar_hidden": bad}), 422, fmt.Sprintf("%v", bad))
	}
	f.want(f.call(member, "PATCH", "/me", map[string]any{"sidebar_hidden": []string{}}), 200, "show all again")
}

func TestSidebarPinnedKeepsOrderAndValidates(t *testing.T) {
	f := newFixture(t)
	member := f.person("regular")
	pinned := f.call(member, "GET", "/me", nil).body["sidebar_pinned"].([]any)
	if len(pinned) != 2 || pinned[0] != "request_forms" || pinned[1] != "stream" {
		t.Fatalf("default pins = %v", pinned)
	}
	f.want(f.call(member, "PATCH", "/me", map[string]any{"sidebar_pinned": []string{"starred", "my_todo", "stream"}}), 200, "pin in order")
	got := f.call(member, "GET", "/me", nil).body["sidebar_pinned"].([]any)
	if len(got) != 3 || got[0] != "starred" || got[1] != "my_todo" || got[2] != "stream" {
		t.Fatalf("order not kept: %v", got)
	}
	for _, bad := range []any{[]string{"search"}, []string{"inbox"}, []string{"stream", "stream"}, "stream", nil} {
		f.want(f.call(member, "PATCH", "/me", map[string]any{"sidebar_pinned": bad}), 422, fmt.Sprintf("%v", bad))
	}
	f.want(f.call(member, "PATCH", "/me", map[string]any{"sidebar_pinned": []string{}}), 200, "unpin all")
}

func TestStreamFiltersAndVisibility(t *testing.T) {
	f := newFixture(t)
	admin, member, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	shared, hidden := f.space("Shared"), f.space("Hidden")
	f.share(shared, "user", member.id, "read")
	f.share(shared, "user", outsider.id, "read")
	task := func(title, space string) string {
		id := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task',$1,$2) RETURNING id::text", title, admin.id)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
		return id
	}
	followed, assigned, plain, secret := task("Followed", shared), task("Assigned", shared), task("Plain", shared), task("Secret", hidden)
	f.exec("INSERT INTO item_followers(item_id,user_id) VALUES ($1,$2)", followed, member.id)
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", assigned, member.id)
	for _, id := range []string{followed, assigned, plain, secret} {
		f.exec("INSERT INTO activity(item_id,actor_id,action,changes) VALUES ($1,$2,'update','{\"title\":{}}')", id, admin.id)
	}
	f.exec("INSERT INTO activity(item_id,actor_id,action,changes,auto) VALUES ($1,$2,'update','{\"due_date\":{}}',true)", plain, admin.id)
	f.exec(`INSERT INTO comments(id,item_id,author_id,body,body_text) VALUES (gen_random_uuid(),$1,$2,'{"type":"doc"}','mine')`, plain, member.id)
	f.exec(`INSERT INTO comments(id,item_id,author_id,body,body_text) VALUES (gen_random_uuid(),$1,$2,'{"type":"doc"}','not mine')`, followed, admin.id)

	items := func(p *person, query string) []string {
		t.Helper()
		reply := f.call(p, "GET", "/stream"+query, nil)
		f.want(reply, 200, "stream "+query)
		var out []string
		for _, e := range reply.body["items"].([]any) {
			entry := e.(map[string]any)
			out = append(out, entry["kind"].(string)+":"+entry["item"].(map[string]any)["id"].(string))
		}
		return out
	}
	count := func(list []string, id string) int {
		n := 0
		for _, entry := range list {
			if entry[len(entry)-len(id):] == id {
				n++
			}
		}
		return n
	}
	all := items(member, "")
	if count(all, secret) != 0 {
		t.Fatalf("stream leaked an item in an inaccessible space: %v", all)
	}
	if len(all) != 6 { // 3 visible activity rows + the auto row + 2 comments
		t.Fatalf("everything = %v", all)
	}
	if items(outsider, "") == nil || count(items(outsider, ""), secret) != 0 {
		t.Fatal("another user must also be filtered by access")
	}
	if got := items(member, "?filter=followed"); count(got, followed) != 2 || len(got) != 2 {
		t.Fatalf("followed = %v", got)
	}
	if got := items(member, "?filter=assigned"); len(got) != 1 || count(got, assigned) != 1 {
		t.Fatalf("assigned = %v", got)
	}
	if got := items(member, "?filter=conversations"); len(got) != 1 || got[0] != "comment:"+plain {
		t.Fatalf("conversations = %v", got)
	}
	if got := items(member, "?hide_auto=true"); len(got) != 5 {
		t.Fatalf("hide_auto = %v", got)
	}
	// Paging with a tiny limit visits every entry exactly once.
	var seen []string
	cursor := ""
	for range 10 {
		path := "/stream?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		reply := f.call(member, "GET", path, nil)
		f.want(reply, 200, "page")
		for _, e := range reply.body["items"].([]any) {
			seen = append(seen, e.(map[string]any)["kind"].(string)+e.(map[string]any)["id"].(string))
		}
		next, _ := reply.body["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	slices.Sort(seen)
	if len(seen) != 6 || len(slices.Compact(slices.Clone(seen))) != 6 {
		t.Fatalf("paging repeated or lost entries: %v", seen)
	}
	f.want(f.call(member, "GET", "/stream?filter=nope", nil), 422, "unknown filter")
	f.want(f.call(member, "GET", "/stream?cursor=%25%25", nil), 422, "bad cursor")
}

func TestSearchExtraFilters(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	space := f.space("Search space")
	f.share(space, "user", member.id, "read")
	ids := splitComma(f.queryID("SELECT string_agg(id::text||':'||grp::text, ',' ORDER BY position) FROM statuses WHERE workflow_id=(SELECT id FROM workflows WHERE is_default)"))
	statusOf := func(group string) string {
		for _, entry := range ids {
			if len(entry) > len(group) && entry[len(entry)-len(group):] == group {
				return entry[:36]
			}
		}
		t.Fatalf("default workflow has no %s status: %v", group, ids)
		return ""
	}
	task := func(title, status string) string {
		id := f.queryID("INSERT INTO items(type,title,created_by,status_id) VALUES ('task',$1,$2,$3) RETURNING id::text", title, admin.id, status)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
		return id
	}
	open, done := task("Zebra open", statusOf("active")), task("Zebra done", statusOf("completed"))
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", open, member.id)
	f.exec("UPDATE items SET title='Zebra done!' WHERE id=$1", done) // the trigger stamps updated_at, so this makes done the newest
	titles := func(query string) []string {
		reply := f.call(member, "GET", "/search?q=zebra"+query, nil)
		f.want(reply, 200, query)
		var out []string
		for _, e := range reply.body["items"].([]any) {
			out = append(out, e.(map[string]any)["id"].(string))
		}
		return out
	}
	if got := titles(""); len(got) != 2 {
		t.Fatalf("plain = %v", got)
	}
	if got := titles("&assigned_to_me=true"); len(got) != 1 || got[0] != open {
		t.Fatalf("assigned = %v", got)
	}
	if got := titles("&status_group=completed"); len(got) != 1 || got[0] != done {
		t.Fatalf("completed = %v", got)
	}
	if got := titles("&sort=date_modified"); len(got) != 2 || got[0] != done {
		t.Fatalf("newest first = %v", got)
	}
	f.want(f.call(member, "GET", "/search?q=zebra&status_group=bogus", nil), 422, "bad status group")
	f.want(f.call(member, "GET", "/search?q=zebra&sort=bogus", nil), 422, "bad sort")
}
