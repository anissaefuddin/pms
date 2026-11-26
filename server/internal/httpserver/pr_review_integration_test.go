package httpserver_test

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// DEC-10 as clarified: Full at the destination alone does not let someone take others' work away from where it is.
func TestFullAtTheDestinationDoesNotTransferOthersWork(t *testing.T) {
	c := newScopeCast(t)
	f := c.f
	task := c.make("task", "theirs", "", c.teammate.id)
	own := f.call(c.editor, "POST", "/spaces", map[string]any{"name": "Mine"})
	f.want(own, 201, "own space")
	mine := map[string]any{"space_id": own.body["id"]}
	put := func(p *person, locations ...map[string]any) reply {
		return f.call(p, "PUT", "/items/"+task+"/locations", map[string]any{"locations": locations})
	}
	before := c.state(task)
	f.want(put(c.editor, mine), 403, "into a Space of one's own")
	f.want(put(c.editor, c.at(""), mine), 403, "nor added as a second place")
	c.move(c.editor, task, 403, "nor into the folder one is Full on", c.g)
	if got := c.state(task); got != before {
		t.Fatalf("refused, yet changed:\n%s\n%s", before, got)
	}
	// Removing the place the others are in is a transfer too.
	both := c.make("task", "two places", "", c.teammate.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'M')", both, own.body["id"])
	f.want(f.call(c.editor, "PUT", "/items/"+both+"/locations", map[string]any{"locations": []map[string]any{mine}}), 403, "remove the source place")
	// Same scope stays free; someone who may assign at the source and the destination may transfer.
	c.move(c.editor, task, 200, "plain folder to plain folder", c.plain1)
	f.share(own.body["id"].(string), "user", c.full.id, "full")
	f.want(put(c.full, mine), 200, "Full on both sides")
}

// DEF-047: an invitation obeys the locked-Space rule like a direct share.
func TestInvitationCannotOpenAnItemOfALockedSpace(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.spaceWithoutAdmins("Locked")
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','P',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", item, space)
	f.share2(item, admin.id, "full") // may share the item, but does not reach the Space
	f.exec("UPDATE spaces SET locked=true WHERE id=$1", space)
	outsider := f.person("regular")
	got := f.call(admin, "GET", "/items/"+item+"/shares", nil)
	direct := f.call(admin, "PUT", "/items/"+item+"/shares", map[string]any{"shares": []map[string]any{
		{"principal_type": "user", "principal_id": admin.id, "role": "full"}, {"principal_type": "user", "principal_id": outsider.id, "role": "read"}}}, "If-Match", got.header.Get("ETag"))
	f.want(direct, 403, "direct share to an outsider")
	invite := f.call(admin, "POST", "/users/invitations", map[string]any{"email": "locked@example.com", "user_type": "regular",
		"share": []map[string]any{{"target_type": "item", "target_id": item, "role": "read"}}})
	f.want(invite, 403, "the same through an invitation")
	if n := f.queryID("SELECT count(*)::text FROM users WHERE email='locked@example.com'"); n != "0" {
		t.Fatalf("account left behind: %s", n)
	}
}

// DEF-048: purging the folder of a binned task and then restoring the task must not drop it at the root with its assignment.
func TestPurgedFolderThenRestoreKeepsTheBoundary(t *testing.T) {
	c := newScopeCast(t)
	f := c.f
	box := c.folder("Box", c.g)
	task := c.make("task", "in box", box, c.teammate.id)
	del := func(id string) {
		f.want(f.call(c.full, "DELETE", "/items/"+id, nil, "If-Match", `"`+f.queryID("SELECT version::text FROM items WHERE id=$1", id)+`"`), 204, "delete")
	}
	// Emptying the bin removes only what the caller may delete: this one may delete the folder, not the task.
	purger := f.person("regular")
	f.share(c.space, "user", purger.id, "read")
	f.share2(box, purger.id, "full")
	del(task)
	del(box)
	f.want(f.call(purger, "DELETE", "/recycle-bin", nil), 200, "empty the bin")
	if got := f.queryID("SELECT (SELECT count(*) FROM items WHERE id=$1)||'/'||(SELECT count(*) FROM items WHERE id=$2)", box, task); got != "0/1" {
		t.Fatalf("setup: folder gone and task kept, got %s", got)
	}
	cleaner := f.person("regular")
	role := f.queryID("INSERT INTO custom_roles(name,caps_mask) VALUES ('Cleaner',127) RETURNING id::text")
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,custom_role_id) VALUES('space',$1,'user',$2,'read',$3)", c.space, cleaner.id, role)
	f.want(f.call(cleaner, "POST", "/recycle-bin/"+task+"/restore", nil), 403, "Delete without Share after the folder is gone")
	if n := f.queryID("SELECT count(*)::text FROM items WHERE id=$1 AND deleted_at IS NOT NULL", task); n != "1" {
		t.Fatalf("refused restore changed the item")
	}
	f.want(f.call(c.full, "POST", "/recycle-bin/"+task+"/restore", nil), 200, "Full on the Space can")
}

