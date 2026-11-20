package httpserver_test

import (
	"net/http"
	"testing"
)

// Field-level access (CF-D03): a field with no grants inherits item access; a restricted field is hidden from people without a
// grant and fails closed. Reads, writes, sort and filter, formulas that read it, and the grant API are all covered.
func TestCustomFieldAccessGrants(t *testing.T) {
	f := newFixture(t)
	admin, owner, editor, viewer, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Payroll space")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	f.share(space, "user", owner.id, "full")
	f.share(space, "user", editor.id, "editor")
	f.share(space, "user", viewer.id, "editor") // item edit allowed, so a refusal can only come from the field grant
	const group = "77777777-7777-4777-8777-777777777777"
	f.exec("INSERT INTO groups(id,name) VALUES($1,'Payroll team')", group)
	f.exec("INSERT INTO group_members(group_id,user_id) VALUES($1,$2)", group, viewer.id)

	salary := f.call(owner, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{"name": "Salary", "type": "number"})
	f.want(salary, 201, "Salary field")
	sid := salary.body["id"].(string)
	bonus := f.call(owner, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{"name": "Bonus", "type": "formula", "config": map[string]any{"expression": "[Salary]*2"}})
	f.want(bonus, 201, "formula Bonus reads Salary")
	bid := bonus.body["id"].(string)

	typ := f.call(admin, http.MethodPost, "/item-types", map[string]any{"key": "pay_req", "name": "Pay request", "base": "task", "space_id": space})
	f.want(typ, 201, "Space item type")
	tid := typ.body["id"].(string)
	f.want(f.call(admin, http.MethodPut, "/item-types/"+tid+"/fields/"+sid, map[string]any{}), 200, "type takes Salary")
	f.want(f.call(admin, http.MethodPut, "/item-types/"+tid+"/fields/"+bid, map[string]any{}), 200, "type takes Bonus")

	item := f.call(editor, http.MethodPost, "/items", map[string]any{"type_key": "pay_req", "title": "Offer", "locations": []map[string]any{{"space_id": space}}})
	f.want(item, 201, "item in the Space")
	itemID := item.body["id"].(string)
	f.exec("UPDATE items SET fields=jsonb_build_object($2::text,5000,$3::text,10000) WHERE id=$1", itemID, sid, bid)

	// Inherit: no grants, so the editor sees and writes the value as before.
	before := f.call(editor, http.MethodGet, "/items/"+itemID, nil)
	if values, _ := before.body["fields"].(map[string]any); values[sid] != float64(5000) {
		t.Fatalf("an unrestricted field inherits item access: %v", before.body["fields"])
	}

	// Restrict Salary to the owner. Bonus reads Salary, so it is hidden from the editor too.
	f.want(f.call(owner, http.MethodPut, "/spaces/"+space+"/custom-fields/"+sid+"/grants",
		map[string]any{"grants": []map[string]any{{"subject_kind": "user", "subject_id": owner.id, "level": "edit"}}}), 200, "owner restricts Salary")

	hidden := f.call(editor, http.MethodGet, "/items/"+itemID, nil)
	values, _ := hidden.body["fields"].(map[string]any)
	if _, ok := values[sid]; ok {
		t.Fatalf("a restricted field is hidden from people without a grant: %v", hidden.body["fields"])
	}
	if _, ok := values[bid]; ok {
		t.Fatalf("a formula that reads a hidden field is hidden too: %v", hidden.body["fields"])
	}
	f.want(f.call(editor, http.MethodPatch, "/items/"+itemID, map[string]any{"fields": map[string]any{sid: 1}}, "If-Match", `"1"`), 422, "no grant: write refused like an unknown field")
	f.want(f.call(editor, http.MethodGet, "/spaces/"+space+"/items?sort=field:"+sid, nil), 422, "no grant: sort refused like an unknown field")
	f.want(f.call(editor, http.MethodGet, "/spaces/"+space+"/items?field="+sid+":5000", nil), 422, "no grant: filter refused like an unknown field")

	owned := f.call(owner, http.MethodGet, "/items/"+itemID, nil)
	if values, _ := owned.body["fields"].(map[string]any); values[sid] != float64(5000) || values[bid] != float64(10000) {
		t.Fatalf("the grant holder reads the value and the formula: %v", owned.body["fields"])
	}

	// A group grants view; viewer reads through the group but cannot write.
	f.want(f.call(owner, http.MethodPut, "/spaces/"+space+"/custom-fields/"+sid+"/grants",
		map[string]any{"grants": []map[string]any{
			{"subject_kind": "user", "subject_id": owner.id, "level": "edit"},
			{"subject_kind": "group", "subject_id": group, "level": "view"},
		}}), 200, "group view grant")
	viaGroup := f.call(viewer, http.MethodGet, "/items/"+itemID, nil)
	if values, _ := viaGroup.body["fields"].(map[string]any); values[sid] != float64(5000) {
		t.Fatalf("a group view grant reads the value: %v", viaGroup.body["fields"])
	}
	f.want(f.call(viewer, http.MethodPatch, "/items/"+itemID, map[string]any{"fields": map[string]any{sid: 1}}, "If-Match", `"1"`), 422, "view grant cannot write")

	// A personal "none" beats the group's view.
	f.want(f.call(owner, http.MethodPut, "/spaces/"+space+"/custom-fields/"+sid+"/grants",
		map[string]any{"grants": []map[string]any{
			{"subject_kind": "user", "subject_id": owner.id, "level": "edit"},
			{"subject_kind": "group", "subject_id": group, "level": "view"},
			{"subject_kind": "user", "subject_id": viewer.id, "level": "none"},
		}}), 200, "explicit none")
	denied := f.call(viewer, http.MethodGet, "/items/"+itemID, nil)
	if values, _ := denied.body["fields"].(map[string]any); values[sid] != nil {
		t.Fatalf("a none grant overrides the group's view: %v", denied.body["fields"])
	}

	// Grant API: only managers read or change grants; the list is validated.
	f.want(f.call(editor, http.MethodGet, "/spaces/"+space+"/custom-fields/"+sid+"/grants", nil), 403, "editor cannot read grants")
	f.want(f.call(outsider, http.MethodGet, "/spaces/"+space+"/custom-fields/"+sid+"/grants", nil), 404, "outsider sees nothing")
	f.want(f.call(editor, http.MethodPut, "/spaces/"+space+"/custom-fields/"+sid+"/grants", map[string]any{"grants": []map[string]any{}}), 403, "editor cannot change grants")
	f.want(f.call(owner, http.MethodPut, "/spaces/"+space+"/custom-fields/"+sid+"/grants",
		map[string]any{"grants": []map[string]any{{"subject_kind": "user", "subject_id": outsider.id, "level": "bogus"}}}), 422, "bad level")
	f.want(f.call(owner, http.MethodPut, "/spaces/"+space+"/custom-fields/"+sid+"/grants",
		map[string]any{"grants": []map[string]any{{"subject_kind": "user", "subject_id": owner.id, "level": "edit"}, {"subject_kind": "user", "subject_id": owner.id, "level": "view"}}}), 422, "a subject once")
	f.want(f.call(owner, http.MethodPut, "/spaces/"+space+"/custom-fields/"+sid+"/grants",
		map[string]any{"grants": []map[string]any{{"subject_kind": "user", "subject_id": "88888888-8888-4888-8888-888888888888", "level": "view"}}}), 422, "unknown user")

	// Analytics: a restricted field cannot be summed or grouped by a person without a grant, and the total stays hidden too.
	sum := func(p *person) reply {
		return f.call(p, "POST", "/analytics/query", map[string]any{"reporting_on": "tasks", "metric": "sum_field", "field_id": sid, "space_ids": []string{space}})
	}
	f.want(sum(editor), 422, "analytics: no grant is treated as unknown")
	f.want(sum(owner), 200, "analytics: grant holder can sum the field")

	// Removing every grant lifts the restriction.
	f.want(f.call(owner, http.MethodPut, "/spaces/"+space+"/custom-fields/"+sid+"/grants", map[string]any{"grants": []map[string]any{}}), 200, "empty list lifts the restriction")
	lifted := f.call(editor, http.MethodGet, "/items/"+itemID, nil)
	if values, _ := lifted.body["fields"].(map[string]any); values[sid] != float64(5000) {
		t.Fatalf("with no grants the field inherits item access again: %v", lifted.body["fields"])
	}
}
