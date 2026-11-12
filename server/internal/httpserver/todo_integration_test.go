package httpserver_test

import (
	"net/url"
	"slices"
	"testing"
)

func TestMyTodoOnlyReturnsVisibleAssignedTasks(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	visible, hidden := f.space("Visible to-do"), f.space("Hidden to-do")
	f.share(visible, "user", member.id, "read")
	active := f.queryID("SELECT id::text FROM statuses WHERE name='New'")
	deferred := f.queryID("SELECT id::text FROM statuses WHERE name='On Hold'")
	completed := f.queryID("SELECT id::text FROM statuses WHERE name='Completed'")
	add := func(space, title, kind string, status, due any, assigned bool) string {
		id := f.queryID(`INSERT INTO items(type,title,status_id,due_date,created_by)
VALUES ($1::item_type,$2,$3::uuid,current_date + $4::int,$5) RETURNING id::text`, kind, title, status, due, admin.id)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", id, space)
		if assigned {
			f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", id, member.id)
		}
		return id
	}

	late := add(visible, "Beta", "task", active, -7, true)
	hold := add(visible, "Gamma", "task", deferred, -3, true)
	done := add(visible, "Delta", "task", completed, -6, true)
	undated := add(visible, "Alpha", "task", active, nil, true)
	add(visible, "Unassigned", "task", active, -10, false)
	add(visible, "Not a task", "project", nil, nil, true)
	add(hidden, "Hidden", "task", active, -9, true)
	deleted := add(visible, "Deleted", "task", active, -10, true)
	f.want(f.call(admin, "DELETE", "/items/"+deleted, nil), 204, "soft-delete item")

	assertIDs := func(path string, expected ...string) reply {
		t.Helper()
		result := f.call(member, "GET", "/me/todo"+path, nil)
		f.want(result, 200, "my to-do")
		var ids []string
		for _, item := range result.items() {
			ids = append(ids, item["id"].(string))
		}
		if !slices.Equal(ids, expected) {
			t.Fatalf("query %q: got %v, want %v", path, ids, expected)
		}
		return result
	}

	assertIDs("", late, undated)
	assertIDs("?filter=overdue", late, hold)
	assertIDs("?filter=all&sort=title", undated, late, done, hold)
	assertIDs("?filter=all&sort=date_created", undated, done, hold, late)
	page := assertIDs("?filter=all&sort=date&limit=2", late, done)
	cursor, ok := page.body["next_cursor"].(string)
	if !ok {
		t.Fatalf("expected next cursor: %+v", page)
	}
	assertIDs("?filter=all&sort=date&limit=2&cursor="+url.QueryEscape(cursor), hold, undated)
	f.want(f.call(member, "GET", "/me/todo?filter=active&limit=2&cursor="+url.QueryEscape(cursor), nil), 422, "cursor scope")
	f.want(f.call(member, "GET", "/me/todo?sort=unknown", nil), 422, "invalid sort")
}
