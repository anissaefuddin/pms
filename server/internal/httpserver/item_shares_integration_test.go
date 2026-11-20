package httpserver_test

import "testing"

func TestItemSharesVersionedReplace(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	member := f.person("regular")
	space := f.space("Shared")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Plan',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	path := "/items/" + item + "/shares"
	first := f.call(admin, "GET", path, nil)
	if first.status != 200 || first.header.Get("ETag") != `"1"` {
		t.Fatalf("GET: %+v", first)
	}
	body := map[string]any{"shares": []map[string]string{{"principal_type": "user", "principal_id": member.id, "role": "read"}}}
	updated := f.call(admin, "PUT", path, body, "If-Match", first.header.Get("ETag"))
	if updated.status != 200 || len(updated.items()) != 1 || updated.header.Get("ETag") != `"2"` {
		t.Fatalf("PUT: %+v", updated)
	}
	if stale := f.call(admin, "PUT", path, body, "If-Match", first.header.Get("ETag")); stale.status != 409 {
		t.Fatalf("stale PUT: %+v", stale)
	}
	if detail := f.call(member, "GET", "/items/"+item, nil); detail.status != 200 {
		t.Fatalf("granted view: %+v", detail)
	}
}
