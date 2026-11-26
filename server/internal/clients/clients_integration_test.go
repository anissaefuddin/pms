package clients_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/clients"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Customer and membership domain tests (B2-03). Every database is a fresh migrated database from the test harness.

type env struct {
	t      *testing.T
	pool   *pgxpool.Pool
	svc    *clients.Service
	admin  string // active admin: a valid actor
	admin2 string
	mu     sync.Mutex
	events []clients.Event
	rec    clients.Recorder // records events; safe for concurrent use
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.NewMigrated(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	e := &env{t: t, pool: pool, svc: clients.New(pool)}
	e.rec = func(_ context.Context, _ pgx.Tx, ev clients.Event) error {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.events = append(e.events, ev)
		return nil
	}
	e.admin = e.user("admin-a@example.com", "admin", "active")
	e.admin2 = e.user("admin-b@example.com", "admin", "active")
	return e
}

func (e *env) user(email, userType, status string) string {
	e.t.Helper()
	var id string
	if err := e.pool.QueryRow(context.Background(),
		`INSERT INTO users(email,first_name,last_name,password_hash,user_type,status) VALUES ($1,'Test','User',CASE WHEN $3='invited' THEN NULL ELSE 'x' END,$2::user_type,$3::user_status) RETURNING id::text`,
		email, userType, status).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func (e *env) customer(code string) clients.Customer {
	e.t.Helper()
	c, err := e.svc.Create(context.Background(), e.admin, clients.CreateInput{Code: code, Name: "Customer " + code}, e.rec)
	if err != nil {
		e.t.Fatalf("create %s: %v", code, err)
	}
	return c
}

func isInvalid(err error, field string) bool {
	var inv clients.InvalidError
	return errors.As(err, &inv) && inv.Field == field
}

func strp(s string) *string { return &s }

// CL-01, CL-02, CL-03, CL-04, CL-05: create, normalization, duplicates, names, and lists by lifecycle state.
func TestCustomerLifecycle(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	t.Run("CL-01 create and canonical identity", func(t *testing.T) {
		c, err := e.svc.Create(ctx, e.admin, clients.CreateInput{Code: "  acme-holding ", Name: " ACME Holding ", ShortName: strp("  ACME  ")}, e.rec)
		if err != nil {
			t.Fatal(err)
		}
		if c.Code != "ACME-HOLDING" || c.Name != "ACME Holding" || c.Status != "active" || c.CreatedBy != e.admin || c.ID == "" {
			t.Fatalf("canonical identity = %+v", c)
		}
		if c.ShortName == nil || *c.ShortName != "ACME" {
			t.Fatalf("short name = %v, want trimmed ACME", c.ShortName)
		}
		got, err := e.svc.Get(ctx, c.ID)
		if err != nil || got.Code != "ACME-HOLDING" {
			t.Fatalf("get by UUID = %+v, %v", got, err)
		}
	})

	t.Run("CL-02 code and name whitespace validation", func(t *testing.T) {
		for _, code := range []string{"", "   ", "\t", "\n \t"} {
			if _, err := e.svc.Create(ctx, e.admin, clients.CreateInput{Code: code, Name: "X"}, e.rec); !isInvalid(err, "code") {
				t.Fatalf("code %q: err = %v, want invalid code", code, err)
			}
		}
	})

	t.Run("CL-03 duplicate normalized code is rejected by the database", func(t *testing.T) {
		e.customer("DUP-CODE")
		if _, err := e.svc.Create(ctx, e.admin, clients.CreateInput{Code: " dup-code ", Name: "Other"}, e.rec); !errors.Is(err, clients.ErrDuplicateCode) {
			t.Fatalf("err = %v, want ErrDuplicateCode", err)
		}
	})

	t.Run("CL-04 blank name rejected", func(t *testing.T) {
		for _, name := range []string{"", "  ", "\t"} {
			if _, err := e.svc.Create(ctx, e.admin, clients.CreateInput{Code: "NAME-" + name, Name: name}, e.rec); !isInvalid(err, "name") {
				t.Fatalf("name %q: err = %v, want invalid name", name, err)
			}
		}
	})

	t.Run("CL-05 list by state, and Get distinguishes archived and missing", func(t *testing.T) {
		live := e.customer("LIST-LIVE")
		old := e.customer("LIST-OLD")
		if _, err := e.svc.Archive(ctx, e.admin, old.ID, e.rec); err != nil {
			t.Fatal(err)
		}
		activeOnly, err := e.svc.List(ctx, clients.Active, 0)
		if err != nil {
			t.Fatal(err)
		}
		archivedOnly, err := e.svc.List(ctx, clients.Archived, 0)
		if err != nil {
			t.Fatal(err)
		}
		all, err := e.svc.List(ctx, clients.All, 0)
		if err != nil {
			t.Fatal(err)
		}
		has := func(list []clients.Customer, id string) bool {
			for _, c := range list {
				if c.ID == id {
					return true
				}
			}
			return false
		}
		if !has(activeOnly, live.ID) || has(activeOnly, old.ID) {
			t.Fatal("active filter must show the live customer and hide the archived one")
		}
		if !has(archivedOnly, old.ID) || has(archivedOnly, live.ID) {
			t.Fatal("archived filter must show only the archived customer")
		}
		if !has(all, live.ID) || !has(all, old.ID) {
			t.Fatal("all must show both")
		}
		for i := 1; i < len(all); i++ {
			if all[i-1].Code > all[i].Code {
				t.Fatal("list is not ordered by code")
			}
		}
		if got, err := e.svc.Get(ctx, old.ID); err != nil || got.Status != "archived" {
			t.Fatalf("archived customer must still be readable: %+v, %v", got, err)
		}
		if _, err := e.svc.Get(ctx, "00000000-0000-4000-8000-000000000404"); !errors.Is(err, clients.ErrNotFound) {
			t.Fatalf("missing: err = %v, want ErrNotFound", err)
		}
		if _, err := e.svc.Get(ctx, "not-a-uuid"); !errors.Is(err, clients.ErrNotFound) {
			t.Fatalf("malformed id: err = %v, want ErrNotFound", err)
		}
		if _, err := e.svc.List(ctx, clients.All, 201); !isInvalid(err, "limit") {
			t.Fatalf("limit above cap: err = %v", err)
		}
	})
}

// CL-06, CL-07, CL-08, CL-09, CL-10.
func TestCustomerUpdateAndArchive(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	t.Run("CL-06 update changes only name and short name", func(t *testing.T) {
		c := e.customer("UPD-ONE")
		got, err := e.svc.Update(ctx, e.admin, c.ID, clients.UpdateInput{Name: strp("Renamed"), ShortName: strp("  R  ")}, e.rec)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Renamed" || got.ShortName == nil || *got.ShortName != "R" || got.Code != c.Code || got.ID != c.ID || got.CreatedBy != c.CreatedBy {
			t.Fatalf("after update = %+v", got)
		}
		cleared, err := e.svc.Update(ctx, e.admin, c.ID, clients.UpdateInput{ShortName: strp("   ")}, e.rec)
		if err != nil || cleared.ShortName != nil || cleared.Name != "Renamed" {
			t.Fatalf("blank short name must clear it and keep the name: %+v, %v", cleared, err)
		}
		if _, err := e.svc.Update(ctx, e.admin, c.ID, clients.UpdateInput{Name: strp("  ")}, e.rec); !isInvalid(err, "name") {
			t.Fatalf("blank name: err = %v", err)
		}
	})

	t.Run("CL-07 code and UUID cannot change through the contract or the database", func(t *testing.T) {
		typ := reflect.TypeOf(clients.UpdateInput{})
		for i := 0; i < typ.NumField(); i++ {
			if n := typ.Field(i).Name; n == "Code" || n == "ID" {
				t.Fatalf("UpdateInput must not carry %s", n)
			}
		}
		c := e.customer("UPD-CODE")
		_, err := e.pool.Exec(ctx, `UPDATE clients SET code='CHANGED' WHERE id=$1::uuid`, c.ID)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "23514" {
			t.Fatalf("direct code change: err = %v, want the database to refuse it", err)
		}
	})

	t.Run("CL-08 archive keeps identity and membership history", func(t *testing.T) {
		c := e.customer("ARCH-KEEP")
		m, err := e.svc.Grant(ctx, e.admin, c.ID, e.user("member-arch@example.com", "regular", "active"), e.rec)
		if err != nil {
			t.Fatal(err)
		}
		archived, err := e.svc.Archive(ctx, e.admin, c.ID, e.rec)
		if err != nil {
			t.Fatal(err)
		}
		if archived.Status != "archived" || archived.ID != c.ID || archived.Code != c.Code || archived.ArchivedAt == nil || archived.ArchivedBy == nil || *archived.ArchivedBy != e.admin {
			t.Fatalf("archived = %+v", archived)
		}
		rows, err := e.svc.ListMembers(ctx, c.ID, true)
		if err != nil || len(rows) != 1 || rows[0].ID != m.ID || rows[0].Status != "active" {
			t.Fatalf("membership history changed by archive: %+v, %v", rows, err)
		}
	})

	t.Run("CL-09 repeated archive keeps the first archive metadata", func(t *testing.T) {
		c := e.customer("ARCH-TWICE")
		first, err := e.svc.Archive(ctx, e.admin, c.ID, e.rec)
		if err != nil {
			t.Fatal(err)
		}
		second, err := e.svc.Archive(ctx, e.admin2, c.ID, e.rec)
		if err != nil {
			t.Fatal(err)
		}
		if !second.ArchivedAt.Equal(*first.ArchivedAt) || *second.ArchivedBy != e.admin || second.Status != "archived" {
			t.Fatalf("second archive overwrote metadata: first %+v second %+v", first, second)
		}
	})

	t.Run("CL-10 archived customer rejects metadata change and new grants", func(t *testing.T) {
		c := e.customer("ARCH-RO")
		if _, err := e.svc.Archive(ctx, e.admin, c.ID, e.rec); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.Update(ctx, e.admin, c.ID, clients.UpdateInput{Name: strp("Nope")}, e.rec); !errors.Is(err, clients.ErrArchived) {
			t.Fatalf("update archived: err = %v, want ErrArchived", err)
		}
		user := e.user("member-ro@example.com", "regular", "active")
		if _, err := e.svc.Grant(ctx, e.admin, c.ID, user, e.rec); !errors.Is(err, clients.ErrArchived) {
			t.Fatalf("grant to archived: err = %v, want ErrArchived", err)
		}
		if n := e.count(`SELECT count(*)::int FROM client_memberships WHERE client_id=$1::uuid`, c.ID); n != 0 {
			t.Fatalf("a rejected grant left %d rows", n)
		}
	})
}

// MEM-01..MEM-09 and MEM-12.
func TestMembership(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	c := e.customer("MEM-CLIENT")
	member := e.user("member-main@example.com", "regular", "active")

	var first clients.Membership
	t.Run("MEM-01 grant creates one active membership with actor metadata", func(t *testing.T) {
		m, err := e.svc.Grant(ctx, e.admin, c.ID, member, e.rec)
		if err != nil {
			t.Fatal(err)
		}
		if m.Status != "active" || m.GrantedBy != e.admin || m.GrantedAt.IsZero() || m.RevokedAt != nil || m.ClientID != c.ID || m.UserID != member {
			t.Fatalf("membership = %+v", m)
		}
		first = m
	})

	t.Run("MEM-02 repeated grant is idempotent and keeps the original actor and time", func(t *testing.T) {
		again, err := e.svc.Grant(ctx, e.admin2, c.ID, member, e.rec)
		if err != nil {
			t.Fatal(err)
		}
		if again.ID != first.ID || again.GrantedBy != e.admin || !again.GrantedAt.Equal(first.GrantedAt) {
			t.Fatalf("repeated grant changed the row: first %+v again %+v", first, again)
		}
		if n := e.count(`SELECT count(*)::int FROM client_memberships WHERE client_id=$1::uuid AND user_id=$2::uuid`, c.ID, member); n != 1 {
			t.Fatalf("rows = %d, want 1", n)
		}
	})

	t.Run("MEM-03 invalid customer or user rejected", func(t *testing.T) {
		if _, err := e.svc.Grant(ctx, e.admin, "00000000-0000-4000-8000-000000000404", member, e.rec); !errors.Is(err, clients.ErrNotFound) {
			t.Fatalf("missing customer: %v", err)
		}
		if _, err := e.svc.Grant(ctx, e.admin, c.ID, "00000000-0000-4000-8000-000000000404", e.rec); !errors.Is(err, clients.ErrUserNotFound) {
			t.Fatalf("missing user: %v", err)
		}
		if _, err := e.svc.Grant(ctx, e.admin, c.ID, "not-a-uuid", e.rec); !errors.Is(err, clients.ErrUserNotFound) {
			t.Fatalf("malformed user: %v", err)
		}
	})

	t.Run("MEM-04 ineligible users rejected, eligible staff accepted", func(t *testing.T) {
		for _, tc := range []struct {
			name, userType, status string
			ok                     bool
		}{
			{"external", "external", "active", false},
			{"collaborator", "collaborator", "active", false},
			{"viewer", "viewer", "active", false},
			{"invited", "regular", "invited", false},
			{"deactivated", "regular", "deactivated", false},
			{"regular", "regular", "active", true},
			{"admin", "admin", "active", true},
		} {
			id := e.user(tc.name+"@example.com", tc.userType, tc.status)
			_, err := e.svc.Grant(ctx, e.admin, c.ID, id, e.rec)
			if tc.ok && err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if !tc.ok && !errors.Is(err, clients.ErrIneligibleUser) {
				t.Fatalf("%s: err = %v, want ErrIneligibleUser", tc.name, err)
			}
		}
	})

	t.Run("MEM-05 revoke keeps the grant and records revocation", func(t *testing.T) {
		m, err := e.svc.Revoke(ctx, e.admin2, c.ID, member, e.rec)
		if err != nil {
			t.Fatal(err)
		}
		if m.ID != first.ID || m.Status != "revoked" || m.RevokedAt == nil || m.RevokedBy == nil || *m.RevokedBy != e.admin2 || m.GrantedBy != e.admin || !m.GrantedAt.Equal(first.GrantedAt) {
			t.Fatalf("revoked = %+v", m)
		}
	})

	t.Run("MEM-06 repeated revoke does not rewrite history", func(t *testing.T) {
		before, err := e.svc.Revoke(ctx, e.admin2, c.ID, member, e.rec)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
		after, err := e.svc.Revoke(ctx, e.admin, c.ID, member, e.rec)
		if err != nil {
			t.Fatal(err)
		}
		if !after.RevokedAt.Equal(*before.RevokedAt) || *after.RevokedBy != e.admin2 {
			t.Fatalf("second revoke rewrote history: before %+v after %+v", before, after)
		}
	})

	t.Run("MEM-07 regrant creates a new row and keeps the old one", func(t *testing.T) {
		regrant, err := e.svc.Grant(ctx, e.admin, c.ID, member, e.rec)
		if err != nil {
			t.Fatal(err)
		}
		if regrant.ID == first.ID || regrant.Status != "active" {
			t.Fatalf("regrant = %+v, want a new active row", regrant)
		}
		if n := e.count(`SELECT count(*)::int FROM client_memberships WHERE client_id=$1::uuid AND user_id=$2::uuid`, c.ID, member); n != 2 {
			t.Fatalf("rows = %d, want 2", n)
		}
		if n := e.count(`SELECT count(*)::int FROM client_memberships WHERE client_id=$1::uuid AND user_id=$2::uuid AND status='active'`, c.ID, member); n != 1 {
			t.Fatalf("active rows = %d, want 1", n)
		}
	})

	t.Run("MEM-08 active and history lists are distinct", func(t *testing.T) {
		other := e.user("member-list@example.com", "regular", "active")
		if _, err := e.svc.Grant(ctx, e.admin, c.ID, other, e.rec); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.Revoke(ctx, e.admin, c.ID, other, e.rec); err != nil {
			t.Fatal(err)
		}
		active, err := e.svc.ListMembers(ctx, c.ID, false)
		if err != nil {
			t.Fatal(err)
		}
		history, err := e.svc.ListMembers(ctx, c.ID, true)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range active {
			if m.Status != "active" || m.UserID == other {
				t.Fatalf("active list shows an ineligible row: %+v", m)
			}
		}
		if len(history) <= len(active) {
			t.Fatalf("history (%d) must include revoked rows beyond active (%d)", len(history), len(active))
		}
		fields := reflect.TypeOf(clients.Membership{})
		for i := 0; i < fields.NumField(); i++ {
			switch fields.Field(i).Name {
			case "ID", "ClientID", "UserID", "UserName", "Status", "GrantedAt", "GrantedBy", "RevokedAt", "RevokedBy":
			default:
				t.Fatalf("Membership exposes unexpected field %s", fields.Field(i).Name)
			}
		}
	})

	t.Run("MEM-09 membership creates no shares, admins or resource roles", func(t *testing.T) {
		before := e.count(`SELECT (SELECT count(*) FROM shares)+(SELECT count(*) FROM space_admins)+(SELECT count(*) FROM custom_roles)`)
		other := e.user("member-noshare@example.com", "regular", "active")
		if _, err := e.svc.Grant(ctx, e.admin, c.ID, other, e.rec); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.Revoke(ctx, e.admin, c.ID, other, e.rec); err != nil {
			t.Fatal(err)
		}
		if after := e.count(`SELECT (SELECT count(*) FROM shares)+(SELECT count(*) FROM space_admins)+(SELECT count(*) FROM custom_roles)`); after != before {
			t.Fatalf("grant and revoke changed resource rows: %d -> %d", before, after)
		}
	})

	t.Run("MEM-12 revoke works for an archived customer and for a deactivated user", func(t *testing.T) {
		arch := e.customer("MEM-ARCH")
		u := e.user("member-deact@example.com", "regular", "active")
		if _, err := e.svc.Grant(ctx, e.admin, arch.ID, u, e.rec); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.Archive(ctx, e.admin, arch.ID, e.rec); err != nil {
			t.Fatal(err)
		}
		if _, err := e.pool.Exec(ctx, `UPDATE users SET status='deactivated' WHERE id=$1::uuid`, u); err != nil {
			t.Fatal(err)
		}
		if m, err := e.svc.Revoke(ctx, e.admin, arch.ID, u, e.rec); err != nil || m.Status != "revoked" {
			t.Fatalf("revoke archived customer / deactivated user: %+v, %v", m, err)
		}
		if _, err := e.svc.Revoke(ctx, e.admin, arch.ID, "00000000-0000-4000-8000-000000000ab1", e.rec); !errors.Is(err, clients.ErrNotMember) {
			t.Fatalf("never a member: err = %v, want ErrNotMember", err)
		}
	})
}

// CL-03 under concurrency: several creates of one code produce exactly one customer.
func TestConcurrentCreateKeepsOneCode(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, dup := 0, 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.svc.Create(ctx, e.admin, clients.CreateInput{Code: "RACE-CODE", Name: "Race"}, e.rec)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, clients.ErrDuplicateCode):
				dup++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 1 || dup != 7 {
		t.Fatalf("creates: ok=%d duplicate=%d, want 1 and 7", ok, dup)
	}
}

