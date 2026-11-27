package httpserver_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/projectexcel"
	"github.com/google/uuid"
)

// typeDefaultsEnv: a custom type "bug" whose defaults are importance=high (built-in field) plus Note="n/a" and Sev=<option> (custom fields).
type typeDefaultsEnv struct {
	e           *autoEnv
	bug, idea   string
	note, sev   string
	sevOption   string
	bugVersion  string
	workflowKey string
}

func newTypeDefaultsEnv(t *testing.T) *typeDefaultsEnv {
	e := newAutoEnv(t)
	f := e.f
	d := &typeDefaultsEnv{e: e, sevOption: uuid.NewString()}
	bug := f.call(e.admin, "POST", "/item-types", map[string]any{"key": "bug", "name": "Bug", "base": "task", "defaults": map[string]any{"importance": "high"}})
	idea := f.call(e.admin, "POST", "/item-types", map[string]any{"key": "idea", "name": "Idea", "base": "task"})
	f.want(bug, 201, "bug type")
	f.want(idea, 201, "idea type")
	d.bug, d.idea = bug.body["id"].(string), idea.body["id"].(string)
	note := f.call(e.admin, "POST", "/custom-fields", map[string]any{"name": "Note", "type": "text"})
	sev := f.call(e.admin, "POST", "/custom-fields", map[string]any{"name": "Sev", "type": "single_select",
		"options": []map[string]any{{"id": d.sevOption, "label": "High", "color": "#DC2626"}}})
	f.want(note, 201, "note field")
	f.want(sev, 201, "sev field")
	d.note, d.sev = note.body["id"].(string), sev.body["id"].(string)
	// Workspace fields apply in a Space only once the Space is assigned to them (DR-08).
	for _, fid := range []string{d.note, d.sev} {
		f.want(f.call(e.admin, "POST", "/spaces/"+e.space+"/custom-fields/"+fid+"/assignments", map[string]any{"scope_kind": "space", "scope_id": e.space}), 201, "assign Workspace field to the Space")
	}
	f.want(f.call(e.admin, "PUT", "/item-types/"+d.bug+"/fields/"+d.note, map[string]any{"default": "n/a"}), 200, "note default")
	f.want(f.call(e.admin, "PUT", "/item-types/"+d.bug+"/fields/"+d.sev, map[string]any{"default": d.sevOption}), 200, "sev default")
	return d
}

// wantDefaults checks the item whose title is given got every default of the bug type.
func (d *typeDefaultsEnv) wantDefaults(t *testing.T, title, path string) {
	t.Helper()
	f := d.e.f
	id := f.queryID("SELECT id::text FROM items WHERE title=$1 AND deleted_at IS NULL", title)
	got := f.call(d.e.admin, "GET", "/items/"+id, nil)
	f.want(got, 200, path+": read")
	fields, _ := got.body["fields"].(map[string]any)
	if got.body["importance"] != "high" || fields[d.note] != "n/a" || fields[d.sev] != d.sevOption || got.body["item_type"].(map[string]any)["key"] != "bug" {
		t.Fatalf("%s: defaults missing: %+v", path, got.body)
	}
}

