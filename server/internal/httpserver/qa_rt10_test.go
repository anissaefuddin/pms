package httpserver_test

import (
	"net/http"
	"testing"
)

// RT-10: a task with subitems must report child_count, like a project does.
func TestRT10TaskChildCount(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("RT10")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','P',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, space)
	task := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','T',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'U')", task, space, project)
	child := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','C',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'U')", child, space, task)
	r := f.call(admin, http.MethodGet, "/items/"+task, nil)
	if r.status != 200 || r.body["child_count"] != float64(1) {
		t.Fatalf("detail child_count: %d %v", r.status, r.body)
	}
	l := f.call(admin, http.MethodGet, "/items/"+project+"/children", nil)
	if l.status != 200 || len(l.items()) != 1 || l.items()[0]["child_count"] != float64(1) {
		t.Fatalf("list child_count: %d %v", l.status, l.body)
	}
}
