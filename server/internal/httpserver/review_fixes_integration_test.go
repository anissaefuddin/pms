package httpserver_test

import (
	"context"
	"testing"
	"time"
)

// DEF-032 (review H1): assignment rights cannot be carried from one Space into another by moving the work.
func TestMovingWorkDoesNotCarryAssignmentRights(t *testing.T) {
	c := newAssignCast(t)
	f := c.f
	own := f.call(c.editor, "POST", "/spaces", map[string]any{"name": "Mine"})
	f.want(own, 201, "own space")
	mine := own.body["id"].(string)
	f.share(mine, "user", c.teammate.id, "editor")
	in := func(space string) map[string]any {
		return map[string]any{"locations": []map[string]any{{"space_id": space}}}
	}
	task := f.call(c.full, "POST", "/items", map[string]any{"type": "task", "title": "T", "locations": []map[string]string{{"space_id": c.space}}})
	id := task.body["id"].(string)
	f.want(f.call(c.editor, "PUT", "/items/"+id+"/locations", in(mine)), 200, "move an unassigned task out")
	version := f.queryID("SELECT version::text FROM items WHERE id=$1", id)
	f.want(f.call(c.editor, "PATCH", "/items/"+id, map[string]any{"assignee_ids": []string{c.teammate.id}}, "If-Match", `"`+version+`"`), 200, "assign where Full")
	f.want(f.call(c.editor, "PUT", "/items/"+id+"/locations", in(c.space)), 403, "and cannot bring the assignment back")

	// The same through a parent: the assignee sits on a subtask of what is moved.
	parent := f.call(c.editor, "POST", "/items", map[string]any{"type": "task", "title": "Parent", "locations": []map[string]string{{"space_id": mine}}})
	pid := parent.body["id"].(string)
	f.want(f.call(c.editor, "POST", "/items", map[string]any{"type": "task", "title": "Child", "assignee_ids": []string{c.teammate.id},
		"locations": []map[string]any{{"space_id": mine, "parent_item_id": pid}}}), 201, "assigned child")
	f.want(f.call(c.editor, "PUT", "/items/"+pid+"/locations", in(c.space)), 403, "assigned subtask comes along")

	// Still allowed: someone with Full on the destination, and an Editor moving work that is only theirs.
	self := f.call(c.editor, "POST", "/items", map[string]any{"type": "task", "title": "Self", "assignee_ids": []string{c.editor.id}, "locations": []map[string]string{{"space_id": mine}}})
	f.want(f.call(c.editor, "PUT", "/items/"+self.body["id"].(string)+"/locations", in(c.space)), 200, "own work moves")
	f.share(mine, "user", c.full.id, "full")
	f.want(f.call(c.full, "PUT", "/items/"+id+"/locations", in(c.space)), 200, "Full on the destination moves assigned work")
}

// DEF-033 (review H2): a status change under an already recorded workflow revision waits for nobody.
func TestStatusChangesDoNotQueueOnTheWorkflow(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.spaceWithoutAdmins("Revisions")
	f.share(space, "user", admin.id, "full")
	task := func(title string) string {
		made := f.call(admin, "POST", "/items", map[string]any{"type": "task", "title": title, "locations": []map[string]string{{"space_id": space}}})
		f.want(made, 201, title)
		return made.body["id"].(string)
	}
	first, second := task("one"), task("two")
	status := f.queryID("SELECT s.id::text FROM statuses s JOIN workflows w ON w.id=s.workflow_id WHERE w.is_default ORDER BY s.position LIMIT 1")
	const log = `INSERT INTO activity(item_id,actor_id,action,changes) VALUES($1,$2,'update',jsonb_build_object('status_id',jsonb_build_object('to',$3::text)))`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	held, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Rollback(context.Background())
	if _, err = held.Exec(ctx, log, first, admin.id, status); err != nil {
		t.Fatal(err)
	}
	other, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Rollback(context.Background())
	if _, err = other.Exec(ctx, "SET LOCAL lock_timeout='1s'"); err == nil {
		_, err = other.Exec(ctx, log, second, admin.id, status)
	}
	if err != nil {
		t.Fatalf("a second change waited for the first transaction: %v", err)
	}
}