// S8.1.2: the same defaults reach a new item whichever way it is created.
func TestItemTypeDefaultsApplyOnEveryCreatePath(t *testing.T) {
	d := newTypeDefaultsEnv(t)
	e, f := d.e, d.e.f

	// API
	f.want(f.call(e.editor, "POST", "/items", map[string]any{"type_key": "bug", "title": "via api", "locations": []map[string]any{{"space_id": e.space}}}), 201, "api create")
	d.wantDefaults(t, "via api", "api")

	// Form: the answer fills the title only; the type's defaults fill the rest.
	requester := f.person("regular")
	form := f.call(e.editor, "POST", "/spaces/"+e.space+"/forms", map[string]any{"name": "Report", "definition": map[string]any{"pages": []map[string]any{{"title": "P", "elements": []map[string]any{
		{"key": "t", "kind": "single_line", "label": "Title", "required": true}}}}},
		"response": map[string]any{"item_type": "task", "location": map[string]any{"space_id": e.space}, "title_question": "t", "type_key": "bug"}})
	f.want(form, 201, "form")
	f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", form.body["id"])
	f.publishForm(e.editor, form.body["id"].(string))
	f.want(f.submit(requester, form.body["id"].(string), map[string]any{"t": "via form"}), 201, "form submit")
	d.wantDefaults(t, "via form", "form")

	// Automation: a rule creates a bug when an idea appears.
	e.rule(t, e.editor, "Idea to bug", `{"when":{"type":"item.created","item_type":"idea"},"then":[{"type":"create_item","title":"via rule","type_key":"bug"}]}`)
	f.want(f.call(e.editor, "POST", "/items", map[string]any{"type_key": "idea", "title": "an idea", "locations": []map[string]any{{"space_id": e.space}}}), 201, "idea")
	e.drain(t)
	d.wantDefaults(t, "via rule", "automation")

	// Excel: blank cells take the defaults, an explicit cell wins, and the type is named by key or by name.
	file, _ := projectexcel.Write([][]string{{"title", "type", "field:" + d.note},
		{"via excel", "bug", ""}, {"excel by name", "Bug", ""}, {"excel explicit", "bug", "mine"}})
	up := f.uploadExcel(e.admin, e.space, file, nil)
	f.want(up, 200, "excel preview")
	f.want(f.uploadExcel(e.admin, e.space, file, map[string]string{"commit": "true", "digest": up.body["digest"].(string)}), 201, "excel commit")
	d.wantDefaults(t, "via excel", "excel")
	d.wantDefaults(t, "excel by name", "excel by name")
	explicit := f.call(e.admin, "GET", "/items/"+f.queryID("SELECT id::text FROM items WHERE title='excel explicit'"), nil)
	if explicit.body["fields"].(map[string]any)[d.note] != "mine" || explicit.body["fields"].(map[string]any)[d.sev] != d.sevOption {
		t.Fatalf("explicit value must beat the default and others still apply: %+v", explicit.body)
	}
	// Unknown and inactive types are refused row by row, naming the row and column; nothing is created.
	f.want(f.call(e.admin, "PATCH", "/item-types/"+d.idea, map[string]any{"active": false}, "If-Match", `"1"`), 200, "deactivate idea")
	bad, _ := projectexcel.Write([][]string{{"title", "type"}, {"ok row", "bug"}, {"ghost", "nope"}, {"inactive", "idea"}})
	got := f.uploadExcel(e.admin, e.space, bad, nil)
	f.want(got, 200, "bad types preview")
	errs := got.body["errors"].([]any)
	if len(errs) != 2 || errs[0].(map[string]any)["row"] != float64(3) || errs[0].(map[string]any)["column"] != "type" || errs[1].(map[string]any)["row"] != float64(4) {
		t.Fatalf("unknown type errors: %+v", errs)
	}
	f.want(f.uploadExcel(e.admin, e.space, bad, map[string]string{"commit": "true", "digest": got.body["digest"].(string)}), 422, "commit refused")
	if f.queryID("SELECT count(*)::text FROM items WHERE title IN ('ok row','ghost','inactive')") != "0" {
		t.Fatal("refused import created items")
	}
}

// The template path: a Space built from a template creates items through the same code, so a type's default reaches them.
func TestItemTypeDefaultsApplyToSpaceTemplates(t *testing.T) {
	f := newFixture(t)
	admin, regular := f.person("admin"), f.person("regular")
	task := f.queryID("SELECT id::text FROM item_type_defs WHERE is_builtin AND key='task'")
	f.want(f.call(admin, "PATCH", "/item-types/"+task, map[string]any{"defaults": map[string]any{"importance": "high"}}, "If-Match", `"1"`), 200, "task defaults")
	made := f.call(regular, "POST", "/space-templates/project-management/spaces", map[string]any{"scope_state": "internal", "request_id": uuid.NewString(), "name": "Templated"})
	f.want(made, 201, "template space")
	if f.queryID(`SELECT count(*)::text FROM items i JOIN item_locations l ON l.item_id=i.id WHERE l.space_id=$1 AND i.type='task' AND i.importance='high'`, made.body["id"]) == "0" {
		t.Fatal("template tasks without an explicit importance must take the type default")
	}
}

