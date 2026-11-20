package httpserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func dependencyPath(id string) string { return "/items/" + id + "/dependencies" }

func dependencyInput(id, kind string) map[string]any {
	return map[string]any{"predecessor_id": id, "type": kind}
}

func dependencyItem(f *fixture, actor *person, space, title string) string {
	f.t.Helper()
	id := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task',$1,$2) RETURNING id::text", title, actor.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
	return id
}

func TestDependencyLifecycle(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Dependencies")
	f.share(space, "user", admin.id, "full") // A1.2: the schedule owner is an explicit member
	a := dependencyItem(f, admin, space, "A")
	b := dependencyItem(f, admin, space, "B")
	c := dependencyItem(f, admin, space, "C")
	d := dependencyItem(f, admin, space, "D")

	makeEdge := func(from, to, kind string, lag int) string {
		t.Helper()
		input := dependencyInput(from, kind)
		input["lag"] = lag
		got := f.call(admin, "POST", dependencyPath(to), input)
		if got.status != 201 || got.body["type"] != kind || got.body["lag"] != float64(lag) ||
			got.body["predecessor_id"] != from || got.body["successor_id"] != to {
			t.Fatalf("create %s: %+v", kind, got)
		}
		return got.body["id"].(string)
	}
	ab := makeEdge(a, b, "FS", -2)
	makeEdge(b, c, "SS", 0)
	makeEdge(c, d, "FF", 2)
	makeEdge(a, d, "SF", 0)
	if got := f.call(admin, "GET", dependencyPath(b), nil); got.status != 200 || len(got.items()) != 2 {
		t.Fatalf("incoming and outgoing dependencies: %+v", got)
	}
	if got := f.call(admin, "POST", dependencyPath(b), dependencyInput(a, "FF")); got.status != 409 || got.body["code"] != "dependency_exists" {
		t.Fatalf("duplicate pair: %+v", got)
	}
	for _, edge := range [][2]string{{a, a}, {b, a}, {c, a}} {
		if got := f.call(admin, "POST", dependencyPath(edge[1]), dependencyInput(edge[0], "FS")); got.status != 422 {
			t.Fatalf("cyclic edge %v: %+v", edge, got)
		}
	}
	if got := f.call(admin, "POST", dependencyPath(a), dependencyInput(c, "unknown")); got.status != 422 {
		t.Fatalf("invalid type: %+v", got)
	}
	if got := f.call(admin, "DELETE", "/items/"+b, nil); got.status != 204 {
		t.Fatalf("soft delete: %+v", got)
	}
	if got := f.call(admin, "GET", dependencyPath(c), nil); got.status != 200 || len(got.items()) != 1 {
		t.Fatalf("edge with deleted predecessor still listed: %+v", got)
	}
	if got := f.call(admin, "GET", dependencyPath(a), nil); got.status != 200 || len(got.items()) != 1 {
		t.Fatalf("edge with deleted successor still listed: %+v", got)
	}
	if got := f.call(admin, "DELETE", "/dependencies/"+ab, nil); got.status != 404 {
		t.Fatalf("delete dormant dependency: %+v", got)
	}
	if got := f.call(admin, "POST", dependencyPath(a), dependencyInput(c, "FS")); got.status != 422 {
		t.Fatalf("new edge would produce cycle after restore: %+v", got)
	}
	if got := f.call(admin, "POST", "/recycle-bin/"+b+"/restore", nil); got.status != 200 {
		t.Fatalf("restore: %+v", got)
	}
	if got := f.call(admin, "GET", dependencyPath(b), nil); got.status != 200 || len(got.items()) != 2 {
		t.Fatalf("restored dependency list: %+v", got)
	}
	if got := f.call(admin, "DELETE", "/dependencies/"+ab, nil); got.status != 204 {
		t.Fatalf("remove: %+v", got)
	}
	if got := f.call(admin, "POST", dependencyPath(a), dependencyInput(c, "FS")); got.status != 201 || got.body["lag"] != float64(0) {
		t.Fatalf("newly valid edge with default lag: %+v", got)
	}
}

