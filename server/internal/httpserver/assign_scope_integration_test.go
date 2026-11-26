package httpserver_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// DEF-041 (DEC-10): assignment rights belong to a place. A folder the actor controls gives no say over work that is also, or goes
// back to, somewhere else, and Move is never another way to make an assignment.
type scopeCast struct {
	assignCast
	g, plain1, plain2 string // g is a folder the Editor has Full on; plain1 and plain2 carry no shares of their own
}

func newScopeCast(t *testing.T) scopeCast {
	c := scopeCast{assignCast: newAssignCast(t)}
	c.g, c.plain1, c.plain2 = c.folder("G", ""), c.folder("Plain 1", ""), c.folder("Plain 2", "")
	c.f.share2(c.g, c.editor.id, "full")
	return c
}

func (c scopeCast) at(parent string) map[string]any {
	if parent == "" {
		return map[string]any{"space_id": c.space}
	}
	return map[string]any{"space_id": c.space, "parent_item_id": parent}
}

func (c scopeCast) make(kind, title, parent string, assignees ...string) string {
	body := map[string]any{"type": kind, "title": title, "locations": []map[string]any{c.at(parent)}}
	if len(assignees) > 0 {
		body["assignee_ids"] = assignees
	}
	made := c.f.call(c.full, "POST", "/items", body)
	c.f.want(made, 201, title)
	return made.body["id"].(string)
}

func (c scopeCast) folder(title, parent string) string { return c.make("folder", title, parent) }

func (c scopeCast) move(p *person, item string, want int, what string, parents ...string) {
	c.f.t.Helper()
	locations := []map[string]any{}
	for _, parent := range parents {
		locations = append(locations, c.at(parent))
	}
	c.f.want(c.f.call(p, "PUT", "/items/"+item+"/locations", map[string]any{"locations": locations}), want, what)
}

func (c scopeCast) assign(p *person, item string, want int, what string, ids ...string) {
	c.f.t.Helper()
	version := c.f.queryID("SELECT version::text FROM items WHERE id=$1", item)
	c.f.want(c.f.call(p, "PATCH", "/items/"+item, map[string]any{"assignee_ids": ids}, "If-Match", `"`+version+`"`), want, what)
}

// state is everything a refused Move must leave alone.
func (c scopeCast) state(items ...string) string {
	var out []string
	for _, id := range items {
		out = append(out, c.f.queryID(`SELECT i.version||'|'||COALESCE((SELECT string_agg(COALESCE(parent_item_id::text,'root'),',' ORDER BY 1) FROM item_locations WHERE item_id=i.id AND deleted_root_id IS NULL),'')
||'|'||COALESCE((SELECT string_agg(user_id::text,',' ORDER BY user_id) FROM item_assignees WHERE item_id=i.id),'')
||'|'||(SELECT count(*) FROM activity WHERE item_id=i.id)||'|'||(SELECT count(*) FROM notifications WHERE item_id=i.id) FROM items i WHERE i.id=$1`, id))
	}
	return strings.Join(out, ";")
}

func TestAFolderGivesNoAssignmentSayOutsideItself(t *testing.T) {
	c := newScopeCast(t)

	// Into the folder, assign there, and back.
	task := c.make("task", "A", "")
	c.move(c.editor, task, 200, "into G", c.g)
	c.assign(c.editor, task, 200, "Full on G assigns inside G", c.teammate.id)
	before := c.state(task)
	c.move(c.editor, task, 403, "and cannot take the assignment out", "")
	c.move(c.editor, task, 403, "nor into another folder", c.plain1)
	c.move(c.editor, task, 403, "nor keep both places", c.g, "")
	if got := c.state(task); got != before {
		t.Fatalf("a refused move changed something:\n%s\n%s", before, got)
	}

	// A second place under the folder, without leaving the first.
	second := c.make("task", "B", "")
	c.move(c.editor, second, 200, "add a place under G", "", c.g)
	c.assign(c.editor, second, 403, "the Space root still says Editor", c.teammate.id)

	// The same through the parent: a folder placed a second time under G.
	holder := c.folder("Holder", "")
	child := c.make("task", "C", holder)
	c.move(c.editor, holder, 200, "folder also under G", "", c.g)
	c.assign(c.editor, child, 403, "its tasks are not under G only", c.teammate.id)
	c.f.want(c.f.call(c.editor, "POST", "/items", map[string]any{"type": "task", "title": "D", "assignee_ids": []string{c.teammate.id},
		"locations": []map[string]any{c.at(holder)}}), 403, "nor on create inside it")
	c.assign(c.full, child, 200, "Full on the Space still assigns", c.teammate.id)
}

