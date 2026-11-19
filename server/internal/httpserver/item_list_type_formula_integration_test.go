package httpserver_test

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

// S8.1.5: the item list filters by type key (built-in or custom) and refuses unknown keys without a 5xx.
// S8.2.1: a formula field can be filtered and sorted like the other numeric fields.
func TestListFiltersByTypeKeyAndFormulaField(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Type list")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	f.want(f.call(admin, http.MethodPost, "/item-types", map[string]any{"key": "bug", "name": "Bug", "base": "task"}), http.StatusCreated, "type")
	loc := []map[string]any{{"space_id": space}}
	mk := func(title, key string, fields map[string]any) {
		body := map[string]any{"type": "task", "title": title, "locations": loc}
		if key != "" {
			body["type_key"] = key
		}
		res := f.call(admin, http.MethodPost, "/items", body)
		f.want(res, http.StatusCreated, title)
		if len(fields) > 0 {
			f.want(f.call(admin, http.MethodPatch, "/items/"+res.body["id"].(string), map[string]any{"fields": fields}, "If-Match", `"1"`), http.StatusOK, "fields "+title)
		}
	}
	field := func(name, kind string, config map[string]any) string {
		res := f.call(admin, http.MethodPost, customFieldsPath(space), map[string]any{"name": name, "type": kind, "config": config})
		f.want(res, http.StatusCreated, name)
		return res.body["id"].(string)
	}
	price := field("Price", "number", nil)
	total := field("Total", "formula", map[string]any{"expression": "[Price]*2"})
	mk("plain", "", map[string]any{price: 5})
	mk("crash", "bug", map[string]any{price: 1})
	mk("empty", "bug", nil)
	titles := func(query string, status int) []string {
		res := f.call(admin, http.MethodGet, "/spaces/"+space+"/items?"+query, nil)
		f.want(res, status, query)
		out := []string{}
		if status == http.StatusOK {
			for _, it := range res.body["items"].([]any) {
				out = append(out, it.(map[string]any)["title"].(string))
			}
		}
		return out
	}
	q := func(k, v string) string { return k + "=" + url.QueryEscape(v) }
	for query, want := range map[string][]string{
		q("type", "bug") + "&" + q("sort", "title"):      {"crash", "empty"},
		q("type", "task") + "&" + q("sort", "title"):     {"plain"},
		q("type", "bug,task") + "&" + q("sort", "title"): {"crash", "empty", "plain"},
		q("sort", "field:"+total):                        {"crash", "plain", "empty"}, // 2, 10, no result last
		q("field", total+":10"):                          {"plain"},
		q("type", "bug") + "&" + q("field", total+":2"):  {"crash"},
	} {
		if got := titles(query, http.StatusOK); !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %v, want %v", query, got, want)
		}
	}
	for _, key := range []string{"nope", "bug,nope", "'; DROP TABLE items;--"} {
		titles(q("type", key), http.StatusUnprocessableEntity)
	}
}
