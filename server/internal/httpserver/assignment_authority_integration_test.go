package httpserver_test

import (
	"fmt"
	"net/http"
	"testing"
)

type assignCast struct {
	f                                        *fixture
	full, editor, adminEditor, adminRead     *person
	adminFull, outsiderAdmin, teammate, read *person
	space                                    string
}

func newAssignCast(t *testing.T) assignCast {
	f := newFixture(t)
	c := assignCast{f: f, full: f.person("regular"), editor: f.person("regular"), adminEditor: f.unlinkedAdmin(), adminRead: f.unlinkedAdmin(),
		adminFull: f.unlinkedAdmin(), outsiderAdmin: f.unlinkedAdmin(), teammate: f.person("regular"), read: f.person("regular")}
	c.space = f.spaceWithoutAdmins("Assign matrix")
	for p, role := range map[*person]string{c.full: "full", c.editor: "editor", c.adminEditor: "editor", c.adminRead: "read", c.adminFull: "full", c.teammate: "editor", c.read: "read"} {
		f.share(c.space, "user", p.id, role)
	}
	return c
}

// DEF-028: effective permission decides who may assign, for every kind of account.
func TestAssignmentFollowsEffectivePermissionNotWorkspaceRole(t *testing.T) {
	c := newAssignCast(t)
	f := c.f
	task := f.call(c.full, "POST", "/items", map[string]any{"type": "task", "title": "T", "locations": []map[string]string{{"space_id": c.space}}})
	f.want(task, 201, "task")
	id, version := task.body["id"].(string), 1
	try := func(p *person, want int, what string, ids ...string) {
		t.Helper()
		if ids == nil {
			ids = []string{}
		}
		got := f.call(p, "PATCH", "/items/"+id, map[string]any{"assignee_ids": ids}, "If-Match", fmt.Sprintf(`"%d"`, version))
		f.want(got, want, what)
		if want == http.StatusOK {
			version++
		}
	}
	create := func(p *person, want int, what string, ids ...string) {
		t.Helper()
		f.want(f.call(p, "POST", "/items", map[string]any{"type": "task", "title": what, "assignee_ids": ids, "locations": []map[string]string{{"space_id": c.space}}}), want, what)
	}
	create(c.adminEditor, 403, "workspace admin with Editor assigns on create", c.teammate.id)
	create(c.adminEditor, 201, "workspace admin with Editor assigns self", c.adminEditor.id)
	create(c.adminFull, 201, "workspace admin with Full", c.teammate.id)
	create(c.adminRead, 403, "workspace admin with Read", c.adminRead.id)
	create(c.outsiderAdmin, 403, "workspace admin without a grant", c.teammate.id)
	try(c.adminEditor, 403, "workspace admin with Editor assigns on edit", c.teammate.id)
	try(c.adminEditor, 200, "and takes it", c.adminEditor.id)
	try(c.editor, 403, "multi-assignee with self and someone else", c.adminEditor.id, c.editor.id, c.teammate.id)
	try(c.editor, 200, "adds self beside an existing assignee", c.adminEditor.id, c.editor.id)
	try(c.adminRead, 403, "Read", c.adminRead.id)
	try(c.outsiderAdmin, 404, "no grant", c.teammate.id)
	try(c.full, 200, "Full assigns", c.teammate.id)
	f.want(f.call(c.editor, "PATCH", "/items/"+id, map[string]any{"title": "Renamed"}, "If-Match", fmt.Sprintf(`"%d"`, version)), 200, "other fields keep the assignee")
	version++
	// Behaviour as it stands, not a decision: an Editor may still remove other people (documented as open).
	try(c.editor, 200, "empty list")
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", c.read.id)
	try(c.full, 422, "inactive person", c.read.id)
	f.exec("UPDATE shares SET role='editor' WHERE target_id=$1 AND principal_id=$2", c.space, c.full.id)
	try(c.full, 403, "after losing Full", c.teammate.id)
}

