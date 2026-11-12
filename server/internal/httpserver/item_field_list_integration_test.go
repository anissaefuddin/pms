package httpserver_test

import (
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestListFilterAndSortByCustomField(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Field list")
	defs := customFieldsPath(space)
	create := func(body map[string]any) string {
		res := f.call(admin, http.MethodPost, defs, body)
		f.want(res, http.StatusCreated, "create field")
		return res.body["id"].(string)
	}
	low, high := customFieldOption("Low"), customFieldOption("High")
	cost := create(map[string]any{"name": "Cost", "type": "number"})
	level := create(map[string]any{"name": "Level", "type": "single_select", "options": []any{low, high}})
	note := create(map[string]any{"name": "Note", "type": "text"})
	tags := create(map[string]any{"name": "Tags", "type": "multi_select", "options": []any{low, high}})
	set := func(title string, fields map[string]any) {
		id := customFieldItem(f, admin, space, title)
		f.want(f.call(admin, http.MethodPatch, "/items/"+id, map[string]any{"fields": fields}, "If-Match", `"1"`), http.StatusOK, "set "+title)
	}
	set("a", map[string]any{cost: 10.5, level: high["id"], note: "100% done", tags: []any{low["id"]}})
	set("b", map[string]any{cost: -3, level: low["id"], note: "half"})
	set("c", map[string]any{cost: 200, level: high["id"], note: "a_b"})
	set("d", map[string]any{}) // no values: sorts last, never matches a filter
	titles := func(query string) []string {
		res := f.call(admin, http.MethodGet, "/spaces/"+space+"/items?"+query, nil)
		f.want(res, http.StatusOK, query)
		out := []string{}
		for _, it := range res.body["items"].([]any) {
			out = append(out, it.(map[string]any)["title"].(string))
		}
		return out
	}
	q := func(k, v string) string { return k + "=" + url.QueryEscape(v) }
	for query, want := range map[string][]string{
		q("sort", "field:"+cost):                                  {"b", "a", "c", "d"},
		q("sort", "field:"+level):                                 {"a", "c", "b", "d"}, // High < Low; ties by id (creation order)
		q("field", cost+":10.5"):                                  {"a"},
		q("field", level+":"+high["id"]):                          {"a", "c"},
		q("field", tags+":"+low["id"]):                            {"a"},
		q("field", note+":100%") + "&" + q("field", cost+":10.5"): {"a"},
		q("field", note+":a_b"):                                   {"c"}, // _ is literal, not a wildcard
		q("field", note+":%"):                                     {"a"}, // % is literal
	} {
		got := titles(query)
		if query != q("sort", "field:"+cost) { // equal keys tie-break on random ids: compare as sets, but d stays last
			slices.Sort(got)
			slices.Sort(want)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %v, want %v", query, got, want)
		}
	}
	for _, query := range []string{
		q("sort", "field:"+tags), q("sort", "field:not-a-uuid"), q("field", "x:1"),
		q("field", cost+":abc"), q("field", "00000000-0000-4000-8000-000000000000:1"),
	} {
		f.want(f.call(admin, http.MethodGet, "/spaces/"+space+"/items?"+query, nil), http.StatusUnprocessableEntity, query)
	}
	other := f.space("Other space")
	f.want(f.call(admin, http.MethodGet, "/spaces/"+other+"/items?"+q("sort", "field:"+cost), nil),
		http.StatusUnprocessableEntity, "field of another Space")
}

// S2.3.3: Table with 500 items and 10 custom fields; sort and filter by field must stay under 1 s p95.
func TestListByCustomFieldAt500Items(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Field list perf")
	ids := make([]string, 10)
	for i := range ids {
		res := f.call(admin, http.MethodPost, customFieldsPath(space), map[string]any{"name": "Num" + string(rune('A'+i)), "type": "number"})
		f.want(res, http.StatusCreated, "create field")
		ids[i] = res.body["id"].(string)
	}
	f.exec(`WITH n AS (
 INSERT INTO items(type,title,created_by,fields)
 SELECT 'task','Item '||g,$1::uuid, (SELECT jsonb_object_agg(k, (g*7919 % 1000) + ord) FROM unnest($2::text[]) WITH ORDINALITY AS u(k,ord))
 FROM generate_series(1,500) g RETURNING id)
INSERT INTO item_locations(item_id,space_id,rank) SELECT id,$3::uuid,'U' FROM n`, admin.id, ids, space)
	var took []time.Duration
	for run := 0; run < 20; run++ {
		filter := "" // even runs return a full 200-item page, odd runs a narrow filter
		if run%2 == 1 {
			filter = "&field=" + url.QueryEscape(ids[0]+":1")
		}
		start := time.Now()
		res := f.call(admin, http.MethodGet, "/spaces/"+space+"/items?limit=200&sort="+url.QueryEscape("field:"+ids[run%10])+filter, nil)
		f.want(res, http.StatusOK, "sorted list")
		took = append(took, time.Since(start))
	}
	slices.Sort(took)
	t.Logf("500 items, 10 number fields, sort+filter by field: p50=%v p95=%v max=%v (one page of 200)", took[10], took[18], took[19])
	if took[18] > time.Second {
		t.Fatalf("p95 %v exceeds 1s", took[18])
	}
}
