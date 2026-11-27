package httpserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/clients"
	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

// B3-03: ownership is derived by the server when an item is created (ADR §7j). These tests use the real HTTP path, real
// sessions, and the real database. Grants and assignment rules are unchanged; ownership here is integrity, not access.

// scopedSpace creates a Space with explicit ownership. The shared f.space helper is left unchanged, so existing tests keep
// their current fixture; only the tests in this file use explicit ownership.
func (f *fixture) scopedSpace(name, scope, client string) string {
	f.t.Helper()
	var c any
	if client != "" {
		c = client
	}
	return f.queryID("INSERT INTO spaces(name,kind,client_id,scope_state) VALUES ($1,'shared',$2::uuid,$3) RETURNING id::text", name, c, scope)
}

func (f *fixture) personalSpace(owner *person) string {
	f.t.Helper()
	return f.queryID("SELECT id::text FROM spaces WHERE owner_id=$1 AND kind='personal'", owner.id)
}

// itemBody is a POST /items body; parent is optional.
func itemBody(kind, title, space string, parent string) map[string]any {
	loc := map[string]any{"space_id": space}
	if parent != "" {
		loc["parent_item_id"] = parent
	}
	return map[string]any{"type": kind, "title": title, "locations": []map[string]any{loc}}
}

// ownershipOf returns "scope/client" for a stored item, with "-" for NULL.
func (f *fixture) ownershipOf(itemID string) string {
	f.t.Helper()
	var scope string
	var client *string
	if err := f.pool.QueryRow(f.ctx, "SELECT scope_state, client_id::text FROM items WHERE id=$1::uuid", itemID).Scan(&scope, &client); err != nil {
		f.t.Fatal(err)
	}
	c := "-"
	if client != nil {
		c = *client
	}
	return scope + "/" + c
}

func (f *fixture) itemCount(title string) int {
	f.t.Helper()
	return f.count("SELECT count(*) FROM items WHERE title=$1", title)
}

// waitForLockWait polls the database until some session waits on a lock. It checks database state, not a duration.
func waitForLockWait(t *testing.T, f *fixture) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if f.count(`SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND datname=current_database()`) > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no session started waiting on a lock")
}

// CRT-01..CRT-03, CRT-05, CRT-06, CRT-12, CRT-13: the derived ownership of root items and their persisted row.
func TestRootItemsTakeTheOwnershipOfTheirSpace(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	creator := f.person("regular")
	client := f.createClient(admin, "CRT-A")["id"].(string)
	f.addMember(admin, client, creator) // B3-04: the creator is an active member of this Customer
	spaceA := f.scopedSpace("Client A", "client", client)
	internal := f.scopedSpace("Internal", "internal", "")
	f.share(spaceA, "user", creator.id, "full")
	f.share(internal, "user", creator.id, "full")
	personal := f.personalSpace(creator)

	cases := []struct {
		id, kind, title, space, want string
	}{
		{"CRT-01", "project", "crt01-project", spaceA, "client/" + client},
		{"CRT-02", "folder", "crt02-folder", spaceA, "client/" + client},
		{"CRT-03", "task", "crt03-task", spaceA, "client/" + client},
		{"CRT-05", "task", "crt05-internal", internal, "internal/-"},
		{"CRT-06", "task", "crt06-personal", personal, "internal/-"},
	}
	for _, tc := range cases {
		t.Run(tc.id+" "+tc.kind, func(t *testing.T) {
			f := f.on(t)
			r := f.call(creator, "POST", "/items", itemBody(tc.kind, tc.title, tc.space, ""))
			f.want(r, http.StatusCreated, tc.id+" create")
			id := r.body["id"].(string)
			if got := f.ownershipOf(id); got != tc.want {
				t.Fatalf("%s ownership = %s, want %s", tc.id, got, tc.want)
			}
		})
	}

	t.Run("CRT-12 owner_id keeps its user meaning and CRT-13 the initial location is persisted with the item", func(t *testing.T) {
		f := f.on(t)
		r := f.call(creator, "POST", "/items", itemBody("task", "crt13-persist", spaceA, ""))
		f.want(r, http.StatusCreated, "create")
		id := r.body["id"].(string)
		var createdBy string
		var owner *string
		if err := f.pool.QueryRow(f.ctx, "SELECT created_by::text, owner_id::text FROM items WHERE id=$1::uuid", id).Scan(&createdBy, &owner); err != nil {
			t.Fatal(err)
		}
		if createdBy != creator.id || owner != nil {
			t.Fatalf("created_by=%s owner_id=%v; owner_id must stay a user reference and stay unset here", createdBy, owner)
		}
		if n := f.count("SELECT count(*) FROM item_locations WHERE item_id=$1::uuid AND space_id=$2::uuid", id, spaceA); n != 1 {
			t.Fatalf("initial location rows = %d, want 1", n)
		}
	})
}

