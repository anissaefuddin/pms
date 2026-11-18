package httpserver_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// AUT-A0 / AUT-T14: Try it reads a restricted custom field the way a real run does. A tester without the grant sees the field
// as absent, so the answer for a restricted field is the same as for a field that does not exist, and the value cannot be inferred.
func TestDryRunHidesRestrictedFieldValues(t *testing.T) {
	f := newFixture(t)
	admin, owner, tester := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Try it space")
	f.share(space, "user", owner.id, "full")
	f.share(space, "user", tester.id, "full") // Edit on the Space lets the tester open Try it; the field grant is what differs

	salary := f.call(owner, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{"name": "Salary", "type": "number"})
	f.want(salary, 201, "Salary field")
	sid := salary.body["id"].(string)
	f.want(f.call(owner, http.MethodPut, "/spaces/"+space+"/custom-fields/"+sid+"/grants",
		map[string]any{"grants": []map[string]any{{"subject_kind": "user", "subject_id": owner.id, "level": "edit"}}}), 200, "restrict Salary to the owner")

	typ := f.call(admin, http.MethodPost, "/item-types", map[string]any{"key": "offer", "name": "Offer", "base": "task", "space_id": space})
	f.want(typ, 201, "Space item type")
	f.want(f.call(admin, http.MethodPut, "/item-types/"+typ.body["id"].(string)+"/fields/"+sid, map[string]any{}), 200, "type takes Salary")
	item := f.call(owner, http.MethodPost, "/items", map[string]any{"type_key": "offer", "title": "Offer A", "locations": []map[string]any{{"space_id": space}}})
	f.want(item, 201, "item")
	itemID := item.body["id"].(string)
	f.exec("UPDATE items SET fields=jsonb_build_object($2::text,5000) WHERE id=$1", itemID, sid)

	// holds runs one condition through Try it and returns whether it holds.
	holds := func(p *person, field, op string, value any) bool {
		cond := map[string]any{"field": field, "op": op}
		if value != nil {
			cond["value"] = value
		}
		def, _ := json.Marshal(map[string]any{"when": map[string]any{"type": "item.created"}, "if": []any{cond}, "then": []any{map[string]any{"type": "comment", "text": "x"}}})
		got := f.call(p, http.MethodPost, "/spaces/"+space+"/automation-rules/dry-run", map[string]any{"definition": json.RawMessage(def), "item_id": itemID})
		f.want(got, 200, "dry run "+op)
		return got.body["conditions"].([]any)[0].(map[string]any)["holds"].(bool)
	}
	restricted := "fields." + sid
	missing := "fields." + uuid.NewString() // a field that does not exist, for comparison

	// The grant holder reads the value, exactly as a real run would.
	if !holds(owner, restricted, "eq", 5000) || !holds(owner, restricted, "gt", 4000) || holds(owner, restricted, "lt", 4000) {
		t.Fatal("the grant holder must see the real value in Try it")
	}

	// Without the grant: every answer matches the answer for a missing field, so eq, gt, lt cannot tell the value apart.
	for _, c := range []struct {
		op    string
		value any
	}{{"eq", 5000}, {"eq", 1}, {"neq", 5000}, {"gt", 4000}, {"gt", 9000}, {"lt", 9000}, {"lt", 1}} {
		got := holds(tester, restricted, c.op, c.value)
		if got != holds(tester, missing, c.op, c.value) {
			t.Fatalf("%s %v on a restricted field differs from a missing field: the value leaks", c.op, c.value)
		}
		if got && c.op == "eq" && c.value == 5000 {
			t.Fatal("eq 5000 holds for a tester without the grant")
		}
	}
	if holds(tester, restricted, "set", nil) || !holds(tester, restricted, "unset", nil) {
		t.Fatal("a restricted field must read as unset to a tester without the grant")
	}
	if holds(tester, restricted, "set", nil) != holds(tester, missing, "set", nil) {
		t.Fatal("set on a restricted field must match a missing field")
	}

	// Try it stays read-only.
	if f.queryID("SELECT count(*)::text FROM automation_runs") != "0" || f.queryID("SELECT fields->>$2 FROM items WHERE id=$1", itemID, sid) != "5000" {
		t.Fatal("Try it must change nothing")
	}
}
