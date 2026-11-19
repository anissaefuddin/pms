package httpserver_test

import (
	"strconv"
	"testing"
	"time"
)

// TASK-11B end to end: a formula is validated when saved, is recalculated whenever an input changes, cannot be written
// directly, and its inputs cannot be renamed or archived from under it.
func TestFormulaFieldsAreValidatedRecalculatedAndProtected(t *testing.T) {
	f := newFixture(t)
	admin, editor := f.person("admin"), f.person("regular")
	space, elsewhere := f.space("Calc"), f.space("Elsewhere")
	f.share(space, "user", editor.id, "full")
	f.share(space, "user", admin.id, "full") // A1.2: the admin defines the formula fields in both Spaces
	f.share(elsewhere, "user", admin.id, "full")
	mk := func(sp, name, kind string, config map[string]any) string {
		body := map[string]any{"name": name, "type": kind}
		if config != nil {
			body["config"] = config
		}
		got := f.call(admin, "POST", "/spaces/"+sp+"/custom-fields", body)
		f.want(got, 201, "field "+name)
		return got.body["id"].(string)
	}
	price := mk(space, "Price", "number", nil)
	qty := mk(space, "Qty", "number", nil)
	note := mk(space, "Note", "text", nil)
	foreign := mk(elsewhere, "Elsewhere", "number", nil)
	post := func(name, expr string) reply {
		return f.call(admin, "POST", "/spaces/"+space+"/custom-fields", map[string]any{"name": name, "type": "formula", "config": map[string]any{"expression": expr}})
	}
	for name, expr := range map[string]string{
		"empty": "", "syntax": "[Price]*", "unknown word": "sqrt([Price])", "unknown field": "[Nope]+1", "text input": "[Note]+1",
		"other space": "[Elsewhere]+1", "self": "[Total]+1",
	} {
		if got := post("Total", expr); got.status != 422 {
			t.Fatalf("%s accepted: %+v", name, got)
		}
	}
	f.want(f.call(admin, "POST", "/spaces/"+space+"/custom-fields", map[string]any{"name": "NoExpr", "type": "formula"}), 422, "expression is required")
	f.want(f.call(admin, "POST", "/spaces/"+space+"/custom-fields", map[string]any{"name": "Misplaced", "type": "number", "config": map[string]any{"expression": "1+1"}}), 422, "expression only on formulas")
	total := post("Total", "[Price]*[Qty]")
	f.want(total, 201, "total")
	tid := total.body["id"].(string)
	double := post("Double", "[Total]*2")
	f.want(double, 201, "formula reading a formula")
	did := double.body["id"].(string)
	// Cycles are refused, whether direct or through another formula.
	cyc := f.call(admin, "PATCH", "/spaces/"+space+"/custom-fields/"+tid, map[string]any{"config": map[string]any{"expression": "[Double]+1"}})
	if cyc.status != 422 {
		t.Fatalf("cycle accepted: %+v", cyc)
	}
	f.want(f.call(admin, "PATCH", "/spaces/"+space+"/custom-fields/"+tid, map[string]any{"config": map[string]any{"expression": "[Total]"}}), 422, "self cycle on edit")

	item := f.call(editor, "POST", "/items", map[string]any{"type": "task", "title": "Order", "locations": []map[string]any{{"space_id": space}}})
	f.want(item, 201, "item")
	id, version := item.body["id"].(string), 1
	set := func(values map[string]any) reply {
		got := f.call(editor, "PATCH", "/items/"+id, map[string]any{"fields": values}, "If-Match", `"`+strconv.Itoa(version)+`"`)
		if got.status == 200 {
			version++
		}
		return got
	}
	value := func(field string) any {
		return f.call(editor, "GET", "/items/"+id, nil).body["fields"].(map[string]any)[field]
	}
	set(map[string]any{price: 12.5})
	if value(tid) != nil {
		t.Fatalf("a formula with a missing input has no value: %v", value(tid))
	}
	set(map[string]any{qty: 4})
	if value(tid) != float64(50) || value(did) != float64(100) {
		t.Fatalf("totals: %v %v", value(tid), value(did))
	}
	set(map[string]any{price: 20})
	if value(tid) != float64(80) || value(did) != float64(160) {
		t.Fatalf("recalculated totals: %v %v", value(tid), value(did))
	}
	set(map[string]any{qty: nil})
	if value(tid) != nil || value(did) != nil {
		t.Fatalf("clearing an input must clear the results: %v %v", value(tid), value(did))
	}
	// A formula cannot be written by hand, and an input of another Space cannot be written here either.
	f.want(set(map[string]any{tid: 5}), 422, "calculated fields are read-only")
	f.want(set(map[string]any{foreign: 5}), 422, "field of another Space")
	// Changing a formula recalculates every existing item at once.
	set(map[string]any{qty: 2})
	f.want(f.call(admin, "PATCH", "/spaces/"+space+"/custom-fields/"+tid, map[string]any{"config": map[string]any{"expression": "[Price]+[Qty]"}}), 200, "edit formula")
	if value(tid) != float64(22) || value(did) != float64(44) {
		t.Fatalf("existing items not recalculated after the formula changed: %v %v", value(tid), value(did))
	}
	// Inputs used by a formula cannot be renamed or archived; free ones can.
	f.want(f.call(admin, "PATCH", "/spaces/"+space+"/custom-fields/"+price, map[string]any{"name": "Cost"}), 422, "rename of an input")
	f.want(f.call(admin, "DELETE", "/spaces/"+space+"/custom-fields/"+qty, nil), 422, "archive of an input")
	f.want(f.call(admin, "PATCH", "/spaces/"+space+"/custom-fields/"+note, map[string]any{"name": "Memo"}), 200, "unused field renames")
	_ = time.Now
}

// S8.2.1 performance: saving a change on one item must not slow down with the number of items, and defining a formula
// over 500 items must finish promptly.
func TestFormulaRecalculationOver500Items(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Perf")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	price := f.call(admin, "POST", "/spaces/"+space+"/custom-fields", map[string]any{"name": "Price", "type": "number"})
	qty := f.call(admin, "POST", "/spaces/"+space+"/custom-fields", map[string]any{"name": "Qty", "type": "number"})
	f.want(price, 201, "price")
	f.want(qty, 201, "qty")
	for i := range 500 {
		id := f.queryID("INSERT INTO items(type,title,created_by,fields) VALUES ('task',$1,$2,jsonb_build_object($3::text,$4::int,$5::text,3)) RETURNING id::text", "Row "+strconv.Itoa(i), admin.id, price.body["id"], i, qty.body["id"])
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,$3)", id, space, "R"+strconv.Itoa(10000+i))
	}
	start := time.Now()
	total := f.call(admin, "POST", "/spaces/"+space+"/custom-fields", map[string]any{"name": "Total", "type": "formula", "config": map[string]any{"expression": "[Price]*[Qty]+if([Price]>250,1,0)"}})
	f.want(total, 201, "formula over 500 items")
	took := time.Since(start)
	if took > 3*time.Second {
		t.Fatalf("defining a formula over 500 items took %s", took)
	}
	if n := f.queryID("SELECT count(*)::text FROM items WHERE fields ? $1", total.body["id"]); n != "500" {
		t.Fatalf("formula values written for %s of 500 items", n)
	}
	if v := f.queryID("SELECT (fields->>$1)::text FROM items WHERE title='Row 400'", total.body["id"]); v != "1201" {
		t.Fatalf("Row 400 = %s, want 1201", v)
	}
	t.Logf("formula over 500 items: %s", took)
}
