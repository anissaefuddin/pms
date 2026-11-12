package httpserver_test

import "testing"

func TestCommentLifecycle(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	member := f.person("regular")
	moderator := f.person("regular")
	space := f.space("Comments")
	f.share(space, "user", moderator.id, "full")
	group := f.group("Reviewers", member)
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Review',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	body := func(nodes ...map[string]any) map[string]any {
		return map[string]any{"body": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": nodes}}}}
	}
	path := "/items/" + item + "/comments"
	invalid := body(map[string]any{"type": "text", "text": "Link", "marks": []any{
		map[string]any{"type": "link", "attrs": map[string]any{"href": "javascript:void(0)"}},
	}})
	if got := f.call(admin, "POST", path, invalid); got.status != 422 {
		t.Fatalf("unsafe link: %+v", got)
	}
	r := f.call(admin, "POST", path, body(
		map[string]any{"type": "mention", "attrs": map[string]any{"kind": "user", "id": member.id, "label": "Member"}},
		map[string]any{"type": "mention", "attrs": map[string]any{"kind": "group", "id": group, "label": "Reviewers"}},
		map[string]any{"type": "mention", "attrs": map[string]any{"kind": "followers", "label": "Followers"}},
	))
	if r.status != 201 || r.header.Get("ETag") != `"1"` {
		t.Fatalf("create: %+v", r)
	}
	id := r.body["id"].(string)
	if got := f.queryID(`SELECT count(*)::text FROM comment_mentions WHERE comment_id=$1 AND (
		(mention_type='user' AND target_id=$2) OR (mention_type='group' AND target_id=$3) OR mention_type='followers')`, id, member.id, group); got != "3" {
		t.Fatalf("stored mentions: %s", got)
	}
	if got := f.call(admin, "GET", path, nil); got.status != 200 || len(got.items()) != 1 {
		t.Fatalf("list: %+v", got)
	}
	comment := "/comments/" + id
	updated := body(map[string]any{"type": "text", "text": "Updated"})
	if got := f.call(moderator, "PATCH", comment, updated, "If-Match", `"1"`); got.status != 200 || got.header.Get("ETag") != `"2"` {
		t.Fatalf("edit: %+v", got)
	}
	if got := f.call(admin, "PATCH", comment, updated, "If-Match", `"1"`); got.status != 409 {
		t.Fatalf("stale: %+v", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM comment_mentions WHERE comment_id=$1", id); got != "0" {
		t.Fatalf("old mentions: %s", got)
	}
	if got := f.call(admin, "DELETE", comment, nil); got.status != 204 {
		t.Fatalf("delete: %+v", got)
	}
	if got := f.call(admin, "GET", path, nil); got.status != 200 || len(got.items()) != 0 {
		t.Fatalf("after delete: %+v", got)
	}
}