// CRT-04, CRT-08, CRT-09, CRT-10: child ownership and the parent and destination rules.
func TestChildItemsInheritTheParentOwnership(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	creator := f.person("regular")
	client := f.createClient(admin, "CRT-B")["id"].(string)
	f.addMember(admin, client, creator)
	spaceA := f.scopedSpace("Client A", "client", client)
	internal := f.scopedSpace("Internal", "internal", "")
	f.share(spaceA, "user", creator.id, "full")
	f.share(internal, "user", creator.id, "full")
	personal := f.personalSpace(creator)

	parentRes := f.call(creator, "POST", "/items", itemBody("project", "crt04-parent", spaceA, ""))
	f.want(parentRes, http.StatusCreated, "client parent")
	parent := parentRes.body["id"].(string)

	t.Run("CRT-04 child in the same client Space takes the parent's ownership", func(t *testing.T) {
		f := f.on(t)
		r := f.call(creator, "POST", "/items", itemBody("task", "crt04-child", spaceA, parent))
		f.want(r, http.StatusCreated, "child")
		if got := f.ownershipOf(r.body["id"].(string)); got != "client/"+client {
			t.Fatalf("child ownership = %s, want the parent's client/%s", got, client)
		}
		if got := f.ownershipOf(parent); got != "client/"+client {
			t.Fatalf("parent ownership changed to %s", got)
		}
	})

	t.Run("CRT-08 an unresolved parent refuses a child", func(t *testing.T) {
		f := f.on(t)
		unresolved := f.queryID("INSERT INTO items(type,title,created_by,scope_state) VALUES ('project','crt08-unresolved',$1,'unresolved') RETURNING id::text", creator.id)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'a')", unresolved, internal)
		r := f.call(creator, "POST", "/items", itemBody("task", "crt08-child", internal, unresolved))
		f.want(r, http.StatusUnprocessableEntity, "unresolved parent")
		if f.itemCount("crt08-child") != 0 {
			t.Fatal("a refused child was written")
		}
	})

	t.Run("CRT-09 a parent and destination with different ownership are refused", func(t *testing.T) {
		f := f.on(t)
		// A child needs its parent placed in the destination Space. That placement is the cross-table state (ADR §7i) that
		// later mutation paths must prevent; it is written here directly so the create guard is tested on its own.
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'zz')", parent, internal)
		r := f.call(creator, "POST", "/items", itemBody("task", "crt09-child", internal, parent))
		f.want(r, http.StatusUnprocessableEntity, "mismatch")
		if f.itemCount("crt09-child") != 0 {
			t.Fatal("a mismatched child was written")
		}
	})

	t.Run("CRT-10 a client-owned parent cannot place a child in a Personal Space", func(t *testing.T) {
		f := f.on(t)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'zz')", parent, personal)
		r := f.call(creator, "POST", "/items", itemBody("task", "crt10-child", personal, parent))
		f.want(r, http.StatusUnprocessableEntity, "client parent in personal")
		if f.itemCount("crt10-child") != 0 {
			t.Fatal("a client-owned child was written into a Personal Space")
		}
	})

	t.Run("CRT-16 locations must share one ownership", func(t *testing.T) {
		f := f.on(t)
		body := map[string]any{"type": "task", "title": "crt16-mixed", "locations": []map[string]any{{"space_id": spaceA}, {"space_id": internal}}}
		f.want(f.call(creator, "POST", "/items", body), http.StatusUnprocessableEntity, "mixed locations")
		if f.itemCount("crt16-mixed") != 0 {
			t.Fatal("a mixed-ownership item was written")
		}
	})
}

