package httpserver_test

import (
	"net/http"
	"testing"
)

// AUT-T20: a rule that creates items of a custom item type is invalid while that type is inactive, says why, and runs again once
// the type is active and the rule is resumed.
func TestRuleIsInvalidWhileItsItemTypeIsInactive(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	typ := f.call(e.admin, http.MethodPost, "/item-types", map[string]any{"key": "offer", "name": "Offer", "base": "task", "space_id": e.space})
	f.want(typ, 201, "Space item type")
	rule := e.rule(t, e.editor, "Make offers", `{"when":{"type":"item.created"},"then":[{"type":"create_item","title":"Offer for {item.title}","type_key":"offer"}]}`)

	f.exec("UPDATE item_type_defs SET active=false WHERE key='offer'")
	e.item(t, e.admin, "While inactive", nil)
	e.drain(t)
	got := f.call(e.admin, "GET", "/automation-rules/"+rule, nil)
	if got.body["health"] != "invalid" || got.body["health_reason"] != "item_type_missing" {
		t.Fatalf("an inactive item type makes the rule invalid: %+v", got.body)
	}
	if e.runs(rule)["done"] != 0 {
		t.Fatal("an invalid rule creates nothing")
	}
	f.want(f.call(e.admin, "PATCH", "/automation-rules/"+rule, map[string]any{"enabled": true}), 422, "resume refused while the type is inactive")

	f.exec("UPDATE item_type_defs SET active=true WHERE key='offer'")
	f.want(f.call(e.admin, "PATCH", "/automation-rules/"+rule, map[string]any{"enabled": true}), 200, "resume after the type is active")
	e.item(t, e.admin, "After", nil)
	e.drain(t)
	if f.queryID("SELECT count(*)::text FROM items WHERE title='Offer for After'") != "1" {
		t.Fatal("a resumed rule creates the item of the type")
	}
}
