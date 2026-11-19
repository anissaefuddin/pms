package httpserver_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestCustomFieldTypesAndConfig(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Typed custom fields")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	definitions := customFieldsPath(space)
	item := customFieldItem(f, admin, space, "Item with every type")
	path := "/items/" + item
	option := customFieldOption("High")
	fields := []struct {
		kind   string
		config map[string]any
		value  any
	}{
		{"text", nil, "See notes"},
		{"number", map[string]any{"aggregation": "average"}, -1.25},
		{"currency", map[string]any{
			"currency": "USD", "decimals": 3, "aggregation": "sum",
			"markers": []map[string]any{{"operator": "gte", "value": 1000, "color": "#ff2200"}},
		}, 1234.567},
		{"percent", map[string]any{"allow_over_100": true}, 120.5},
		{"single_select", nil, option["id"]},
		{"multi_select", nil, []any{option["id"]}},
		{"people", nil, []any{admin.id}},
		{"date", nil, "2028-02-29"},
		{"duration", map[string]any{"markers": []map[string]any{{"operator": "eq", "value": 2147483647, "color": "#ff2200"}}}, float64(2147483647)},
		{"checkbox", nil, false},
	}
	values, fieldIDs := map[string]any{}, map[string]string{}
	for _, field := range fields {
		create := map[string]any{"name": field.kind, "type": field.kind}
		if field.config != nil {
			create["config"] = field.config
		}
		if field.kind == "single_select" || field.kind == "multi_select" {
			create["options"] = []any{option}
		}
		res := f.call(admin, http.MethodPost, definitions, create)
		f.want(res, http.StatusCreated, "create "+field.kind)
		id := res.body["id"].(string)
		values[id], fieldIDs[field.kind] = field.value, id
		if _, ok := res.body["options"].([]any); !ok {
			t.Fatalf("definition must return an array of options: %v", res.body)
		}
	}
	for _, config := range []map[string]any{
		{"currency": "ZZZ"}, {"currency": "USD", "decimals": 7}, {"currency": "usd"},
		{"currency": "USD", "aggregation": "median"},
		{"currency": "USD", "markers": []map[string]any{{"operator": "gt", "color": "#ffffff"}}},
	} {
		f.want(f.call(admin, http.MethodPost, definitions, map[string]any{
			"name": "Invalid configuration", "type": "currency", "config": config,
		}), http.StatusUnprocessableEntity, "reject invalid currency config")
	}
	for _, body := range []map[string]any{
		{"type": "text", "name": "Options for scalar", "options": []any{option}},
		{"type": "text", "name": "Aggregate text", "config": map[string]string{"aggregation": "count"}},
		{"type": "percent", "name": "Wrong config", "config": map[string]string{"currency": "USD"}},
	} {
		f.want(f.call(admin, http.MethodPost, definitions, body), http.StatusUnprocessableEntity, "reject type-specific config")
	}

	saved := f.call(admin, http.MethodPatch, path, map[string]any{"fields": values}, "If-Match", `"1"`)
	f.want(saved, http.StatusOK, "save all ten types in one item edit")
	if !reflect.DeepEqual(saved.body["fields"], values) {
		t.Fatalf("fields after saving = %#v, want %#v", saved.body["fields"], values)
	}
	percent, amount := fieldIDs["percent"], fieldIDs["currency"]
	for fieldID, value := range map[string]any{fieldIDs["date"]: "2026-02-29", amount: 9.1234, fieldIDs["checkbox"]: 0} {
		bad := f.call(admin, http.MethodPatch, path, map[string]any{"fields": map[string]any{fieldID: value}}, "If-Match", `"2"`)
		f.want(bad, http.StatusUnprocessableEntity, "typed field-specific error")
		errors := bad.body["errors"].([]any)
		if errors[0].(map[string]any)["field"] != "fields."+fieldID {
			t.Fatalf("expected the invalid field ID in the error, got %+v", bad.body)
		}
	}
	// Settings that reinterpret committed values must be rejected.
	for _, cfg := range []map[string]any{{"currency": "IDR"}, {"currency": "USD", "decimals": 2}} {
		f.want(f.call(admin, http.MethodPatch, definitions+"/"+amount, map[string]any{"config": cfg}),
			http.StatusUnprocessableEntity, "protect stored amount and precision")
	}
	f.want(f.call(admin, http.MethodPatch, definitions+"/"+percent, map[string]any{"config": map[string]any{}}),
		http.StatusUnprocessableEntity, "cannot disable >100 while used")
	f.want(f.call(admin, http.MethodDelete, path, nil), http.StatusNoContent, "move referenced amount to recycle bin")
	f.want(f.call(admin, http.MethodPatch, definitions+"/"+amount, map[string]any{"config": map[string]any{"currency": "IDR"}}),
		http.StatusUnprocessableEntity, "recycled values still preserve currency")
	restored := f.call(admin, http.MethodPost, "/recycle-bin/"+item+"/restore", nil)
	f.want(restored, http.StatusOK, "restore referenced values")
	cleared := f.call(admin, http.MethodPatch, path, map[string]any{"fields": map[string]any{percent: nil, amount: nil}},
		"If-Match", restored.header.Get("ETag"))
	f.want(cleared, http.StatusOK, "clear typed values")
	if got := cleared.body["fields"].(map[string]any); got[amount] != nil || got[percent] != nil || got[fieldIDs["checkbox"]] != false {
		t.Fatalf("clear should preserve other values, including false: %v", got)
	}
	f.want(f.call(admin, http.MethodPatch, definitions+"/"+amount, map[string]any{"config": map[string]any{"currency": "IDR"}}),
		http.StatusOK, "change currency after clearing all values")
}

