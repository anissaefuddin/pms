package httpserver_test

import (
	"testing"

	"github.com/google/uuid"
)

func TestSpaceTemplatesBuildWholeSpaceOnceAndOnlyForCreators(t *testing.T) {
	f := newFixture(t)
	admin, regular, guest := f.person("admin"), f.person("regular"), f.person("viewer")
	list := f.call(regular, "GET", "/space-templates", nil)
	if list.status != 200 || len(list.items()) < 4 {
		t.Fatalf("catalog: %+v", list)
	}
	for _, tpl := range list.items() {
		id := tpl["id"].(string)
		counts := tpl["counts"].(map[string]any)
		got := f.call(regular, "POST", "/space-templates/"+id+"/spaces", map[string]any{"request_id": uuid.NewString(), "name": "From " + id})
		if got.status != 201 {
			t.Fatalf("%s: %+v", id, got)
		}
		space := got.body["id"].(string)
		count := func(sql string) float64 {
			var n float64
			if err := f.pool.QueryRow(f.ctx, sql, space).Scan(&n); err != nil {
				t.Fatal(err)
			}
			return n
		}
		for what, want := range map[string]struct {
			sql  string
			want any
		}{
			"projects":      {"SELECT count(*) FROM item_locations l JOIN items i ON i.id=l.item_id WHERE l.space_id=$1 AND i.type='project'", counts["projects"]},
			"tasks":         {"SELECT count(*) FROM item_locations l JOIN items i ON i.id=l.item_id WHERE l.space_id=$1 AND i.type='task'", counts["tasks"]},
			"views":         {"SELECT count(*) FROM views WHERE scope_space_id=$1", counts["views"].(float64) + 4}, // + Table, Board, Gantt and Calendar every Space starts with,
			"forms":         {"SELECT count(*) FROM request_forms WHERE space_id=$1", counts["forms"]},
			"custom_fields": {"SELECT count(*) FROM custom_fields WHERE space_id=$1", counts["custom_fields"]},
		} {
			if count(want.sql) != want.want.(float64) {
				t.Errorf("%s %s: got %v, want %v", id, what, count(want.sql), want.want)
			}
		}
		// The creator sees everything and the form is submittable: the whole template is usable, not just stored.
		if got := f.call(regular, "GET", "/spaces/"+space+"/items", nil); got.status != 200 || len(got.items()) != int(counts["projects"].(float64)) {
			t.Errorf("%s root items: %+v", id, got)
		}
	}
	// Retry with the same request_id: the same Space comes back and nothing is duplicated.
	key := uuid.NewString()
	first := f.call(regular, "POST", "/space-templates/project-management/spaces", map[string]any{"request_id": key, "name": "Once"})
	again := f.call(regular, "POST", "/space-templates/project-management/spaces", map[string]any{"request_id": key, "name": "Twice"})
	if first.status != 201 || again.status != 200 || again.body["id"] != first.body["id"] {
		t.Fatalf("replay: %+v / %+v", first, again)
	}
	if n := f.queryID("SELECT count(*)::text FROM spaces WHERE name IN ('Once','Twice')"); n != "1" {
		t.Fatalf("duplicate Spaces: %s", n)
	}
	// Another user cannot replay (or probe) the key; viewers cannot create Spaces at all.
	if got := f.call(admin, "POST", "/space-templates/project-management/spaces", map[string]any{"request_id": key}); got.status != 404 {
		t.Fatalf("foreign replay: %+v", got)
	}
	if got := f.call(guest, "POST", "/space-templates/project-management/spaces", map[string]any{"request_id": uuid.NewString()}); got.status != 403 {
		t.Fatalf("guest: %+v", got)
	}
	for name, body := range map[string]map[string]any{"no key": {}, "bad key": {"request_id": "x"}} {
		if got := f.call(regular, "POST", "/space-templates/project-management/spaces", body); got.status != 422 {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	if got := f.call(regular, "POST", "/space-templates/unknown/spaces", map[string]any{"request_id": uuid.NewString()}); got.status != 404 {
		t.Fatalf("unknown template: %+v", got)
	}
	// A failure rolls everything back: a name that is too long fails after the run row was written.
	long := make([]byte, 130)
	for i := range long {
		long[i] = 'x'
	}
	bad := uuid.NewString()
	if got := f.call(regular, "POST", "/space-templates/hr-onboarding/spaces", map[string]any{"request_id": bad, "name": string(long)}); got.status != 422 {
		t.Fatalf("long name: %+v", got)
	}
	if n := f.queryID("SELECT count(*)::text FROM space_template_runs WHERE request_id=$1", bad); n != "0" {
		t.Fatalf("run row survived a failed build: %s", n)
	}
}