func TestMoveKeepsOrChecksTheAssignmentScope(t *testing.T) {
	c := newScopeCast(t)
	f := c.f

	c.move(c.editor, c.make("task", "unassigned", ""), 200, "no assignee", c.plain1)
	c.move(c.editor, c.make("task", "self", "", c.editor.id), 200, "only self, into G", c.g)
	shared := c.make("task", "others", c.plain1, c.teammate.id, c.read.id)
	c.move(c.editor, shared, 200, "same scope: another plain folder", c.plain2)
	c.move(c.editor, shared, 200, "same scope: the root", "")
	// DEC-10 as clarified (26 Nov): Full at the destination alone does not move others' work out of where it is.
	c.move(c.editor, shared, 403, "Full on the destination only: into G", c.g)
	c.move(c.full, shared, 200, "Full on both: into G", c.g)
	c.move(c.editor, shared, 403, "Full on the source is not Full on the destination", c.plain1)
	c.move(c.full, shared, 200, "Full on the Space moves it back", "")

	parent := c.make("task", "parent", "")
	sub := c.make("task", "sub", parent, c.teammate.id)
	own := f.call(c.editor, "POST", "/spaces", map[string]any{"name": "Elsewhere"})
	f.want(own, 201, "own space")
	elsewhere := map[string]any{"locations": []map[string]any{{"space_id": own.body["id"]}}}
	before := c.state(parent, sub)
	f.want(f.call(c.editor, "PUT", "/items/"+parent+"/locations", elsewhere), 403, "an assigned subtask is not taken away by Full elsewhere")
	if got := c.state(parent, sub); got != before {
		t.Fatalf("refused, yet changed:\n%s\n%s", before, got)
	}
	f.share(own.body["id"].(string), "user", c.full.id, "full")
	f.want(f.call(c.full, "PUT", "/items/"+parent+"/locations", elsewhere), 200, "Full on both sides moves it")
	f.want(f.call(c.editor, "PUT", "/items/"+parent+"/locations", map[string]any{"locations": []map[string]any{c.at("")}}), 403, "nor does an Editor of the destination bring it back")
	f.want(f.call(c.outsiderAdmin, "PUT", "/items/"+parent+"/locations", elsewhere), 404, "no grant")

	editorAdmin := c.make("task", "ws admin", c.g, c.teammate.id)
	c.f.share2(c.g, c.adminEditor.id, "full")
	c.move(c.adminEditor, editorAdmin, 403, "a workspace admin with Editor has no extra say", "")

	revoked := c.make("task", "revoked", "", c.teammate.id)
	f.exec("UPDATE shares SET role='read' WHERE target_id=$1 AND principal_id=$2", c.space, c.editor.id)
	c.move(c.editor, revoked, 403, "permission lost before the move", c.plain1)
}

// Restoring a task whose folder is gone puts it at the Space root: the same boundary applies.
func TestRestoreDoesNotCarryAssignmentOutOfAFolder(t *testing.T) {
	c := newScopeCast(t)
	f := c.f
	box := c.folder("Box", c.g)
	task := c.make("task", "in box", box, c.teammate.id)
	f.want(f.call(c.full, "DELETE", "/items/"+task, nil, "If-Match", `"`+f.queryID("SELECT version::text FROM items WHERE id=$1", task)+`"`), 204, "delete task")
	f.want(f.call(c.full, "DELETE", "/items/"+box, nil, "If-Match", `"`+f.queryID("SELECT version::text FROM items WHERE id=$1", box)+`"`), 204, "delete folder")
	// Restoring needs Delete, which the built-in roles only give together with assignment; a custom role can give it alone.
	cleaner := f.person("regular")
	role := f.queryID("INSERT INTO custom_roles(name,caps_mask) VALUES ('Cleaner',127) RETURNING id::text")
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,custom_role_id) VALUES('space',$1,'user',$2,'read',$3)", c.space, cleaner.id, role)
	f.want(f.call(cleaner, "POST", "/recycle-bin/"+task+"/restore", nil), 403, "Delete without Share cannot bring the assignment to the root")
	f.want(f.call(c.full, "POST", "/recycle-bin/"+task+"/restore", nil), 200, "Full on the Space can")
}

func TestMoveAndAssignAtTheSameTime(t *testing.T) {
	c := newScopeCast(t)
	for i := range 10 {
		task := c.make("task", fmt.Sprintf("race %d", i), "")
		c.move(c.editor, task, 200, "into G", c.g)
		version := c.f.queryID("SELECT version::text FROM items WHERE id=$1", task)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.f.call(c.editor, "PATCH", "/items/"+task, map[string]any{"assignee_ids": []string{c.teammate.id}}, "If-Match", `"`+version+`"`)
		}()
		go func() {
			defer wg.Done()
			c.f.call(c.editor, "PUT", "/items/"+task+"/locations", map[string]any{"locations": []map[string]any{c.at("")}})
		}()
		wg.Wait()
		bad := c.f.queryID(`SELECT (EXISTS(SELECT 1 FROM item_locations WHERE item_id=$1 AND parent_item_id IS NULL AND deleted_root_id IS NULL)
AND EXISTS(SELECT 1 FROM item_assignees WHERE item_id=$1))::text`, task)
		if bad != "false" {
			t.Fatalf("round %d: the task ended at the root with an assignment the Editor could not make there", i)
		}
	}
}