// DEF-028: a second location the actor controls, or a personal Space, gives no say over who does the work.
func TestSecondLocationDoesNotGrantAssignment(t *testing.T) {
	c := newAssignCast(t)
	f := c.f
	task := f.call(c.full, "POST", "/items", map[string]any{"type": "task", "title": "Shared work", "locations": []map[string]string{{"space_id": c.space}}})
	id := task.body["id"].(string)
	own := f.call(c.editor, "POST", "/spaces", map[string]any{"name": "Mine"})
	f.want(own, 201, "own space")
	personal := f.queryID("SELECT id::text FROM spaces WHERE kind='personal' AND owner_id=$1", c.editor.id)
	for _, extra := range []string{own.body["id"].(string), personal} {
		f.want(f.call(c.editor, "PUT", "/items/"+id+"/locations", map[string]any{"locations": []map[string]any{{"space_id": c.space}, {"space_id": extra}}}), 200, "add a location")
		version := f.queryID("SELECT version::text FROM items WHERE id=$1", id)
		f.want(f.call(c.editor, "PATCH", "/items/"+id, map[string]any{"assignee_ids": []string{c.teammate.id}}, "If-Match", `"`+version+`"`), 403, "still an Editor where the work lives")
		f.want(f.call(c.editor, "PUT", "/items/"+id+"/locations", map[string]any{"locations": []map[string]any{{"space_id": c.space}}}), 200, "remove it again")
	}
	mine := f.call(c.editor, "POST", "/items", map[string]any{"type": "task", "title": "Private", "locations": []map[string]string{{"space_id": personal}}})
	f.want(f.call(c.editor, "PATCH", "/items/"+mine.body["id"].(string), map[string]any{"assignee_ids": []string{c.teammate.id}}, "If-Match", `"1"`), 403, "a personal Task takes nobody else, as on create")
	// Full somewhere else, no sight of the second Space: the manager keeps the right.
	elsewhere := f.spaceWithoutAdmins("Other team")
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, elsewhere)
	version := f.queryID("SELECT version::text FROM items WHERE id=$1", id)
	f.want(f.call(c.full, "PATCH", "/items/"+id, map[string]any{"assignee_ids": []string{c.teammate.id}}, "If-Match", `"`+version+`"`), 200, "Full where visible")
}

// DEF-028: automation runs with its owner's rights, so only someone who may assign changes what it assigns or hands it over.
func TestAutomationCannotBorrowAssignmentRights(t *testing.T) {
	c := newAssignCast(t)
	f := c.f
	def := func(action map[string]any) map[string]any {
		return map[string]any{"when": map[string]any{"type": "item.created"}, "then": []any{action}}
	}
	assign := def(map[string]any{"type": "assign", "user_ids": []string{c.teammate.id}})
	harmless := def(map[string]any{"type": "comment", "text": "hello"})
	rule := f.call(c.full, "POST", "/spaces/"+c.space+"/automation-rules", map[string]any{"name": "Full's rule", "definition": harmless})
	f.want(rule, 201, "rule")
	id := rule.body["id"].(string)
	f.want(f.call(c.adminEditor, "PUT", "/automation-rules/"+id+"/draft", map[string]any{"definition": assign}), 403, "an admin who is an Editor cannot make Full's rule assign")
	f.want(f.call(c.adminEditor, "PUT", "/automation-rules/"+id+"/draft", map[string]any{"definition": harmless}), 200, "other changes stay as before")
	f.want(f.call(c.full, "PUT", "/automation-rules/"+id+"/draft", map[string]any{"definition": assign}), 200, "owner drafts it")
	f.want(f.call(c.adminEditor, "POST", "/automation-rules/"+id+"/publish", nil), 403, "cannot publish it either")
	f.want(f.call(c.adminFull, "POST", "/automation-rules/"+id+"/publish", nil), 200, "an admin with Full can")
	mine := f.call(c.adminEditor, "POST", "/spaces/"+c.space+"/automation-rules", map[string]any{"name": "Mine", "definition": assign})
	f.want(mine, 201, "own rule: it only ever runs with own rights")
	f.want(f.call(c.adminEditor, "POST", "/automation-rules/"+mine.body["id"].(string)+"/owner", map[string]any{"owner_id": c.full.id}), 403, "cannot hand it to a Full owner")
}

