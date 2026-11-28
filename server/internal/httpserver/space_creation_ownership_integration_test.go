package httpserver_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/clients"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/spaces"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// B3-SC: ownership-aware shared Space creation (ADR §7m). The creator must state the ownership explicitly. A Customer
// Space needs the creator's active membership of that Customer. Real HTTP, real sessions, real database.

// SC-01..SC-15 share one fixture; mutating steps (archive, revoke) run last, in order.
func TestSpaceCreationOwnership(t *testing.T) {
	base := newFixture(t)
	f := base
	admin := f.person("admin")
	creator := f.person("regular")
	nonMember := f.person("regular")
	adminNoMember := f.person("admin")
	external := f.person("external")
	clientA := f.createClient(admin, "SC-A")["id"].(string)
	clientB := f.createClient(admin, "SC-B")["id"].(string)
	f.addMember(admin, clientA, creator)
	f.addMember(admin, clientB, creator)
	f.exec("INSERT INTO client_memberships(client_id,user_id,granted_by) VALUES ($1,$2,$3)", clientA, external.id, admin.id) // direct row: the API refuses externals (SC-10)
	missing := uuid.NewString()

	createBody := func(extra map[string]any) map[string]any {
		b := map[string]any{"name": "SC space"}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}
	state := func() string {
		return fmt.Sprintf("%d/%d/%d", f.count("SELECT count(*) FROM spaces"), f.count("SELECT count(*) FROM shares"), f.count("SELECT count(*) FROM space_admins"))
	}

	t.Run("SC-01 internal creation persists internal ownership", func(t *testing.T) {
		f := base.on(t)
		r := f.call(creator, "POST", "/spaces", createBody(map[string]any{"scope_state": "internal"}))
		f.want(r, http.StatusCreated, "internal create")
		id := r.body["id"].(string)
		if got := f.count("SELECT count(*) FROM spaces WHERE id=$1 AND scope_state='internal' AND client_id IS NULL", id); got != 1 {
			t.Fatalf("stored ownership wrong (%d rows)", got)
		}
	})

	t.Run("SC-02 and SC-03 an active member creates a Customer Space that is visible at once", func(t *testing.T) {
		f := base.on(t)
		r := f.call(creator, "POST", "/spaces", createBody(map[string]any{"scope_state": "client", "client_id": clientA}))
		f.want(r, http.StatusCreated, "customer create")
		id := r.body["id"].(string)
		if got := f.count("SELECT count(*) FROM spaces WHERE id=$1 AND scope_state='client' AND client_id=$2", id, clientA); got != 1 {
			t.Fatalf("stored ownership wrong (%d rows)", got)
		}
		f.want(f.call(creator, "GET", "/spaces/"+id, nil), http.StatusOK, "immediately visible to the creator")
		if !containsID(listedIDs(t, f.call(creator, "GET", "/spaces", nil)), id) {
			t.Fatal("the new Space is missing from the creator's list")
		}
	})

	t.Run("SC-04 a missing or empty ownership choice is rejected", func(t *testing.T) {
		f := base.on(t)
		before := state()
		f.want(f.call(creator, "POST", "/spaces", createBody(nil)), http.StatusUnprocessableEntity, "missing")
		f.want(f.call(creator, "POST", "/spaces", createBody(map[string]any{"scope_state": ""})), http.StatusUnprocessableEntity, "empty")
		if state() != before {
			t.Fatal("a refused create wrote rows")
		}
	})

	t.Run("SC-05 invalid combinations are rejected", func(t *testing.T) {
		f := base.on(t)
		before := state()
		f.want(f.call(creator, "POST", "/spaces", createBody(map[string]any{"scope_state": "internal", "client_id": clientA})), http.StatusUnprocessableEntity, "internal with client")
		f.want(f.call(creator, "POST", "/spaces", createBody(map[string]any{"scope_state": "client"})), http.StatusUnprocessableEntity, "client without client_id")
		f.want(f.call(creator, "POST", "/spaces", createBody(map[string]any{"scope_state": "client", "client_id": "not-a-uuid"})), http.StatusUnprocessableEntity, "malformed client_id")
		if state() != before {
			t.Fatal("an invalid combination wrote rows")
		}
	})

	t.Run("SC-06 unresolved and unknown states cannot be submitted", func(t *testing.T) {
		f := base.on(t)
		before := state()
		for _, scope := range []string{"unresolved", "shared", "CLIENT"} {
			f.want(f.call(creator, "POST", "/spaces", createBody(map[string]any{"scope_state": scope, "client_id": clientA})), http.StatusUnprocessableEntity, scope)
		}
		if state() != before {
			t.Fatal("a refused state wrote rows")
		}
	})

	t.Run("SC-07 a nonexistent or archived Customer is rejected", func(t *testing.T) {
		f := base.on(t)
		before := state()
		f.want(f.call(creator, "POST", "/spaces", createBody(map[string]any{"scope_state": "client", "client_id": missing})), http.StatusUnprocessableEntity, "nonexistent")
		f.want(f.call(admin, "POST", "/clients/"+clientB+"/archive", nil), http.StatusOK, "archive B")
		f.want(f.call(creator, "POST", "/spaces", createBody(map[string]any{"scope_state": "client", "client_id": clientB})), http.StatusUnprocessableEntity, "archived")
		if state() != before {
			t.Fatal("a refused Customer wrote rows")
		}
	})

	t.Run("SC-08 a non-member cannot create a Customer Space", func(t *testing.T) {
		f := base.on(t)
		before := state()
		f.want(f.call(nonMember, "POST", "/spaces", createBody(map[string]any{"scope_state": "client", "client_id": clientA})), http.StatusForbidden, "non-member")
		if state() != before {
			t.Fatal("a non-member create wrote rows")
		}
	})

	t.Run("SC-09 a Workspace Admin without membership cannot bypass eligibility", func(t *testing.T) {
		f := base.on(t)
		before := state()
		f.want(f.call(adminNoMember, "POST", "/spaces", createBody(map[string]any{"scope_state": "client", "client_id": clientA})), http.StatusForbidden, "admin without membership")
		if state() != before {
			t.Fatal("an admin create wrote rows")
		}
	})

	t.Run("SC-10 membership without the create capability is insufficient", func(t *testing.T) {
		f := base.on(t)
		before := state()
		f.want(f.call(external, "POST", "/spaces", createBody(map[string]any{"scope_state": "client", "client_id": clientA})), http.StatusForbidden, "external with membership")
		if state() != before {
			t.Fatal("an external create wrote rows")
		}
	})

	t.Run("SC-11 a failed creation leaves no Space, share, or Space Admin row", func(t *testing.T) {
		f := base.on(t)
		before := state()
		f.want(f.call(nonMember, "POST", "/spaces", createBody(map[string]any{"scope_state": "client", "client_id": clientA, "shares": []map[string]any{{"principal_type": "user", "principal_id": admin.id, "role": "full"}}})), http.StatusForbidden, "refused with shares")
		if state() != before {
			t.Fatalf("a refused create left rows: %s -> %s", before, state())
		}
	})

	t.Run("SC-12 a normal update cannot change ownership", func(t *testing.T) {
		f := base.on(t)
		id := f.call(creator, "POST", "/spaces", createBody(map[string]any{"scope_state": "internal"})).body["id"].(string)
		version := f.queryID("SELECT version::text FROM spaces WHERE id=$1", id)
		f.want(f.call(creator, "PATCH", "/spaces/"+id, map[string]any{"scope_state": "client", "client_id": clientA}, "If-Match", `"`+version+`"`), http.StatusUnprocessableEntity, "scope in PATCH")
		f.want(f.call(creator, "PATCH", "/spaces/"+id, map[string]any{"client_id": clientA}, "If-Match", `"`+version+`"`), http.StatusUnprocessableEntity, "client in PATCH")
		if got := f.count("SELECT count(*) FROM spaces WHERE id=$1 AND scope_state='internal' AND client_id IS NULL", id); got != 1 {
			t.Fatal("ownership changed through PATCH")
		}
	})

	t.Run("SC-15 Personal Space privacy is unchanged by the creation contract", func(t *testing.T) {
		f := base.on(t)
		owner := f.person("regular")
		personal := f.personalSpace(owner)
		f.want(f.call(admin, "GET", "/spaces/"+personal, nil), http.StatusNotFound, "admin on a personal Space")
		f.want(f.call(owner, "GET", "/spaces/"+personal, nil), http.StatusOK, "owner")
	})

	t.Run("SC-16 an archive that waits for an open creation sees the creation, and a later creation is refused", func(t *testing.T) {
		f := base.on(t)
		client := f.createClient(admin, "SC-RACE-ARCH")["id"].(string)
		f.addMember(admin, client, creator)
		tx, err := f.pool.BeginTx(f.ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx)
		actor := policy.Actor{ID: creator.id, Type: policy.Regular}
		created, err := spaces.CreateTx(f.ctx, tx, actor, spaces.CreateInput{Name: "race", ScopeState: "client", ClientID: client}, &notifications.Batch{})
		if err != nil {
			t.Fatalf("create inside open transaction: %v", err)
		}
		done := make(chan error, 1)
		go func() {
			_, err := clients.New(f.pool).Archive(f.ctx, admin.id, client, func(context.Context, pgx.Tx, clients.Event) error { return nil })
			done <- err
		}()
		waitForLockWait(t, f)
		if err := tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatalf("archive after the creation: %v", err)
		}
		if got := f.count("SELECT count(*) FROM spaces WHERE id=$1 AND scope_state='client' AND client_id=$2", created.ID, client); got != 1 {
			t.Fatal("the creation that committed first is missing")
		}
		f.want(f.call(creator, "POST", "/spaces", createBody(map[string]any{"scope_state": "client", "client_id": client})), http.StatusUnprocessableEntity, "create after archive")
	})

	t.Run("SC-17 a revoke that waits for an open creation sees the creation, and a later creation is refused", func(t *testing.T) {
		f := base.on(t)
		client := f.createClient(admin, "SC-RACE-REV")["id"].(string)
		f.addMember(admin, client, creator)
		tx, err := f.pool.BeginTx(f.ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx)
		actor := policy.Actor{ID: creator.id, Type: policy.Regular}
		created, err := spaces.CreateTx(f.ctx, tx, actor, spaces.CreateInput{Name: "race revoke", ScopeState: "client", ClientID: client}, &notifications.Batch{})
		if err != nil {
			t.Fatalf("create inside open transaction: %v", err)
		}
		done := make(chan error, 1)
		go func() {
			_, err := clients.New(f.pool).Revoke(f.ctx, admin.id, client, creator.id, func(context.Context, pgx.Tx, clients.Event) error { return nil })
			done <- err
		}()
		waitForLockWait(t, f)
		if err := tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatalf("revoke after the creation: %v", err)
		}
		if got := f.count("SELECT count(*) FROM spaces WHERE id=$1", created.ID); got != 1 {
			t.Fatal("the creation that committed first is missing")
		}
		f.want(f.call(creator, "POST", "/spaces", createBody(map[string]any{"scope_state": "client", "client_id": client})), http.StatusForbidden, "create after revoke")
	})
}
