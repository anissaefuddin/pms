package httpserver_test

import "testing"

// S11.3.7 (NAV-08, NAV-09): bookmark rights follow Space Edit/View, only http(s) URLs are stored, feedback is rate limited and admin-only to read.
func TestBookmarksAndFeedback(t *testing.T) {
	f := newFixture(t)
	admin, editor, viewer, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Docs")
	f.share(space, "user", editor.id, "editor")
	f.share(space, "user", viewer.id, "read")
	base := "/spaces/" + space + "/bookmarks"

	for _, bad := range []string{"javascript:alert(1)", "JavaScript:alert(1)", "data:text/html,x", "ftp://x.example/a", "//x.example", "/relative", "http://", "https://a b.example", ""} {
		f.want(f.call(editor, "POST", base, map[string]any{"url": bad, "title": "x"}), 422, "bad url "+bad)
	}
	f.want(f.call(editor, "POST", base, map[string]any{"url": "https://a.example", "title": " "}), 422, "blank title")
	made := f.call(editor, "POST", base, map[string]any{"url": "https://a.example/x?y=1", "title": "A", "description": "d"})
	f.want(made, 201, "create")
	id := made.body["id"].(string)
	f.want(f.call(editor, "POST", base, map[string]any{"url": "http://b.example", "title": "B"}), 201, "second")

	f.want(f.call(viewer, "POST", base, map[string]any{"url": "https://c.example", "title": "C"}), 403, "viewer cannot add")
	f.want(f.call(viewer, "PATCH", base+"/"+id, map[string]any{"title": "Z"}), 403, "viewer cannot edit")
	f.want(f.call(viewer, "DELETE", base+"/"+id, nil), 403, "viewer cannot delete")
	f.want(f.call(outsider, "GET", base, nil), 404, "outsider list")
	f.want(f.call(outsider, "POST", base, map[string]any{"url": "https://c.example", "title": "C"}), 404, "outsider add")
	list := f.call(viewer, "GET", base, nil)
	f.want(list, 200, "viewer list")
	if got := list.items(); len(got) != 2 || got[0]["title"] != "A" {
		t.Fatalf("list = %v", got)
	}

	f.want(f.call(editor, "PATCH", base+"/"+id, map[string]any{"url": "javascript:x"}), 422, "patch bad url")
	upd := f.call(editor, "PATCH", base+"/"+id, map[string]any{"title": "A2", "position": 5})
	f.want(upd, 200, "patch")
	if upd.body["title"] != "A2" || upd.body["url"] != "https://a.example/x?y=1" {
		t.Fatalf("patch = %v", upd.body)
	}
	other := f.space("Other")
	f.share(other, "user", editor.id, "editor")
	f.want(f.call(editor, "DELETE", "/spaces/"+other+"/bookmarks/"+id, nil), 404, "wrong space")
	f.want(f.call(editor, "DELETE", base+"/"+id, nil), 204, "delete")
	f.want(f.call(editor, "DELETE", base+"/"+id, nil), 404, "delete again")

	fb := map[string]any{"subject": "Love it", "rating": 5, "comment": "great", "page": "/spaces/abc?token=secret#x"}
	f.want(f.call(viewer, "POST", "/feedback", map[string]any{"subject": "", "comment": "x"}), 422, "empty subject")
	f.want(f.call(viewer, "POST", "/feedback", map[string]any{"subject": "s", "rating": 6}), 422, "rating")
	f.want(f.call(viewer, "POST", "/feedback", map[string]any{"subject": "s", "page": "https://evil.example"}), 422, "page")
	f.want(f.call(viewer, "POST", "/feedback", fb), 201, "feedback")
	f.want(f.call(viewer, "GET", "/feedback", nil), 403, "non-admin read")
	got := f.call(admin, "GET", "/feedback", nil)
	f.want(got, 200, "admin read")
	if rows := got.items(); len(rows) != 1 || rows[0]["page"] != "/spaces/abc" {
		t.Fatalf("feedback = %v", rows)
	}
	for i := 0; i < 4; i++ {
		f.want(f.call(viewer, "POST", "/feedback", fb), 201, "within limit")
	}
	limited := f.call(viewer, "POST", "/feedback", fb)
	f.want(limited, 429, "rate limit")
	if limited.header.Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
	f.want(f.call(editor, "POST", "/feedback", fb), 201, "limit is per user")
}
