package httpserver_test

import (
	"net/http"
	"testing"
)

func TestWorkflowEditor(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	space := f.space("Work")
	status := func(name string) string {
		return f.queryID("SELECT id::text FROM statuses WHERE name=$1 AND workflow_id=(SELECT id FROM workflows WHERE is_default)", name)
	}
	newItem := func() (string, string) {
		r := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": "T", "locations": []map[string]any{{"space_id": space}}})
		f.want(r, 201, "create item")
		return r.body["id"].(string), r.header.Get("ETag")
	}

	// Everyone reads; only admins write.
	got := f.call(member, "GET", "/workflows", nil)
	first := got.items()[0]["statuses"].([]any)[0].(map[string]any)
	if got.status != 200 || first["auto_assign"] != false || first["lock_transitions"] != false || len(first["allowed_next"].([]any)) != 0 {
		t.Fatalf("status flags in list: %v", first)
	}
	f.want(f.call(member, "POST", "/workflows", map[string]any{"name": "X"}), 403, "member create")
	f.want(f.call(member, "PATCH", "/statuses/"+status("New"), map[string]any{"name": "Nope"}), 403, "member edit status")
	f.want(f.call(member, "DELETE", "/statuses/"+status("New"), nil), 403, "member delete status")

	// Create, rename, duplicate name.
	wf := f.call(admin, "POST", "/workflows", map[string]any{"name": "Design"})
	f.want(wf, 201, "create workflow")
	wfID := wf.body["id"].(string)
	if len(wf.body["statuses"].([]any)) != 3 {
		t.Fatalf("new workflow starts with New, In Progress, Completed: %v", wf.body)
	}
	f.want(f.call(admin, "POST", "/workflows", map[string]any{"name": "design"}), 409, "duplicate name ignores case")
	f.want(f.call(admin, "POST", "/workflows", map[string]any{"name": "  "}), 422, "blank name")
	f.want(f.call(admin, "PATCH", "/workflows/"+wfID, map[string]any{"name": "Design v2"}), 200, "rename")
	f.want(f.call(admin, "PATCH", "/workflows/00000000-0000-0000-0000-000000000000", map[string]any{"name": "Z"}), 404, "rename missing")

	// Statuses: validation, group invariants, order.
	add := func(body map[string]any) reply { return f.call(admin, "POST", "/workflows/"+wfID+"/statuses", body) }
	f.want(add(map[string]any{"name": "Review", "group": "active", "color": "teal"}), 201, "add status")
	f.want(add(map[string]any{"name": "Review", "group": "active", "color": "teal"}), 409, "duplicate status name")
	f.want(add(map[string]any{"name": "Bad", "group": "active", "color": "#ff0000"}), 422, "color outside the palette")
	f.want(add(map[string]any{"name": "Bad", "group": "open", "color": "teal"}), 422, "unknown group")
	ids := []string{}
	for _, s := range f.call(admin, "GET", "/workflows", nil).items() {
		if s["id"] == wfID {
			for _, st := range s["statuses"].([]any) {
				ids = append(ids, st.(map[string]any)["id"].(string))
			}
		}
	}
	if len(ids) != 4 {
		t.Fatalf("statuses after add: %v", ids)
	}
	newStatus, inProgress, completed, review := ids[0], ids[1], ids[2], ids[3]
	f.want(f.call(admin, "PATCH", "/statuses/"+newStatus, map[string]any{"group": "deferred"}), 422, "first status must stay Active")
	f.want(f.call(admin, "PATCH", "/statuses/"+completed, map[string]any{"group": "active"}), 422, "last Completed status cannot change group")
	f.want(f.call(admin, "DELETE", "/statuses/"+completed, nil), 422, "last Completed status cannot be deleted")
	f.want(f.call(admin, "PUT", "/workflows/"+wfID+"/status-order", map[string]any{"status_ids": ids[:3]}), 422, "order must list every status")
	f.want(f.call(admin, "PUT", "/workflows/"+wfID+"/status-order", map[string]any{"status_ids": []string{completed, newStatus, inProgress, review}}), 422, "first status must be Active")
	re := f.call(admin, "PUT", "/workflows/"+wfID+"/status-order", map[string]any{"status_ids": []string{newStatus, review, inProgress, completed}})
	if re.status != 200 || re.body["statuses"].([]any)[1].(map[string]any)["id"] != review {
		t.Fatalf("reorder: %d %v", re.status, re.body)
	}
	f.want(f.call(admin, "PATCH", "/statuses/"+review, map[string]any{}), 422, "empty patch")

	// Transitions on the default workflow: a locked status only leads to the listed ones.
	item, etag := newItem()
	newDefault, progress, done := status("New"), status("In Progress"), status("Completed")
	f.want(f.call(admin, "PATCH", "/statuses/"+newDefault, map[string]any{"lock_transitions": true, "allowed_next": []string{progress}}), 200, "lock New")
	f.want(f.call(admin, "PATCH", "/statuses/"+newDefault, map[string]any{"allowed_next": []string{newDefault}}), 422, "status cannot lead to itself")
	f.want(f.call(admin, "PATCH", "/statuses/"+newDefault, map[string]any{"allowed_next": []string{newStatus}}), 422, "transition target from another workflow")
	denied := f.call(admin, "PATCH", "/items/"+item, map[string]any{"status_id": done}, "If-Match", etag)
	if denied.status != 422 || denied.body["errors"] == nil {
		t.Fatalf("locked transition must be rejected with a field error: %d %v", denied.status, denied.body)
	}
	if v := f.queryID("SELECT version::text FROM items WHERE id=$1", item); v != "1" {
		t.Fatalf("rejected change must not touch the item: version %s", v)
	}
	ok := f.call(admin, "PATCH", "/items/"+item, map[string]any{"status_id": progress}, "If-Match", etag)
	f.want(ok, 200, "allowed transition")
	// An unlocked status (In Progress) can go anywhere.
	f.want(f.call(admin, "PATCH", "/items/"+item, map[string]any{"status_id": done}, "If-Match", ok.header.Get("ETag")), 200, "unlocked status leaves freely")

	// Auto-assign: the person who moves the Item into the status becomes an assignee.
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('space',$1,'user',$2,'editor')", space, member.id)
	f.want(f.call(admin, "PATCH", "/statuses/"+progress, map[string]any{"auto_assign": true}), 200, "enable auto-assign")
	second, secondTag := newItem()
	f.want(f.call(admin, "PATCH", "/statuses/"+newDefault, map[string]any{"lock_transitions": false}), 200, "unlock New")
	moved := f.call(member, "PATCH", "/items/"+second, map[string]any{"status_id": progress}, "If-Match", secondTag)
	f.want(moved, 200, "member moves item")
	if n := f.queryID("SELECT count(*)::text FROM item_assignees WHERE item_id=$1 AND user_id=$2", second, member.id); n != "1" {
		t.Fatalf("auto-assign must add the actor as assignee, got %s", n)
	}
	if n := f.queryID("SELECT (changes ? 'assignee_ids')::text FROM activity WHERE item_id=$1 ORDER BY id DESC LIMIT 1", second); n != "true" {
		t.Fatalf("auto-assign must show in the activity log, got %s", n)
	}

	// Deleting a used status needs a place to move its items.
	f.want(f.call(admin, "DELETE", "/statuses/"+progress, nil), 409, "status in use needs move_to")
	f.want(f.call(admin, "DELETE", "/statuses/"+progress+"?move_to="+progress, nil), 422, "cannot move to itself")
	f.want(f.call(admin, "DELETE", "/statuses/"+progress+"?move_to="+newStatus, nil), 422, "move_to in another workflow")
	f.want(f.call(admin, "DELETE", "/statuses/"+progress+"?move_to="+done, nil), 200, "delete with move")
	if got := f.queryID("SELECT status_id::text FROM items WHERE id=$1", second); got != done {
		t.Fatalf("items must move to the chosen status: %s", got)
	}
	if v := f.queryID("SELECT (completed_at IS NOT NULL)::text FROM items WHERE id=$1", second); v != "true" {
		t.Fatalf("moving into a Completed status must set completed_at")
	}

	// Workflows: the default and used ones stay; an unused one can go.
	f.want(f.call(admin, "DELETE", "/workflows/"+f.queryID("SELECT id::text FROM workflows WHERE is_default"), nil), 409, "default workflow")
	f.want(f.call(admin, "DELETE", "/workflows/"+wfID, nil), 204, "unused workflow")
	f.want(f.call(admin, "DELETE", "/workflows/"+wfID, nil), 404, "already deleted")
	used := f.call(admin, "POST", "/workflows", map[string]any{"name": "Used"})
	f.exec("INSERT INTO items(type,title,default_workflow_id,created_by) VALUES ('project','P',$1,$2)", used.body["id"], admin.id)
	f.want(f.call(admin, "DELETE", "/workflows/"+used.body["id"].(string), nil), 409, "workflow used by a container")
}