// CRT-07, CRT-11, CRT-14, CRT-15: refusals write nothing, and caller-supplied ownership is never accepted.
func TestRefusedCreatesLeaveNoPartialState(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	creator := f.person("regular")
	client := f.createClient(admin, "CRT-C")["id"].(string)
	f.addMember(admin, client, creator)
	spaceA := f.scopedSpace("Client A", "client", client)
	unresolved := f.scopedSpace("Legacy", "unresolved", "")
	other := f.createClient(admin, "CRT-D")["id"].(string)
	f.addMember(admin, other, creator) // an active member, so the archive rule is what refuses the create
	f.share(spaceA, "user", creator.id, "full")
	f.share(unresolved, "user", creator.id, "full")

	snapshot := func() string {
		return fmt.Sprintf("%d/%d/%d", f.count("SELECT count(*) FROM items"), f.count("SELECT count(*) FROM item_locations"), f.count("SELECT count(*) FROM item_followers"))
	}

	t.Run("CRT-07 unresolved Space refuses creation and writes nothing", func(t *testing.T) {
		f := f.on(t)
		before := snapshot()
		f.want(f.call(creator, "POST", "/items", itemBody("task", "crt07", unresolved, "")), http.StatusNotFound, "unresolved Space is not visible (B3-04 fails closed)")
		if got := snapshot(); got != before {
			t.Fatalf("refused create changed rows: %s -> %s", before, got)
		}
	})

	t.Run("CRT-11 an archived Customer receives no new work", func(t *testing.T) {
		f := f.on(t)
		f.want(f.call(admin, "POST", "/clients/"+other+"/archive", nil), http.StatusOK, "archive")
		archivedSpace := f.scopedSpace("Archived Client", "client", other)
		f.share(archivedSpace, "user", creator.id, "full")
		before := snapshot()
		r := f.call(creator, "POST", "/items", itemBody("task", "crt11", archivedSpace, ""))
		f.want(r, http.StatusConflict, "archived")
		if r.body["code"] != "customer_archived" {
			t.Fatalf("code = %v, want customer_archived", r.body["code"])
		}
		if got := snapshot(); got != before {
			t.Fatalf("refused create changed rows: %s -> %s", before, got)
		}
	})

	t.Run("CRT-14 a refusal after validation leaves no item, location, or follower", func(t *testing.T) {
		f := f.on(t)
		before := snapshot()
		body := map[string]any{"type": "task", "title": "crt14", "locations": []map[string]any{{"space_id": spaceA}}, "unknown_field": "x"}
		f.want(f.call(creator, "POST", "/items", body), http.StatusUnprocessableEntity, "unknown field")
		if got := snapshot(); got != before {
			t.Fatalf("refused create left rows: %s -> %s", before, got)
		}
	})

	t.Run("CRT-15 caller-supplied ownership is refused, not applied", func(t *testing.T) {
		f := f.on(t)
		before := snapshot()
		body := itemBody("task", "crt15", spaceA, "")
		body["client_id"] = other
		body["scope_state"] = "internal"
		f.want(f.call(creator, "POST", "/items", body), http.StatusUnprocessableEntity, "supplied ownership")
		if f.itemCount("crt15") != 0 || snapshot() != before {
			t.Fatal("supplied ownership reached the database")
		}
		ok := f.call(creator, "POST", "/items", itemBody("task", "crt15-ok", spaceA, ""))
		f.want(ok, http.StatusCreated, "server-derived")
		if got := f.ownershipOf(ok.body["id"].(string)); got != "client/"+client {
			t.Fatalf("ownership = %s, want the Space's client/%s", got, client)
		}
	})
}

