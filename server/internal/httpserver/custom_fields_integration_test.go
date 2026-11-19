package httpserver_test

import (
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestBrowserCustomFields(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	owner := f.person("regular")
	space := f.space("Custom field browser fixture")
	f.share(space, "user", owner.id, "full")
	item := customFieldItem(f, owner, space, "Pilot task")
	typedSpace := f.space("Typed fields browser fixture")
	f.share(typedSpace, "user", owner.id, "full")
	typedItem := customFieldItem(f, owner, typedSpace, "Typed task")
	inactive, active := f.person("regular"), f.person("regular")
	f.exec("UPDATE users SET first_name='Review',last_name='Former' WHERE id=$1", inactive.id)
	f.exec("UPDATE users SET first_name='Review',last_name='Current' WHERE id=$1", active.id)
	currency := f.call(owner, "POST", customFieldsPath(typedSpace), map[string]any{
		"name": "Budget", "type": "currency", "config": map[string]string{"currency": "USD"},
	})
	f.want(currency, 201, "create Currency fixture")
	people := f.call(owner, "POST", customFieldsPath(typedSpace), map[string]string{"name": "Reviewers", "type": "people"})
	f.want(people, 201, "create People fixture")
	amountField, peopleField := currency.body["id"].(string), people.body["id"].(string)
	f.want(f.call(owner, "PATCH", "/items/"+typedItem, map[string]any{"fields": map[string]any{
		amountField: 1234.5, peopleField: []string{inactive.id},
	}}, "If-Match", `"1"`), 200, "seed typed fields")
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", inactive.id)
	runBrowserFixture(t, f, []string{"custom-fields-live.spec.ts"},
		"PMS_FIXTURE_CUSTOM_FIELDS_SPACE="+space,
		"PMS_FIXTURE_CUSTOM_FIELDS_ITEM="+item,
		"PMS_FIXTURE_CUSTOM_FIELDS_COOKIE="+owner.cookie.String(),
		"PMS_FIXTURE_TYPED_FIELDS_ITEM="+typedItem,
		"PMS_FIXTURE_TYPED_FIELDS_AMOUNT="+amountField,
		"PMS_FIXTURE_TYPED_FIELDS_PEOPLE="+peopleField,
		"PMS_FIXTURE_TYPED_FIELDS_INACTIVE="+inactive.id,
		"PMS_FIXTURE_TYPED_FIELDS_ACTIVE="+active.id,
	)
}

func customFieldsPath(space string) string { return "/spaces/" + space + "/custom-fields" }

func customFieldItem(f *fixture, actor *person, space, title string) string {
	f.t.Helper()
	id := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task',$1,$2) RETURNING id::text", title, actor.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
	return id
}

func customFieldOption(label string) map[string]string {
	return map[string]string{"id": uuid.NewString(), "label": label, "color": "#3B82F6"}
}

func TestCustomFieldConcurrentOptionRemoval(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Concurrent field changes")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	selected, remaining := customFieldOption("Selected"), customFieldOption("Remaining")
	definitions := customFieldsPath(space)
	created := f.call(admin, "POST", definitions, map[string]any{
		"name": "Impact", "type": "single_select", "options": []any{selected, remaining},
	})
	if created.status != 201 {
		t.Fatalf("create field: %+v", created)
	}
	field := created.body["id"].(string)
	item := customFieldItem(f, admin, space, "Concurrent value")
	itemPath := "/items/" + item

	var saved, removed reply
	start := make(chan struct{})
	var requests sync.WaitGroup
	requests.Add(2)
	go func() {
		defer requests.Done()
		<-start
		saved = f.call(admin, "PATCH", itemPath, map[string]any{"fields": map[string]any{
			field: selected["id"],
		}}, "If-Match", `"1"`)
	}()
	go func() {
		defer requests.Done()
		<-start
		removed = f.call(admin, "PATCH", definitions+"/"+field, map[string]any{"options": []any{remaining}})
	}()
	close(start)
	requests.Wait()
	if (saved.status == 200) == (removed.status == 200) {
		t.Fatalf("exactly one change must succeed: save=%+v remove=%+v", saved, removed)
	}
	for _, result := range []reply{saved, removed} {
		if result.status != 200 && result.status != 409 && result.status != 422 {
			t.Fatalf("unexpected mutation result: %+v", result)
		}
	}
	after := f.call(admin, "GET", itemPath, nil)
	if after.status != 200 || (after.body["fields"].(map[string]any)[field] == selected["id"]) != (saved.status == 200) {
		t.Fatalf("stored value did not match the committed mutation: %+v", after)
	}
}

func TestCustomFieldLifecycleAndItemValues(t *testing.T) {
	f := newFixture(t)
	admin, editor, reader, owner := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	primary, other := f.space("Primary fields"), f.space("Other fields")
	f.share(other, "user", admin.id, "full") // A1.2: the admin also defines fields in the other Space
	f.share(primary, "user", editor.id, "editor")
	f.share(primary, "user", reader.id, "read")
	f.share(primary, "user", owner.id, "full")
	f.share(primary, "user", admin.id, "full") // A1.2: the admin defines this Space's fields as an explicit member
	low, high, future := customFieldOption("Low"), customFieldOption("High"), customFieldOption("Future")
	body := map[string]any{"name": "Impact", "type": "single_select", "options": []any{low, high}}
	path := customFieldsPath(primary)
	if got := f.call(editor, "POST", path, body); got.status != 403 {
		t.Fatalf("editor create definition: %+v", got)
	}
	created := f.call(owner, "POST", path, body)
	if created.status != 201 || created.body["position"] != float64(0) {
		t.Fatalf("full Space role create: %+v", created)
	}
	single := created.body["id"].(string)
	singlePath := path + "/" + single
	created = f.call(admin, "POST", path, map[string]any{"name": "Labels", "type": "multi_select", "options": []any{low, high}})
	if created.status != 201 || created.body["position"] != float64(1) {
		t.Fatalf("create multi: %+v", created)
	}
	multi := created.body["id"].(string)
	multiPath := path + "/" + multi
	created = f.call(admin, "POST", customFieldsPath(other), map[string]any{"name": "Private scoring", "type": "single_select", "options": []any{future}})
	if created.status != 201 {
		t.Fatalf("create other field: %+v", created)
	}
	foreign := created.body["id"].(string)
	if got := f.call(admin, "PATCH", path+"/"+foreign, map[string]string{"name": "Not in this space"}); got.status != 404 {
		t.Fatalf("cross-Space definition update: %+v", got)
	}
	if got := f.call(admin, "DELETE", path+"/"+foreign, nil); got.status != 404 {
		t.Fatalf("cross-Space archive: %+v", got)
	}
	if got := f.call(editor, "PATCH", singlePath, map[string]string{"name": "Forbidden"}); got.status != 403 {
		t.Fatalf("editor definition rename: %+v", got)
	}
	if got := f.call(reader, "DELETE", singlePath, nil); got.status != 403 {
		t.Fatalf("reader archive: %+v", got)
	}
	if got := f.call(reader, "GET", customFieldsPath(other), nil); got.status != 404 {
		t.Fatalf("hidden Space list: %+v", got)
	}
	if got := f.call(owner, "PATCH", multiPath, map[string]any{"position": 0, "name": "Ordered labels"}); got.status != 200 {
		t.Fatalf("reorder: %+v", got)
	}
	if got := f.call(reader, "GET", path, nil); got.status != 200 || len(got.items()) != 2 ||
		got.items()[0]["id"] != multi || got.items()[0]["name"] != "Ordered labels" || got.items()[1]["position"] != float64(1) {
		t.Fatalf("ordered definitions for reader: %+v", got)
	}

	id := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Shared project',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, primary)
	item := "/items/" + id
	patch := func(p *person, version string, values map[string]any) reply {
		t.Helper()
		return f.call(p, "PATCH", item, map[string]any{"fields": values}, "If-Match", version)
	}
	for _, invalid := range []map[string]any{
		{uuid.NewString(): low["id"]}, {foreign: future["id"]}, {single: []string{low["id"]}},
		{multi: high["id"]}, {multi: []string{high["id"], high["id"]}}, {multi: []string{uuid.NewString()}},
	} {
		if got := patch(admin, `"1"`, invalid); got.status != 422 {
			t.Fatalf("invalid value %v: %+v", invalid, got)
		}
	}
	if got := patch(reader, `"1"`, map[string]any{single: low["id"]}); got.status != 403 {
		t.Fatalf("reader item update: %+v", got)
	}
	if got := patch(editor, `"1"`, map[string]any{single: low["id"], multi: []string{high["id"]}}); got.status != 200 || got.header.Get("ETag") != `"2"` {
		t.Fatalf("editor values: %+v", got)
	}
	if got := patch(editor, `"1"`, map[string]any{single: high["id"]}); got.status != 409 {
		t.Fatalf("stale value update: %+v", got)
	}
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, other)
	if got := patch(editor, `"2"`, map[string]any{foreign: future["id"]}); got.status != 422 {
		t.Fatalf("hidden placement update: %+v", got)
	}
	if got := patch(admin, `"2"`, map[string]any{foreign: future["id"]}); got.status != 200 || len(got.body["fields"].(map[string]any)) != 3 {
		t.Fatalf("merge multi-Space values: %+v", got)
	}
	if got := f.call(reader, "GET", item, nil); got.status != 200 || len(got.body["fields"].(map[string]any)) != 2 || got.body["fields"].(map[string]any)[single] != low["id"] {
		t.Fatalf("reader sees only values in visible Space: %+v", got)
	}
	if got := f.call(owner, "PATCH", singlePath, map[string]any{"options": []any{high}}); got.status != 422 {
		t.Fatalf("cannot remove selected single option: %+v", got)
	}
	if got := f.call(owner, "PATCH", multiPath, map[string]any{"options": []any{low}}); got.status != 422 {
		t.Fatalf("cannot remove selected multi option: %+v", got)
	}
	if got := f.call(owner, "PATCH", singlePath, map[string]any{"options": []any{low}}); got.status != 200 {
		t.Fatalf("can remove unused single option: %+v", got)
	}
	if got := f.call(admin, "DELETE", singlePath, nil); got.status != 204 {
		t.Fatalf("archive: %+v", got)
	}
	if got := f.call(reader, "GET", item, nil); got.status != 200 || got.body["fields"].(map[string]any)[single] != low["id"] {
		t.Fatalf("archival discarded stored value: %+v", got)
	}
	if got := patch(admin, `"3"`, map[string]any{single: low["id"]}); got.status != 422 {
		t.Fatalf("set archived value: %+v", got)
	}
	if got := patch(editor, `"3"`, map[string]any{single: nil}); got.status != 200 {
		t.Fatalf("clear archived value: %+v", got)
	}
	if got := f.call(admin, "GET", item, nil); got.status != 200 || len(got.body["fields"].(map[string]any)) != 2 ||
		got.body["fields"].(map[string]any)[foreign] != future["id"] {
		t.Fatalf("clear preserves hidden and untouched values: %+v", got)
	}
}