func TestDependencyAccess(t *testing.T) {
	f := newFixture(t)
	admin, editor, reader := f.person("admin"), f.person("regular"), f.person("regular")
	first, second := f.space("Predecessor"), f.space("Successor")
	f.share(first, "user", admin.id, "full")
	f.share(second, "user", admin.id, "full") // A1.2: the planner owns both ends of the link
	f.share(second, "user", editor.id, "editor")
	f.share(first, "user", reader.id, "read")
	f.share(second, "user", reader.id, "read")
	a := dependencyItem(f, admin, first, "A")
	b := dependencyItem(f, admin, second, "B")
	body := dependencyInput(a, "FS")

	if got := f.call(editor, "POST", dependencyPath(b), body); got.status != 404 {
		t.Fatalf("hidden predecessor: %+v", got)
	}
	if got := f.call(editor, "GET", dependencyPath(a), nil); got.status != 404 {
		t.Fatalf("hidden anchor: %+v", got)
	}
	if got := f.call(editor, "POST", dependencyPath(a), dependencyInput(b, "FS")); got.status != 404 {
		t.Fatalf("hidden successor: %+v", got)
	}
	if got := f.call(reader, "POST", dependencyPath(b), body); got.status != 403 {
		t.Fatalf("reader create: %+v", got)
	}
	created := f.call(admin, "POST", dependencyPath(b), body)
	if created.status != 201 {
		t.Fatalf("create: %+v", created)
	}
	path := "/dependencies/" + created.body["id"].(string)
	if got := f.call(editor, "GET", dependencyPath(b), nil); got.status != 200 || len(got.items()) != 0 {
		t.Fatalf("hidden counterpart listed: %+v", got)
	}
	if got := f.call(editor, "DELETE", path, nil); got.status != 404 {
		t.Fatalf("delete hidden dependency: %+v", got)
	}
	if got := f.call(reader, "GET", dependencyPath(b), nil); got.status != 200 || len(got.items()) != 1 {
		t.Fatalf("reader list: %+v", got)
	}
	if got := f.call(reader, "DELETE", path, nil); got.status != 403 {
		t.Fatalf("reader delete: %+v", got)
	}
	f.share(first, "user", editor.id, "read")
	if got := f.call(editor, "DELETE", path, nil); got.status != 204 {
		t.Fatalf("edit successor and view predecessor permits delete: %+v", got)
	}
	if got := f.call(editor, "POST", dependencyPath(b), body); got.status != 201 {
		t.Fatalf("edit successor and view predecessor permits create: %+v", got)
	}
	if got := f.call(editor, "POST", dependencyPath(a), dependencyInput(b, "FS")); got.status != 403 {
		t.Fatalf("editing the predecessor cannot substitute for editing the successor: %+v", got)
	}
}

func TestConcurrentDependenciesCannotCreateCycle(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Concurrent dependencies")
	f.share(space, "user", admin.id, "full") // A1.2: the schedule owner is an explicit member
	a := dependencyItem(f, admin, space, "A")
	b := dependencyItem(f, admin, space, "B")

	type result struct {
		status int
		err    error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for _, edge := range [][2]string{{a, b}, {b, a}} {
		go func() {
			<-start
			body, _ := json.Marshal(dependencyInput(edge[0], "FS"))
			req, err := http.NewRequestWithContext(t.Context(), "POST", f.srv.URL+"/api/v1"+dependencyPath(edge[1]), bytes.NewReader(body))
			if err != nil {
				results <- result{err: err}
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", publicURL)
			req.AddCookie(admin.cookie)
			res, err := f.srv.Client().Do(req)
			if err != nil {
				results <- result{err: err}
				return
			}
			res.Body.Close()
			results <- result{status: res.StatusCode}
		}()
	}
	close(start)
	created := 0
	for range 2 {
		res := <-results
		if res.err != nil || (res.status != 201 && res.status != 409 && res.status != 422) {
			t.Errorf("unexpected concurrent response: %+v", res)
		}
		if res.status == 201 {
			created++
		}
	}
	if created != 1 || f.queryID("SELECT count(*)::text FROM item_dependencies") != "1" {
		t.Fatalf("expected one committed edge, got %d successful requests", created)
	}
}

func TestQueryDependenciesAmongItems(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Gantt links")
	f.share(space, "user", admin.id, "full") // A1.2: the schedule owner is an explicit member
	a, b, c := dependencyItem(f, admin, space, "A"), dependencyItem(f, admin, space, "B"), dependencyItem(f, admin, space, "C")
	f.want(f.call(admin, "POST", dependencyPath(b), dependencyInput(a, "FS")), 201, "a to b")
	f.want(f.call(admin, "POST", dependencyPath(c), dependencyInput(b, "SS")), 201, "b to c")
	got := f.call(admin, "POST", "/dependencies/query", map[string]any{"item_ids": []string{a, b}})
	f.want(got, 200, "query")
	if len(got.items()) != 1 {
		t.Fatalf("only a->b has both ends in the list: %+v", got)
	}
	if got = f.call(admin, "POST", "/dependencies/query", map[string]any{"item_ids": []string{}}); got.status != 200 || len(got.items()) != 0 {
		t.Fatalf("empty list: %+v", got)
	}
	f.want(f.call(admin, "POST", "/dependencies/query", map[string]any{"item_ids": []string{"nope"}}), 422, "bad id")
	stranger := f.person("regular")
	if got = f.call(stranger, "POST", "/dependencies/query", map[string]any{"item_ids": []string{a, b, c}}); got.status != 200 || len(got.items()) != 0 {
		t.Fatalf("a user who cannot see the items learns nothing: %+v", got)
	}
}