func TestCustomFieldPeopleDirectoryAndLocation(t *testing.T) {
	f := newFixture(t)
	admin, external, selected := f.person("admin"), f.person("external"), f.person("regular")
	hiddenPerson, active := f.person("regular"), f.person("regular")
	space, hiddenSpace := f.space("Visible"), f.space("Private fields")
	f.share(space, "user", external.id, "editor")
	f.share(space, "user", admin.id, "full")       // A1.2: the admin defines fields in both Spaces as an explicit member
	f.share(hiddenSpace, "user", admin.id, "full") // A1.2: the hidden Space is the admin's own; the others must still not see it
	id := customFieldItem(f, admin, space, "Multi-location")
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, hiddenSpace)
	peopleField := func(spaceID string) string {
		t.Helper()
		res := f.call(admin, http.MethodPost, customFieldsPath(spaceID), map[string]string{"name": "Reviewers", "type": "people"})
		f.want(res, http.StatusCreated, "create People field")
		return res.body["id"].(string)
	}
	field, hidden := peopleField(space), peopleField(hiddenSpace)
	path := "/items/" + id
	patch := func(actor *person, etag string, fields map[string]any) reply {
		return f.call(actor, http.MethodPatch, path, map[string]any{"fields": fields}, "If-Match", etag)
	}
	f.want(patch(admin, `"1"`, map[string]any{field: []string{selected.id}, hidden: []string{hiddenPerson.id}}),
		http.StatusOK, "assign visible active people")
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", selected.id)
	for _, unavailable := range []string{active.id, hiddenPerson.id, uuid.NewString()} {
		f.want(patch(external, `"2"`, map[string]any{field: []string{selected.id, unavailable}}),
			http.StatusUnprocessableEntity, "external editor cannot add people unavailable through directory")
	}
	f.want(patch(admin, `"2"`, map[string]any{field: []string{selected.id, uuid.NewString()}}),
		http.StatusUnprocessableEntity, "nonexistent person is not selectable")
	kept := patch(external, `"2"`, map[string]any{field: []string{selected.id}})
	f.want(kept, http.StatusOK, "previous selection can be kept by editor with no directory")
	for _, res := range []reply{kept, f.call(external, http.MethodGet, path, nil)} {
		people := res.body["field_people"].(map[string]any)
		ref := people[selected.id].(map[string]any)
		if len(people) != 1 || ref["status"] != "deactivated" || ref["name"] == "" || ref["email"] != nil ||
			res.body["fields"].(map[string]any)[hidden] != nil {
			t.Fatalf("projection leaked hidden metadata or failed to resolve inactive selection: %v", res.body)
		}
	}
	removed := patch(external, `"3"`, map[string]any{field: []string{}})
	f.want(removed, http.StatusOK, "existing inactive selection can be removed")
	if removed.body["field_people"] != nil {
		t.Fatalf("removed and hidden people must not be projected: %v", removed.body)
	}
	f.want(patch(admin, `"4"`, map[string]any{field: []string{selected.id}}), http.StatusUnprocessableEntity, "cannot newly add inactive person")
	f.want(patch(admin, `"4"`, map[string]any{field: []string{active.id}}), http.StatusOK, "can add active directory entry")
}