// DEF-034/035 (review M2, M3): a project in two Spaces shows nothing of the unseen one, and is not called complete.
func TestProjectCostAcrossSpaces(t *testing.T) {
	f := newFixture(t)
	finance, manager := f.unlinkedAdmin(), f.person("regular")
	seen, hidden := f.spaceWithoutAdmins("Seen"), f.spaceWithoutAdmins("Hidden")
	f.share(seen, "user", finance.id, "full")
	f.share(hidden, "user", manager.id, "full")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','P',$1) RETURNING id::text", finance.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A'),($1,$3,'B')", project, seen, hidden)
	budget := f.call(manager, "POST", "/spaces/"+hidden+"/custom-fields", map[string]any{"name": "Hidden budget", "type": "currency", "config": map[string]any{"currency": "IDR", "project_budget": true}})
	f.want(budget, 201, "budget in the hidden Space")
	f.exec("UPDATE items SET fields=jsonb_build_object($2::text,500) WHERE id=$1", project, budget.body["id"])
	got := f.call(finance, "GET", "/items/"+project+"/time-summary", nil)
	f.want(got, 200, "summary")
	if got.body["cost_status"] != "partial" || got.body["actual_cost_minor"] != nil || len(got.body["budgets"].([]any)) != 0 {
		t.Fatalf("hidden placement: %v", got.body)
	}
}

// DEF-039: turning the public link on carries no version, so a serialization conflict (a layout save committing at the same
// moment) is retried instead of telling the person that the dashboard changed.
func TestPublicLinkRetriesASerializationConflict(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	made := f.call(admin, "POST", "/dashboards", map[string]any{"name": "Race"})
	f.want(made, 201, "dashboard")
	id := made.body["id"].(string)
	f.exec("CREATE SEQUENCE link_fault")
	f.exec(`CREATE FUNCTION link_fault() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
IF nextval('link_fault') = 1 THEN RAISE serialization_failure; END IF; RETURN NEW; END $$`)
	f.exec("CREATE TRIGGER link_fault BEFORE INSERT ON dashboard_public_links FOR EACH ROW EXECUTE FUNCTION link_fault()")
	expires := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	link := f.call(admin, "PUT", "/dashboards/"+id+"/public-link", map[string]any{"expires_at": expires})
	f.want(link, 200, "link after one conflict")
	if link.body["token"] == nil {
		t.Fatalf("no token: %v", link.body)
	}
}

// DEF-040: a new comment carries no version either, so a serialization conflict is retried, and leaves exactly one comment.
func TestNewCommentRetriesASerializationConflict(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.spaceWithoutAdmins("Comments")
	f.share(space, "user", admin.id, "full")
	made := f.call(admin, "POST", "/items", map[string]any{"type": "task", "title": "T", "locations": []map[string]string{{"space_id": space}}})
	f.want(made, 201, "task")
	id := made.body["id"].(string)
	f.exec("CREATE SEQUENCE comment_fault")
	f.exec(`CREATE FUNCTION comment_fault() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
IF nextval('comment_fault') = 1 THEN RAISE serialization_failure; END IF; RETURN NEW; END $$`)
	f.exec("CREATE TRIGGER comment_fault BEFORE INSERT ON comments FOR EACH ROW EXECUTE FUNCTION comment_fault()")
	body := map[string]any{"body": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Hello"}}}}}}
	f.want(f.call(admin, "POST", "/items/"+id+"/comments", body), 201, "comment after one conflict")
	if n := f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1", id); n != "1" {
		t.Fatalf("comments: %s", n)
	}
}