// Admin API, validation, and that removing a field or type never breaks creation.
func TestItemTypeFieldDefaultsAdminAPIAndOrphans(t *testing.T) {
	d := newTypeDefaultsEnv(t)
	e, f := d.e, d.e.f
	create := func(title string) reply {
		return f.call(e.editor, "POST", "/items", map[string]any{"type_key": "bug", "title": title, "locations": []map[string]any{{"space_id": e.space}}})
	}
	f.want(f.call(e.editor, "GET", "/item-types/"+d.bug+"/fields", nil), 403, "list needs admin")
	f.want(f.call(e.editor, "PUT", "/item-types/"+d.bug+"/fields/"+d.note, map[string]any{"default": "x"}), 403, "set needs admin")
	f.want(f.call(e.admin, "PUT", "/item-types/"+d.bug+"/fields/"+d.sev, map[string]any{"default": uuid.NewString()}), 422, "default must be an option")
	f.want(f.call(e.admin, "PUT", "/item-types/"+d.bug+"/fields/"+d.note, map[string]any{"default": 12}), 422, "default must be text")
	f.want(f.call(e.admin, "PUT", "/item-types/"+d.bug+"/fields/"+uuid.NewString(), map[string]any{"default": "x"}), 404, "unknown field")
	people := f.call(e.admin, "POST", "/custom-fields", map[string]any{"name": "Who", "type": "people"})
	f.want(people, 201, "people field")
	f.want(f.call(e.admin, "PUT", "/item-types/"+d.bug+"/fields/"+people.body["id"].(string), map[string]any{"default": []string{}}), 422, "no defaults for people")
	f.want(f.call(e.admin, "PUT", "/item-types/"+d.bug+"/fields/"+people.body["id"].(string), map[string]any{}), 200, "applies without default")
	list := f.call(e.admin, "GET", "/item-types/"+d.bug+"/fields", nil)
	f.want(list, 200, "list")
	if len(list.items()) != 3 {
		t.Fatalf("pairs: %+v", list.body)
	}

	// An option removed behind the default, and an archived field: creation still works and the orphan is ignored.
	f.exec("UPDATE custom_fields SET options=jsonb_build_array(jsonb_build_object('id',gen_random_uuid()::text,'label','Other','color','#000000')) WHERE id=$1", d.sev)
	f.want(f.call(e.admin, "DELETE", "/custom-fields/"+d.note, nil), 204, "archive note")
	got := create("orphans")
	f.want(got, 201, "create with orphaned defaults")
	if fields, _ := got.body["fields"].(map[string]any); fields[d.note] != nil || fields[d.sev] != nil {
		t.Fatalf("orphan default applied: %+v", got.body)
	}
	f.want(f.call(e.admin, "PUT", "/item-types/"+d.bug+"/fields/"+d.note, map[string]any{"default": "x"}), 422, "archived field cannot be attached")
	// Deleting a field row or a type row cascades; deactivating the type leaves the pairing intact.
	f.exec("DELETE FROM custom_fields WHERE id=$1", d.sev)
	f.want(create("after field delete"), 201, "create after field delete")
	f.want(f.call(e.admin, "DELETE", "/item-types/"+d.bug+"/fields/"+people.body["id"].(string), nil), 204, "detach")
	f.want(f.call(e.admin, "DELETE", "/item-types/"+d.bug+"/fields/"+people.body["id"].(string), nil), 404, "detach twice")
	f.exec("DELETE FROM item_type_defs WHERE id=$1 AND NOT EXISTS (SELECT 1 FROM items WHERE type_id=$1)", d.idea)
	if f.queryID("SELECT count(*)::text FROM item_type_fields WHERE type_id=$1", d.idea) != "0" {
		t.Fatal("pairing must go with its type")
	}
}