// MEM-10: concurrent grants of one pair leave exactly one active row, and every caller sees that same row.
func TestConcurrentGrantsLeaveOneActiveMembership(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	c := e.customer("RACE-GRANT")
	member := e.user("member-race@example.com", "regular", "active")
	var wg sync.WaitGroup
	ids := make([]string, 10)
	errs := make([]error, 10)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m, err := e.svc.Grant(ctx, e.admin, c.ID, member, e.rec)
			ids[i], errs[i] = m.ID, err
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("grant %d: %v", i, err)
		}
		if ids[i] != ids[0] {
			t.Fatalf("grant %d returned row %s, want the single active row %s", i, ids[i], ids[0])
		}
	}
	if n := e.count(`SELECT count(*)::int FROM client_memberships WHERE client_id=$1::uuid AND user_id=$2::uuid AND status='active'`, c.ID, member); n != 1 {
		t.Fatalf("active rows = %d, want 1", n)
	}
}

// MEM-11: a grant waits for an archive in progress and is then refused. The archive lock is held explicitly, so the
// ordering does not depend on timing: the test waits until the grant is blocked on the lock before committing the archive.
func TestGrantWaitsForArchiveAndIsRefused(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	c := e.customer("ARCH-RACE")
	member := e.user("member-archrace@example.com", "regular", "active")

	archiveTx, err := e.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer archiveTx.Rollback(ctx)
	if _, err := archiveTx.Exec(ctx, `SELECT 1 FROM clients WHERE id=$1::uuid FOR UPDATE`, c.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := e.svc.Grant(ctx, e.admin, c.ID, member, e.rec)
		done <- err
	}()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if e.count(`SELECT count(*)::int FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%FROM clients WHERE id%FOR SHARE%'`) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the grant never waited on the client lock")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := archiveTx.Exec(ctx, `UPDATE clients SET status='archived', archived_at=now(), archived_by=$2::uuid, updated_at=now() WHERE id=$1::uuid`, c.ID, e.admin); err != nil {
		t.Fatal(err)
	}
	if err := archiveTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, clients.ErrArchived) {
		t.Fatalf("grant after archive: err = %v, want ErrArchived", err)
	}
	if n := e.count(`SELECT count(*)::int FROM client_memberships WHERE client_id=$1::uuid`, c.ID); n != 0 {
		t.Fatalf("a grant created %d rows after the archive committed", n)
	}
}