func TestCustomFieldOptionsPreservedForDeletedItems(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Restorable options")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	keep, unused := customFieldOption("Keep"), customFieldOption("Unused")
	root := customFieldsPath(space)
	created := f.call(admin, "POST", root, map[string]any{"name": "Labels", "type": "multi_select", "options": []any{keep, unused}})
	if created.status != 201 {
		t.Fatalf("create field: %+v", created)
	}
	field := created.body["id"].(string)
	item := customFieldItem(f, admin, space, "To restore")
	path := "/items/" + item
	if got := f.call(admin, "PATCH", path, map[string]any{"fields": map[string]any{field: []string{keep["id"]}}}, "If-Match", `"1"`); got.status != 200 {
		t.Fatalf("set item values: %+v", got)
	}
	if got := f.call(admin, "DELETE", path, nil); got.status != 204 {
		t.Fatalf("delete item: %+v", got)
	}
	if got := f.call(admin, "PATCH", root+"/"+field, map[string]any{"options": []any{unused}}); got.status != 422 {
		t.Fatalf("cannot remove option used by deleted item: %+v", got)
	}
	if got := f.call(admin, "POST", "/recycle-bin/"+item+"/restore", nil); got.status != 200 ||
		len(got.body["fields"].(map[string]any)[field].([]any)) != 1 {
		t.Fatalf("restore values: %+v", got)
	}
}