// DEF-049: an assignment cannot commit after the Full access it relies on has been taken away.
func TestAssignmentRightCannotBeRevokedUnderACreate(t *testing.T) {
	f := newFixture(t)
	lead, mate := f.person("regular"), f.person("regular")
	space := f.spaceWithoutAdmins("Race")
	f.share(space, "user", lead.id, "full")
	f.share(space, "user", mate.id, "editor")
	ctx := context.Background()
	holder, err := f.pool.Begin(ctx) // writes to items wait; reads, and so the permission checks, do not
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err = holder.Exec(ctx, "LOCK TABLE items IN EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	done := make(chan reply, 1)
	go func() {
		done <- f.call(lead, "POST", "/items", map[string]any{"type": "task", "title": "T", "assignee_ids": []string{mate.id}, "locations": []map[string]string{{"space_id": space}}})
	}()
	for i := 0; ; i++ {
		if f.queryID("SELECT count(*)::text FROM pg_locks WHERE NOT granted AND relation='items'::regclass") != "0" {
			break
		}
		if i > 500 {
			t.Fatal("the create never reached its write")
		}
		time.Sleep(10 * time.Millisecond)
	}
	revoke, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer revoke.Rollback(ctx)
	_, err = revoke.Exec(ctx, "SET LOCAL lock_timeout='500ms'")
	if err == nil {
		_, err = revoke.Exec(ctx, "UPDATE shares SET role='editor' WHERE target_id=$1 AND principal_id=$2", space, lead.id)
	}
	revoked := err == nil && revoke.Commit(ctx) == nil
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	got := <-done
	if revoked && got.status == http.StatusCreated {
		t.Fatalf("Full was revoked and committed, yet the assignment still committed afterwards")
	}
}

// DEF-049, the edit path: the same for assigning on an existing Item.
func TestAssignmentRightCannotBeRevokedUnderAnEdit(t *testing.T) {
	f := newFixture(t)
	lead, mate := f.person("regular"), f.person("regular")
	space := f.spaceWithoutAdmins("Race edit")
	f.share(space, "user", lead.id, "full")
	f.share(space, "user", mate.id, "editor")
	made := f.call(lead, "POST", "/items", map[string]any{"type": "task", "title": "T", "locations": []map[string]string{{"space_id": space}}})
	f.want(made, 201, "task")
	id := made.body["id"].(string)
	ctx := context.Background()
	holder, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err = holder.Exec(ctx, "LOCK TABLE item_assignees IN EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	done := make(chan reply, 1)
	go func() {
		done <- f.call(lead, "PATCH", "/items/"+id, map[string]any{"assignee_ids": []string{mate.id}}, "If-Match", `"1"`)
	}()
	for i := 0; ; i++ {
		if f.queryID("SELECT count(*)::text FROM pg_locks WHERE NOT granted AND relation='item_assignees'::regclass") != "0" {
			break
		}
		if i > 500 {
			t.Fatal("the edit never reached its write")
		}
		time.Sleep(10 * time.Millisecond)
	}
	revoke, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer revoke.Rollback(ctx)
	_, err = revoke.Exec(ctx, "SET LOCAL lock_timeout='500ms'")
	if err == nil {
		_, err = revoke.Exec(ctx, "DELETE FROM shares WHERE target_id=$1 AND principal_id=$2", space, lead.id)
	}
	revoked := err == nil && revoke.Commit(ctx) == nil
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := <-done; revoked && got.status == http.StatusOK {
		t.Fatalf("the grant was removed and committed, yet the assignment still committed afterwards")
	}
}