// S8.1.5: the Excel export names the type, a re-import keeps it, search and reports filter and group by it.
func TestItemTypeInExportSearchAndReports(t *testing.T) {
	d := newTypeDefaultsEnv(t)
	e, f := d.e, d.e.f
	startExcelWorker(t, f)
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Root',$1) RETURNING id::text", e.admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, e.space)
	mk := func(key, title string) string {
		got := f.call(e.editor, "POST", "/items", map[string]any{"type_key": key, "title": title, "locations": []map[string]any{{"space_id": e.space, "parent_item_id": project}}})
		f.want(got, 201, title)
		return got.body["id"].(string)
	}
	mk("bug", "Zebra crash")
	mk("idea", "Zebra idea")
	mk("task", "Zebra chore")

	// search
	for key, want := range map[string]int{"": 3, "bug": 1, "idea": 1, "task": 1, "nope": 0} {
		got := f.call(e.editor, "GET", "/search?q=zebra&type_key="+key, nil)
		f.want(got, 200, "search "+key)
		if n := len(got.body["items"].([]any)); n != want {
			t.Fatalf("search type_key=%q: %d items, want %d", key, n, want)
		}
	}
	// reports: filter by type and group by item type, through the policy-checked engine
	run := func(q map[string]any) []any {
		q["reporting_on"], q["metric"], q["space_ids"] = "tasks", "count", []string{e.space}
		got := f.call(e.editor, "POST", "/analytics/query", q)
		f.want(got, 200, "analytics")
		return got.body["rows"].([]any)
	}
	if rows := run(map[string]any{"group_by": "item_type"}); len(rows) != 3 {
		t.Fatalf("group by item type: %+v", rows)
	}
	if rows := run(map[string]any{"group_by": "item_type", "filters": map[string]any{"type_keys": []string{"bug"}}}); len(rows) != 1 || rows[0].(map[string]any)["group"].(map[string]any)["key"] != "bug" {
		t.Fatalf("filter by type: %+v", rows)
	}
	f.want(f.call(e.editor, "POST", "/analytics/query", map[string]any{"reporting_on": "time_entries", "metric": "minutes", "filters": map[string]any{"type_keys": []string{"bug"}}}), 422, "type filter is not for time entries")
	// a viewer without access to the Space sees nothing, whatever the type filter
	outsider := f.person("regular")
	if got := f.call(outsider, "GET", "/search?q=zebra&type_key=bug", nil); got.status != 200 || len(got.body["items"].([]any)) != 0 {
		t.Fatalf("type filter leaked: %+v", got)
	}

	// export: the type column carries the key, and importing the workbook again keeps the types
	res := f.call(e.admin, "POST", "/items/"+project+"/excel-exports", map[string]string{"space_id": e.space})
	f.want(res, 202, "export")
	path := "/excel-exports/" + res.body["id"].(string)
	for deadline := time.Now().Add(8 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		st := f.call(e.admin, "GET", path, nil)
		if st.body["status"] == "ready" {
			break
		}
		if st.body["status"] == "failed" || time.Now().After(deadline) {
			t.Fatalf("export: %+v", st)
		}
	}
	req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1"+path+"/file", nil)
	req.AddCookie(e.admin.cookie)
	r, err := f.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	data, _ := io.ReadAll(r.Body)
	rows, err := projectexcel.Read(data)
	if err != nil || rows[0][8] != "type_key" {
		t.Fatalf("export header: %v %v", rows, err)
	}
	keys := []string{}
	for _, row := range rows[1:] {
		keys = append(keys, row[8])
	}
	if got := strings.Join(keys, ","); !strings.Contains(got, "bug") || !strings.Contains(got, "idea") || !strings.Contains(got, "task") {
		t.Fatalf("exported type keys: %s", got)
	}
	// A Status is importable only under a destination whose workflow holds it, and this round trip imports at the Space root.
	// So the statuses are cleared here; the type round trip is what this test covers.
	for i := range rows {
		if i > 0 {
			rows[i][9], rows[i][10] = "", ""
		}
	}
	data, err = projectexcel.Write(rows)
	if err != nil {
		t.Fatal(err)
	}
	up := f.uploadExcel(e.admin, e.space, data, nil)
	f.want(up, 200, "preview the export")
	f.want(f.uploadExcel(e.admin, e.space, data, map[string]string{"commit": "true", "digest": up.body["digest"].(string)}), 201, "re-import")
	if f.queryID("SELECT count(*)::text FROM items i JOIN item_type_defs t ON t.id=i.type_id WHERE i.title='Zebra crash' AND t.key='bug'") != "2" ||
		f.queryID("SELECT count(*)::text FROM items i JOIN item_type_defs t ON t.id=i.type_id WHERE i.title='Zebra idea' AND t.key='idea'") != "2" {
		t.Fatal("round trip lost the item types")
	}
}
