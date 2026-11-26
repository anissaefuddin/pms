package httpserver_test

import (
	"testing"

	"github.com/google/uuid"
)

func TestSpaceTemplatesBuildWholeSpaceOnceAndOnlyForCreators(t *testing.T) {
	f := newFixture(t)
	admin, regular, guest := f.person("admin"), f.person("regular"), f.person("viewer")
	list := f.call(regular, "GET", "/space-templates", nil)
	if list.status != 200 || len(list.items()) < 4 {
		t.Fatalf("catalog: %+v", list)
	}
	for _, tpl := range list.items() {
		id := tpl["id"].(string)
		counts := tpl["counts"].(map[string]any)
		got := f.call(regular, "POST", "/space-templates/"+id+"/spaces", map[string]any{"request_id": uuid.NewString(), "name": "From " + id})
		if got.status != 201 {
			t.Fatalf("%s: %+v", id, got)
		}
		space := got.body["id"].(string)
		count := func(sql string) float64 {
			var n float64
			if err := f.pool.QueryRow(f.ctx, sql, space).Scan(&n); err != nil {
				t.Fatal(err)
			}
			return n
		}
		for what, want := range map[string]struct {
			sql  string
			want any
		}{
			"projects":      {"SELECT count(*) FROM item_locations l JOIN items i ON i.id=l.item_id WHERE l.space_id=$1 AND i.type='project'", counts["projects"]},
			"tasks":         {"SELECT count(*) FROM item_locations l JOIN items i ON i.id=l.item_id WHERE l.space_id=$1 AND i.type='task'", counts["tasks"]},
			"views":         {"SELECT count(*) FROM views WHERE scope_space_id=$1", counts["views"].(float64) + 4}, // + Table, Board, Gantt and Calendar every Space starts with,
			"forms":         {"SELECT count(*) FROM request_forms WHERE space_id=$1", counts["forms"]},
			"custom_fields": {"SELECT count(*) FROM custom_fields WHERE space_id=$1", counts["custom_fields"]},
		} {
			if count(want.sql) != want.want.(float64) {
				t.Errorf("%s %s: got %v, want %v", id, what, count(want.sql), want.want)
			}
		}
		// The creator sees everything and the form is submittable: the whole template is usable, not just stored.
		if got := f.call(regular, "GET", "/spaces/"+space+"/items", nil); got.status != 200 || len(got.items()) != int(counts["projects"].(float64)) {
			t.Errorf("%s root items: %+v", id, got)
		}
	}
	// Retry with the same request_id: the same Space comes back and nothing is duplicated.
	key := uuid.NewString()
	first := f.call(regular, "POST", "/space-templates/project-management/spaces", map[string]any{"request_id": key, "name": "Once"})
	again := f.call(regular, "POST", "/space-templates/project-management/spaces", map[string]any{"request_id": key, "name": "Twice"})
	if first.status != 201 || again.status != 200 || again.body["id"] != first.body["id"] {
		t.Fatalf("replay: %+v / %+v", first, again)
	}
	if n := f.queryID("SELECT count(*)::text FROM spaces WHERE name IN ('Once','Twice')"); n != "1" {
		t.Fatalf("duplicate Spaces: %s", n)
	}
	// Another user cannot replay (or probe) the key; viewers cannot create Spaces at all.
	if got := f.call(admin, "POST", "/space-templates/project-management/spaces", map[string]any{"request_id": key}); got.status != 404 {
		t.Fatalf("foreign replay: %+v", got)
	}
	if got := f.call(guest, "POST", "/space-templates/project-management/spaces", map[string]any{"request_id": uuid.NewString()}); got.status != 403 {
		t.Fatalf("guest: %+v", got)
	}
	for name, body := range map[string]map[string]any{"no key": {}, "bad key": {"request_id": "x"}} {
		if got := f.call(regular, "POST", "/space-templates/project-management/spaces", body); got.status != 422 {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	if got := f.call(regular, "POST", "/space-templates/unknown/spaces", map[string]any{"request_id": uuid.NewString()}); got.status != 404 {
		t.Fatalf("unknown template: %+v", got)
	}
	// A failure rolls everything back: a name that is too long fails after the run row was written.
	long := make([]byte, 130)
	for i := range long {
		long[i] = 'x'
	}
	bad := uuid.NewString()
	if got := f.call(regular, "POST", "/space-templates/hr-onboarding/spaces", map[string]any{"request_id": bad, "name": string(long)}); got.status != 422 {
		t.Fatalf("long name: %+v", got)
	}
	if n := f.queryID("SELECT count(*)::text FROM space_template_runs WHERE request_id=$1", bad); n != "0" {
		t.Fatalf("run row survived a failed build: %s", n)
	}
}

// DEF-012: a Space saved as a template gives a new Space with the same structure and configuration and none of the runtime data;
// only people with Full access save it, and only its author and people who see the source Space can use it.
func TestSaveSpaceAsTemplate(t *testing.T) {
	f := newFixture(t)
	owner, editor, outsider, guest := f.person("regular"), f.person("regular"), f.person("regular"), f.person("external")
	space := f.spaceWithoutAdmins("Source")
	f.share(space, "user", owner.id, "full")
	f.share(space, "user", editor.id, "editor")
	f.share(space, "user", guest.id, "editor")
	create := func(body map[string]any, parent any) string {
		location := map[string]any{"space_id": space}
		if parent != nil {
			location["parent_item_id"] = parent
		}
		body["locations"] = []any{location}
		made := f.call(owner, "POST", "/items", body)
		f.want(made, 201, body["title"].(string))
		return made.body["id"].(string)
	}
	folder := create(map[string]any{"type": "folder", "title": "Phase"}, nil)
	flow := f.queryID("INSERT INTO workflows(name) VALUES ('Delivery flow') RETURNING id::text")
	f.exec("INSERT INTO statuses(workflow_id,name,grp,color,position) VALUES ($1,'Start','active','blue',0),($1,'End','completed','green',1)", flow)
	f.exec("INSERT INTO space_default_workflows(space_id,base_type,workflow_id,set_by) VALUES ($1,'project',$2,$3)", space, flow, owner.id)
	project := create(map[string]any{"type": "project", "title": "Build", "default_view_type": "board", "default_workflow_id": flow}, folder)
	task := create(map[string]any{"type": "task", "title": "Design", "importance": "high", "start_date": "2026-11-23", "due_date": "2026-11-25", "assignee_ids": []string{editor.id}}, project)
	create(map[string]any{"type": "task", "title": "Sketch"}, task)
	create(map[string]any{"type": "task", "title": "Launch", "is_milestone": true}, project)
	gone := create(map[string]any{"type": "task", "title": "Removed"}, project)
	f.want(f.call(owner, "DELETE", "/items/"+gone, nil), 204, "remove one")
	field := func(body map[string]any) string {
		made := f.call(owner, "POST", "/spaces/"+space+"/custom-fields", body)
		f.want(made, 201, "field")
		return made.body["id"].(string)
	}
	amount := field(map[string]any{"name": "Amount", "type": "number"})
	field(map[string]any{"name": "Double", "type": "formula", "config": map[string]any{"expression": "[Amount]*2"}})
	secret := field(map[string]any{"name": "Margin", "type": "number"})
	field(map[string]any{"name": "Derived", "type": "formula", "config": map[string]any{"expression": "[Margin]*2"}})
	f.want(f.call(owner, "PUT", "/spaces/"+space+"/custom-fields/"+secret+"/grants", map[string]any{"grants": []any{map[string]any{"subject_kind": "user", "subject_id": owner.id, "level": "edit"}}}), 200, "restrict")
	f.want(f.call(owner, "PATCH", "/items/"+task, map[string]any{"fields": map[string]any{amount: 777}}, "If-Match", `"1"`), 200, "value")
	f.want(f.call(owner, "POST", "/views", map[string]any{"scope_space_id": space, "type": "table", "name": "Open work", "visibility": "public"}), 201, "view")

	body := map[string]any{"space_id": space, "name": "Delivery", "description": "Our way"}
	f.want(f.call(editor, "POST", "/space-templates", body), 403, "Editor")
	f.want(f.call(outsider, "POST", "/space-templates", body), 404, "outsider")
	f.want(f.call(guest, "POST", "/space-templates", body), 403, "a user type that cannot create Spaces")
	f.want(f.call(owner, "POST", "/space-templates", map[string]any{"space_id": space, "name": " "}), 422, "no name")
	personal := f.queryID("SELECT id::text FROM spaces WHERE kind='personal' AND owner_id=$1", owner.id)
	f.want(f.call(owner, "POST", "/space-templates", map[string]any{"space_id": personal, "name": "P"}), 403, "personal Space")
	saved := f.call(owner, "POST", "/space-templates", body)
	f.want(saved, 201, "save")
	id := saved.body["id"].(string)
	counts := saved.body["counts"].(map[string]any)
	if counts["projects"] != float64(1) || counts["tasks"] != float64(3) || counts["views"] != float64(1) || counts["custom_fields"] != float64(2) || saved.body["mine"] != true {
		t.Fatalf("summary: %v", saved.body)
	}
	listed := func(p *person) bool {
		for _, row := range f.call(p, "GET", "/space-templates", nil).items() {
			if row["id"] == id {
				return true
			}
		}
		return false
	}
	if !listed(owner) || !listed(editor) || listed(outsider) {
		t.Fatalf("visible to author and to people who see the source only: %v %v %v", listed(owner), listed(editor), listed(outsider))
	}
	use := func(p *person) reply {
		return f.call(p, "POST", "/space-templates/"+id+"/spaces", map[string]any{"request_id": uuid.NewString(), "name": "Copy"})
	}
	f.want(use(outsider), 404, "outsider cannot use it")
	made := use(editor)
	f.want(made, 201, "use")
	copied := made.body["id"].(string)
	if got := f.queryID(`SELECT string_agg(i.type||':'||i.title||COALESCE('<'||p.title,'')||':'||i.importance||':'||i.is_milestone, ' ' ORDER BY i.title)
FROM item_locations l JOIN items i ON i.id=l.item_id LEFT JOIN items p ON p.id=l.parent_item_id WHERE l.space_id=$1`, copied); got !=
		"project:Build<Phase:normal:false task:Design<Build:high:false task:Launch<Build:normal:true folder:Phase:normal:false task:Sketch<Design:normal:false" {
		t.Fatalf("structure: %s", got)
	}
	if got := f.queryID(`SELECT count(*)||' '||(SELECT count(*) FROM item_assignees a JOIN item_locations l ON l.item_id=a.item_id WHERE l.space_id=$1)
FROM items i JOIN item_locations l ON l.item_id=i.id WHERE l.space_id=$1 AND (i.start_date IS NOT NULL OR i.due_date IS NOT NULL OR i.fields<>'{}')`, copied); got != "0 0" {
		t.Fatalf("runtime data was copied: %s", got)
	}
	if got := f.queryID("SELECT string_agg(name,',' ORDER BY name) FROM custom_fields WHERE space_id=$1", copied); got != "Amount,Double" {
		t.Fatalf("fields: %s", got)
	}
	if got := f.queryID(`SELECT count(*)||' '||(SELECT count(*) FROM space_default_workflows WHERE space_id=$1 AND workflow_id=$2) FROM items i
JOIN item_locations l ON l.item_id=i.id WHERE l.space_id=$1 AND i.default_workflow_id=$2`, copied, flow); got != "1 1" {
		t.Fatalf("workflows: %s", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM views WHERE scope_space_id=$1 AND name='Open work'", copied); got != "1" {
		t.Fatalf("view: %s", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM shares WHERE target_id=$1", copied); got != "1" {
		t.Fatalf("membership was copied: %s", got)
	}
	f.want(f.call(editor, "DELETE", "/space-templates/"+id, nil), 404, "only the author deletes")
	f.want(f.call(owner, "DELETE", "/space-templates/"+id, nil), 204, "delete")
	f.want(use(owner), 404, "gone")
	f.want(f.call(editor, "GET", "/spaces/"+copied, nil), 200, "the Space made from it stays")
}