// DEF-028: once a form is open to others, only someone who may assign changes whom it assigns, directly or through a blueprint.
func TestOpenFormRoutingNeedsAssignmentRight(t *testing.T) {
	c := newAssignCast(t)
	f := c.f
	here := []map[string]string{{"space_id": c.space}}
	source := f.call(c.full, "POST", "/items", map[string]any{"type": "project", "title": "Kit", "locations": here})
	saved := f.call(c.full, "POST", "/spaces/"+c.space+"/blueprints", map[string]any{"name": "Kit", "source_item_id": source.body["id"]})
	f.want(saved, 201, "blueprint")
	blueprint := saved.body["id"].(string)
	body := func(assignees []string, routes []any, elements ...map[string]any) map[string]any {
		els := append([]map[string]any{{"key": "pick", "kind": "single_choice", "label": "Pick", "options": []string{"A", "B"}}}, elements...)
		response := map[string]any{"item_type": "project", "location": map[string]any{"space_id": c.space}, "assignee_ids": assignees}
		if routes != nil {
			response["routes"] = routes
		}
		return map[string]any{"name": "Intake", "definition": map[string]any{"pages": []map[string]any{{"title": "One", "elements": els}}}, "response": response}
	}
	form := f.call(c.editor, "POST", "/spaces/"+c.space+"/forms", body([]string{}, nil))
	f.want(form, 201, "form")
	id := form.body["id"].(string)
	put := func(p *person, want int, what string, b map[string]any) {
		t.Helper()
		version := f.queryID("SELECT version::text FROM request_forms WHERE id=$1", id)
		f.want(f.call(p, "PUT", "/forms/"+id, b, "If-Match", `"`+version+`"`), want, what)
	}
	put(c.editor, 200, "a private form is the Editor's to shape", body([]string{c.teammate.id}, nil))
	put(c.editor, 200, "and to clear", body([]string{}, nil))
	f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", id)
	put(c.editor, 403, "open form: add a preset", body([]string{c.teammate.id}, nil))
	put(c.adminEditor, 403, "nor a workspace admin who is an Editor", body([]string{c.teammate.id}, nil))
	put(c.editor, 403, "open form: add an assignee question", body([]string{}, nil, map[string]any{"key": "who", "kind": "assignee", "label": "Who"}))
	route := []any{map[string]any{"when": map[string]any{"key": "pick", "op": "equals", "value": "A"}, "type_key": "project", "blueprint_id": blueprint}}
	put(c.editor, 403, "open form: route to a blueprint", body([]string{}, route))
	put(c.editor, 200, "wording stays the Editor's", body([]string{}, nil))
	put(c.full, 200, "Full routes it", body([]string{c.teammate.id}, route))
	version := f.queryID("SELECT version::text FROM blueprints WHERE id=$1", blueprint)
	tree := map[string]any{"nodes": []map[string]any{{"parent": -1, "type": "project", "title": "Kit", "importance": "normal", "assignee_ids": []string{c.teammate.id}}}, "dependencies": []any{}}
	var v int
	fmt.Sscan(version, &v)
	f.want(f.call(c.editor, "PATCH", "/blueprints/"+blueprint, map[string]any{"version": v, "tree": tree}), 403, "nor add people to the blueprint an open form applies")
	f.want(f.call(c.full, "PATCH", "/blueprints/"+blueprint, map[string]any{"version": v, "tree": tree}), 200, "Full can")
}
