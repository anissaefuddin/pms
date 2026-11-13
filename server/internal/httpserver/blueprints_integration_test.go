package httpserver_test

import "testing"

func TestBlueprintSaveAndUse(t *testing.T) {
	f := newFixture(t)
	admin, editor, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	space, other := f.space("Launches"), f.space("Other")
	f.share(space, "user", editor.id, "editor")
	f.share(space, "user", reader.id, "read")
	f.share(other, "user", editor.id, "editor")

	post := func(p *person, path string, body any) reply { return f.call(p, "POST", path, body) }
	root := post(admin, "/items", map[string]any{"type": "project", "title": "Launch", "locations": []map[string]any{{"space_id": space}},
		"importance": "high", "assignee_ids": []string{editor.id}})
	f.want(root, 201, "create root")
	rootID := root.body["id"].(string)
	child := func(title, start, due string) string {
		r := post(admin, "/items", map[string]any{"type": "task", "title": title, "start_date": start, "due_date": due,
			"locations": []map[string]any{{"space_id": space, "parent_item_id": rootID}}})
		f.want(r, 201, "create "+title)
		return r.body["id"].(string)
	}
	a, b := child("Plan", "2026-03-02", "2026-03-04"), child("Ship", "2026-03-09", "2026-03-10")
	f.want(post(admin, "/items/"+b+"/dependencies", map[string]any{"predecessor_id": a, "type": "FS"}), 201, "dependency")
	f.exec(`UPDATE items SET description='{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"plan text"}]}]}', description_text='plan text' WHERE id=$1`, a)

	path := "/spaces/" + space + "/blueprints"
	f.want(post(reader, path, map[string]any{"name": "Standard launch", "source_item_id": rootID}), 403, "read-only cannot save")
	f.want(post(outsider, path, map[string]any{"name": "Standard launch", "source_item_id": rootID}), 404, "outsider")
	f.want(post(editor, path, map[string]any{"name": " ", "source_item_id": rootID}), 422, "blank name")
	saved := post(editor, path, map[string]any{"name": "Standard launch", "source_item_id": rootID})
	f.want(saved, 201, "save")
	if saved.body["item_count"] != float64(3) || saved.body["item_type"] != "project" {
		t.Fatalf("saved summary: %v", saved.body)
	}
	f.want(post(editor, path, map[string]any{"name": "standard LAUNCH", "source_item_id": rootID}), 422, "duplicate name")
	f.want(f.call(outsider, "GET", path, nil), 404, "outsider list")
	if got := f.call(reader, "GET", path, nil); got.status != 200 || len(got.items()) != 1 {
		t.Fatalf("reader lists: %+v", got)
	}
	id := saved.body["id"].(string)

	// Use it in another Space, anchored on a new date; the source must not be touched or shared.
	use := post(editor, "/blueprints/"+id+"/items", map[string]any{"title": "Launch Q2", "start_date": "2026-06-01",
		"locations": []map[string]any{{"space_id": other}}})
	f.want(use, 201, "use")
	newRoot := use.body["id"].(string)
	if use.body["title"] != "Launch Q2" || use.body["importance"] != "high" {
		t.Fatalf("root: %v", use.body)
	}
	row := func(q string, args ...any) string { return f.queryID(q, args...) }
	if n := row("SELECT count(*)::text FROM item_locations WHERE parent_item_id=$1", newRoot); n != "2" {
		t.Fatalf("children = %s", n)
	}
	if got := row("SELECT start_date::text||' '||due_date::text FROM items WHERE title='Ship' AND created_by=$1", editor.id); got != "2026-06-08 2026-06-09" {
		t.Fatalf("relative dates = %s", got)
	}
	if n := row(`SELECT count(*)::text FROM item_dependencies d JOIN items p ON p.id=d.predecessor_id JOIN items s ON s.id=d.successor_id
 WHERE p.title='Plan' AND s.title='Ship' AND p.created_by=$1`, editor.id); n != "1" {
		t.Fatalf("copied dependency = %s", n)
	}
	if got := row("SELECT description_text FROM items WHERE title='Plan' AND created_by=$1", editor.id); got != "plan text" {
		t.Fatalf("description = %q", got)
	}
	if n := row("SELECT count(*)::text FROM item_assignees WHERE item_id=$1", newRoot); n != "1" {
		t.Fatalf("assignees = %s", n)
	}
	// No shared data: editing the copy leaves the source and blueprint unchanged.
	f.want(f.call(editor, "PATCH", "/items/"+newRoot, map[string]any{"title": "Renamed"}, "If-Match", `"`+"1"+`"`), 200, "edit copy")
	if got := row("SELECT title FROM items WHERE id=$1", rootID); got != "Launch" {
		t.Fatalf("source changed: %s", got)
	}
	if n := row("SELECT count(*)::text FROM items WHERE title='Plan'"); n != "2" {
		t.Fatalf("copies of Plan = %s", n)
	}

	// Without a start date nothing is dated; read-only users cannot create; unknown ids are 404.
	undated := post(editor, "/blueprints/"+id+"/items", map[string]any{"locations": []map[string]any{{"space_id": other}}})
	f.want(undated, 201, "use without date")
	if n := row("SELECT count(*)::text FROM items WHERE start_date IS NULL AND title='Ship'"); n != "1" {
		t.Fatalf("undated Ship = %s", n)
	}
	f.want(post(reader, "/blueprints/"+id+"/items", map[string]any{"locations": []map[string]any{{"space_id": space}}}), 403, "read-only cannot create")
	f.want(post(outsider, "/blueprints/"+id+"/items", map[string]any{"locations": []map[string]any{{"space_id": other}}}), 404, "outsider use")
	// One failing node rolls the whole tree back.
	before := row("SELECT count(*)::text FROM items")
	f.want(post(editor, "/blueprints/"+id+"/items", map[string]any{"locations": []map[string]any{{"space_id": space, "parent_item_id": a}}}), 422, "project under a task")
	if after := row("SELECT count(*)::text FROM items"); after != before {
		t.Fatalf("partial tree left behind: %s -> %s", before, after)
	}

	f.want(f.call(reader, "DELETE", "/blueprints/"+id, nil), 403, "reader delete")
	f.want(f.call(editor, "DELETE", "/blueprints/"+id, nil), 204, "delete")
	f.want(f.call(editor, "DELETE", "/blueprints/"+id, nil), 404, "deleted twice")
}