func (f *fixture) share2(itemID, userID, role string) {
	f.t.Helper()
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,'user',$2,$3::share_role) ON CONFLICT (target_type,target_id,principal_type,principal_id) DO UPDATE SET role=EXCLUDED.role", itemID, userID, role)
}

// DEF-041: a folder cannot be turned into a lasting right on the task by sharing it while it is there.
func TestAFolderCannotBeTurnedIntoADirectShare(t *testing.T) {
	c := newScopeCast(t)
	f := c.f
	grant := func(item string) reply {
		got := f.call(c.editor, "GET", "/items/"+item+"/shares", nil)
		return f.call(c.editor, "PUT", "/items/"+item+"/shares", map[string]any{"shares": []map[string]any{{"principal_type": "user", "principal_id": c.editor.id, "role": "full"}}}, "If-Match", got.header.Get("ETag"))
	}
	// Two places at once: the union of rights must not allow granting oneself Full.
	both := c.make("task", "both", "")
	c.move(c.editor, both, 200, "add a place under G", "", c.g)
	f.want(grant(both), 403, "not while it is also at the root")

	// Only in the folder, where sharing is allowed; the share must not leave with the task.
	only := c.make("task", "only", "")
	c.move(c.editor, only, 200, "into G", c.g)
	f.want(grant(only), 200, "inside G the folder's Full may share")
	before := c.state(only)
	c.move(c.editor, only, 403, "the share does not travel out", "")
	c.move(c.editor, only, 403, "nor gain a second place", c.g, "")
	if got := c.state(only); got != before {
		t.Fatalf("a refused move changed something:\n%s\n%s", before, got)
	}
	c.move(c.full, only, 200, "Full on the Space may move it", "")
}

// DEF-044 (final review B1): creating something with initial shares hands out rights too.
func TestCreateWithSharesCannotDelegateFromAFolder(t *testing.T) {
	c := newScopeCast(t)
	f := c.f
	holder := c.folder("Holder", "")
	c.move(c.editor, holder, 200, "folder also under G", "", c.g)
	create := func(p *person, parent, role string) reply {
		return f.call(p, "POST", "/items", map[string]any{"type": "folder", "title": "P", "locations": []map[string]any{c.at(parent)},
			"shares": []map[string]any{{"principal_type": "user", "principal_id": c.editor.id, "role": role}}})
	}
	before := f.queryID("SELECT count(*)||'/'||(SELECT count(*) FROM shares)||'/'||(SELECT count(*) FROM notifications) FROM items")
	f.want(create(c.editor, holder, "full"), 403, "a Full share from a folder placed in two ways")
	if after := f.queryID("SELECT count(*)||'/'||(SELECT count(*) FROM shares)||'/'||(SELECT count(*) FROM notifications) FROM items"); after != before {
		t.Fatalf("refused, yet something was written: %s -> %s", before, after)
	}
	f.want(create(c.editor, holder, "editor"), 201, "a share that does not include sharing")
	f.want(create(c.editor, c.g, "full"), 201, "inside G only, where the folder's Full covers every way up")
}

// DEF-045: an invitation grants no more than the inviter could grant by sharing (D-17.1).
func TestInvitationSharesNeedTheInvitersOwnRight(t *testing.T) {
	f := newFixture(t)
	outsider, owner := f.unlinkedAdmin(), f.unlinkedAdmin()
	space := f.spaceWithoutAdmins("Closed")
	f.share(space, "user", owner.id, "full")
	invite := func(p *person, email, role string) reply {
		return f.call(p, "POST", "/users/invitations", map[string]any{"email": email, "user_type": "regular", "share": []map[string]any{{"target_type": "space", "target_id": space, "role": role}}})
	}
	for _, role := range []string{"full", "read"} {
		f.want(invite(outsider, role+"@example.com", role), 403, "admin without access invites into the Space as "+role)
	}
	if n := f.queryID("SELECT count(*)::text FROM users WHERE email LIKE '%@example.com' AND email IN ('full@example.com','read@example.com')"); n != "0" {
		t.Fatalf("refused invitations left accounts: %s", n)
	}
	f.want(invite(owner, "guest@example.com", "editor"), 201, "admin with Full on it")
	f.want(f.call(outsider, "POST", "/users/invitations", map[string]any{"email": "plain@example.com", "user_type": "regular"}), 201, "no share, no condition")
}