// CRT-17..CRT-19: creation and Customer archive are serialized through the Customer row. Lock waits are observed in the
// database, not timed.
func TestCreateAndCustomerArchiveAreSerialized(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	creator := f.person("regular")
	client := f.createClient(admin, "CRT-E")["id"].(string)
	f.addMember(admin, client, creator)
	space := f.scopedSpace("Client E", "client", client)
	f.share(space, "user", creator.id, "full")
	archive := func() error {
		_, err := clients.New(f.pool).Archive(f.ctx, admin.id, client, func(context.Context, pgx.Tx, clients.Event) error { return nil })
		return err
	}

	t.Run("CRT-17 a create that starts while an archive waits for it succeeds, and a later create fails", func(t *testing.T) {
		f := f.on(t)
		// An open create holds the Customer row FOR SHARE. The archive must wait for it.
		tx, err := f.pool.BeginTx(f.ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx) // no-op after Commit; releases the Customer lock on a failed path
		actor := policy.Actor{ID: creator.id, Type: policy.Regular}
		fields := map[string]json.RawMessage{"type": json.RawMessage(`"task"`), "title": json.RawMessage(`"crt17-open"`),
			"locations": json.RawMessage(fmt.Sprintf(`[{"space_id":"%s"}]`, space))}
		item, err := items.CreateTx(f.ctx, tx, actor, fields, &notifications.Batch{})
		if err != nil {
			t.Fatalf("create inside open transaction: %v", err)
		}
		done := make(chan error, 1)
		go func() { done <- archive() }()
		waitForLockWait(t, f)
		// The create still holds the lock: a no-wait lock on the same Customer row must fail.
		if _, err := f.pool.Exec(f.ctx, "SELECT 1 FROM clients WHERE id=$1::uuid FOR UPDATE NOWAIT", client); err == nil {
			t.Fatal("the open create did not hold the Customer row")
		}
		if err := tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatalf("archive after the create: %v", err)
		}
		var created struct{ ID string }
		if err := json.Unmarshal(item.Body, &created); err != nil || created.ID == "" {
			t.Fatalf("created item id missing (err=%v)", err)
		}
		if got := f.ownershipOf(created.ID); got != "client/"+client {
			t.Fatalf("the create committed before the archive with ownership %s", got)
		}
		f.want(f.call(creator, "POST", "/items", itemBody("task", "crt17-after", space, "")), http.StatusConflict, "create after archive")
	})

	t.Run("CRT-18 concurrent children under one parent keep distinct ranks and locations", func(t *testing.T) {
		f := f.on(t)
		// Its own Customer: CRT-17 archives the one above, and an archived Customer receives no new work.
		clientF := f.createClient(admin, "CRT-E18")["id"].(string)
		f.addMember(admin, clientF, creator)
		space := f.scopedSpace("Client E18", "client", clientF)
		f.share(space, "user", creator.id, "full")
		client := clientF
		parent := f.call(creator, "POST", "/items", itemBody("project", "crt18-parent", space, ""))
		f.want(parent, http.StatusCreated, "parent")
		pid := parent.body["id"].(string)
		const n = 6
		var wg sync.WaitGroup
		statuses := make([]int, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				statuses[i] = f.call(creator, "POST", "/items", itemBody("task", fmt.Sprintf("crt18-child-%d", i), space, pid)).status
			}(i)
		}
		wg.Wait()
		for i, s := range statuses {
			if s != http.StatusCreated {
				t.Fatalf("concurrent child %d status %d, want 201", i, s)
			}
		}
		if got := f.count("SELECT count(DISTINCT rank) FROM item_locations WHERE parent_item_id=$1::uuid AND space_id=$2::uuid", pid, space); got != n {
			t.Fatalf("distinct ranks = %d, want %d", got, n)
		}
		if got := f.count("SELECT count(*) FROM items i JOIN item_locations l ON l.item_id=i.id WHERE l.parent_item_id=$1::uuid AND i.scope_state='client' AND i.client_id=$2::uuid", pid, client); got != n {
			t.Fatalf("children with inherited ownership = %d, want %d", got, n)
		}
	})

	t.Run("CRT-19 creation holds the Customer row for its whole transaction", func(t *testing.T) {
		f := f.on(t)
		client := f.createClient(admin, "CRT-E19")["id"].(string)
		f.addMember(admin, client, creator)
		space := f.scopedSpace("Client E19", "client", client)
		f.share(space, "user", creator.id, "full")
		tx, err := f.pool.BeginTx(f.ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx)
		actor := policy.Actor{ID: creator.id, Type: policy.Regular}
		fields := map[string]json.RawMessage{"type": json.RawMessage(`"task"`), "title": json.RawMessage(`"crt19"`),
			"locations": json.RawMessage(fmt.Sprintf(`[{"space_id":"%s"}]`, space))}
		if _, err := items.CreateTx(f.ctx, tx, actor, fields, &notifications.Batch{}); err != nil {
			t.Fatal(err)
		}
		// FOR NO KEY UPDATE conflicts with an explicit FOR SHARE but not with the foreign-key lock (FOR KEY SHARE) that any
		// item insert takes anyway. So this probe sees only the lock the create takes on purpose when it reads ownership.
		if _, err := f.pool.Exec(f.ctx, "SELECT 1 FROM clients WHERE id=$1::uuid FOR NO KEY UPDATE NOWAIT", client); err == nil || !strings.Contains(err.Error(), "55P03") {
			t.Fatalf("Customer row FOR SHARE not held by the open create (err=%v)", err)
		}
		if _, err := f.pool.Exec(f.ctx, "SELECT 1 FROM spaces WHERE id=$1::uuid FOR NO KEY UPDATE NOWAIT", space); err == nil || !strings.Contains(err.Error(), "55P03") {
			t.Fatalf("Space row FOR SHARE not held by the open create (err=%v)", err)
		}
	})
}

// Permissions are unchanged by ownership: a viewer cannot create, and an administrator without a grant gains nothing.
func TestOwnershipDoesNotChangeCreatePermissions(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	viewer := f.person("regular")
	client := f.createClient(admin, "CRT-F")["id"].(string)
	f.addMember(admin, client, viewer)
	space := f.scopedSpace("Client F", "client", client)
	f.share(space, "user", viewer.id, "read")

	f.want(f.call(viewer, "POST", "/items", itemBody("task", "crt-view", space, "")), http.StatusForbidden, "viewer create")
	f.want(f.call(admin, "POST", "/items", itemBody("task", "crt-admin", space, "")), http.StatusNotFound, "admin without grant")
	if f.itemCount("crt-view") != 0 || f.itemCount("crt-admin") != 0 {
		t.Fatal("a denied create wrote an item")
	}
}

// internalSpace is a shared Space with explicit internal ownership and the same automatic admin shares as f.space. Tests
// whose subject is not ownership use it to create items. Ownership is explicit here, never inferred. f.space still creates
// unresolved Spaces, so an item created there is refused (B3-03).
func (f *fixture) internalSpace(name string) string {
	f.t.Helper()
	id := f.scopedSpace(name, "internal", "")
	for _, admin := range f.admins {
		f.share(id, "user", admin, "full")
	}
	return id
}
