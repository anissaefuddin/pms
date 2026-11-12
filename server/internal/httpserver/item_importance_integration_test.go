package httpserver_test

import (
	"net/http"
	"reflect"
	"testing"
)

func TestImportanceAndPredecessorSort(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Priority")
	mk := func(title, importance, due string) string {
		id := customFieldItem(f, admin, space, title)
		body := map[string]any{"importance": importance}
		if due != "" {
			body["start_date"], body["due_date"] = due, due
		}
		f.want(f.call(admin, http.MethodPatch, "/items/"+id, body, "If-Match", `"1"`), http.StatusOK, "set "+title)
		return id
	}
	lowEarly := mk("low-early", "low", "2027-01-01")
	highLate := mk("high-late", "high", "2027-03-01")
	highEarly := mk("high-early", "high", "2027-02-01")
	normal := mk("normal", "normal", "")
	for _, bad := range []any{"urgent", "", 3, nil} {
		f.want(f.call(admin, http.MethodPatch, "/items/"+normal, map[string]any{"importance": bad}, "If-Match", `"2"`), http.StatusUnprocessableEntity, "bad importance")
	}
	if got := f.call(admin, http.MethodGet, "/items/"+normal, nil); got.body["importance"] != "normal" {
		t.Fatalf("importance = %v", got.body["importance"])
	}
	titles := func(path string) []string {
		res := f.call(admin, http.MethodGet, path, nil)
		f.want(res, http.StatusOK, path)
		out := []string{}
		for _, it := range res.body["items"].([]any) {
			out = append(out, it.(map[string]any)["title"].(string))
		}
		return out
	}
	want := []string{"high-early", "high-late", "normal", "low-early"}
	for _, key := range []string{"priority", "importance"} {
		if got := titles("/spaces/" + space + "/items?sort=" + key); !reflect.DeepEqual(got, want) {
			t.Errorf("sort=%s = %v, want %v", key, got, want)
		}
	}
	f.want(f.call(admin, http.MethodPost, dependencyPath(highEarly), dependencyInput(lowEarly, "FS")), http.StatusCreated, "dep 1")
	f.want(f.call(admin, http.MethodPost, dependencyPath(highEarly), dependencyInput(normal, "FS")), http.StatusCreated, "dep 2")
	f.want(f.call(admin, http.MethodPost, dependencyPath(highLate), dependencyInput(normal, "FS")), http.StatusCreated, "dep 3")
	got := titles("/spaces/" + space + "/items?sort=predecessors")
	if got[len(got)-1] != "high-early" || got[2] != "high-late" {
		t.Errorf("sort=predecessors = %v: want no-predecessor items first, high-early (2) last, high-late (1) before it", got)
	}
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2),($3,$2),($4,$2)", highLate, admin.id, lowEarly, normal)
	if got := titles("/me/todo?filter=all&sort=priority"); !reflect.DeepEqual(got, []string{"high-late", "normal", "low-early"}) {
		t.Errorf("my to-do by priority = %v", got)
	}
}
