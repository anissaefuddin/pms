package database_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/anissaefuddin/pms/server/migrations"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

// B2-01/B2-02 (migration 00363_clients.sql). Each rejection runs in its own transaction and is checked by SQLSTATE, so a
// test cannot pass because an earlier failed statement aborted the transaction.

const clientsVersion = 363

func clientSchemaPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// expectSQLState runs one statement in its own transaction and requires exactly the given SQLSTATE.
func expectSQLState(t *testing.T, pool *pgxpool.Pool, code, what, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, sql, args...)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != code {
		t.Fatalf("%s: got %v, want SQLSTATE %s", what, err, code)
	}
}

func execOK(t *testing.T, pool *pgxpool.Pool, what, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

func scalar(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) string {
	t.Helper()
	var out string
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&out); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return out
}

func addUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	return scalar(t, pool, "INSERT INTO users(email,first_name,last_name,password_hash,user_type,status) VALUES ($1,'T','U','x','admin','active') RETURNING id::text", email)
}

// DB-01, DB-02, DB-15, DB-16: the up migration applies to a schema at 00362, adds the expected columns, keeps every existing
// row, and the down migration removes exactly the new objects and the up migration restores them.
func TestClientsMigrationUpDownFromThePreviousSchema(t *testing.T) {
	ctx := context.Background()
	url := testdb.New(t)
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS, goose.WithTableName("schema_migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, clientsVersion-1); err != nil {
		t.Fatalf("migrate to 00362: %v", err)
	}
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('clients') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("clients must not exist before 00363 (exists=%v err=%v)", exists, err)
	}
	// An existing instance with a user, a Space, and a project-level row, written before the migration.
	if _, err := db.Exec(`INSERT INTO users(email,first_name,last_name,password_hash,user_type,status) VALUES ('owner@example.com','O','W','x','admin','active')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO spaces(name,kind,owner_id) SELECT 'Personal','personal',id FROM users WHERE email='owner@example.com'`); err != nil {
		t.Fatal(err)
	}
	before := func() string {
		var s string
		if err := db.QueryRow(`SELECT (SELECT count(*) FROM users)||'/'||(SELECT count(*) FROM spaces)||'/'||(SELECT count(*) FROM items)||'/'||(SELECT email FROM users LIMIT 1)`).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	snapshot := before()

	if _, err := provider.Up(ctx); err != nil { // DB-01
		t.Fatalf("up migration from 00362: %v", err)
	}
	if got := before(); got != snapshot { // DB-15
		t.Fatalf("existing rows changed: before %q after %q", snapshot, got)
	}

	// DB-02: every required column exists.
	want := map[string][]string{
		"clients":            {"id", "code", "name", "short_name", "status", "created_at", "created_by", "updated_at", "archived_at", "archived_by"},
		"client_memberships": {"id", "client_id", "user_id", "status", "granted_at", "granted_by", "revoked_at", "revoked_by"},
	}
	for table, cols := range want {
		for _, c := range cols {
			var ok bool
			if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name=$1 AND column_name=$2)`, table, c).Scan(&ok); err != nil || !ok {
				t.Fatalf("%s.%s missing (err=%v)", table, c, err)
			}
		}
	}

	// DB-16: the down migration removes the new objects; the up migration brings them back.
	if _, err := provider.DownTo(ctx, clientsVersion-1); err != nil { // reverts 00364 and 00363
		t.Fatalf("down migration 00363: %v", err)
	}
	var gone bool
	if err := db.QueryRow(`SELECT to_regclass('clients') IS NULL AND to_regclass('client_memberships') IS NULL AND to_regprocedure('clients_code_immutable()') IS NULL`).Scan(&gone); err != nil || !gone {
		t.Fatalf("down migration left objects behind (gone=%v err=%v)", gone, err)
	}
	if got := before(); got != snapshot {
		t.Fatalf("down migration changed existing rows: %q", got)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("second up migration: %v", err)
	}
	if err := db.QueryRow(`SELECT to_regclass('client_memberships') IS NOT NULL`).Scan(&gone); err != nil || !gone {
		t.Fatalf("up after down did not recreate client_memberships (err=%v)", err)
	}
}

// DB-03..DB-08, DB-11 (references), DB-12 for clients.
func TestClientIdentityConstraints(t *testing.T) {
	pool := clientSchemaPool(t)
	admin := addUser(t, pool, "admin-b2@example.com")

	execOK(t, pool, "DB-03 valid client", `INSERT INTO clients(code,name,short_name,created_by) VALUES ('ACME-EXAMPLE','Example Client','Example',$1)`, admin) // DB-03

	expectSQLState(t, pool, "23505", "DB-04 duplicate code", `INSERT INTO clients(code,name,created_by) VALUES ('ACME-EXAMPLE','Other',$1)`, admin) // DB-04
	for _, code := range []string{"acme-example", " ACME-EXAMPLE", "ACME-EXAMPLE ", "ACME-EXAMPLE\t", "\tACME", "   ", "\t", "\n", ""} {
		expectSQLState(t, pool, "23514", "DB-05 noncanonical or blank code "+code, `INSERT INTO clients(code,name,created_by) VALUES ($1,'X',$2)`, code, admin) // DB-05
	}
	for _, name := range []string{"", "   ", "\t", "\n \t"} {
		expectSQLState(t, pool, "23514", "DB-06 blank name", `INSERT INTO clients(code,name,created_by) VALUES ('BLANK-NAME',$1,$2)`, name, admin) // DB-06
	}
	expectSQLState(t, pool, "23514", "short_name blank", `INSERT INTO clients(code,name,short_name,created_by) VALUES ('SHORT-BLANK','X',' ',$1)`, admin)
	expectSQLState(t, pool, "23514", "short_name tab only", `INSERT INTO clients(code,name,short_name,created_by) VALUES ('SHORT-TAB','X',$2,$1)`, admin, "\t")
	expectSQLState(t, pool, "23514", "DB-07 invalid status", `INSERT INTO clients(code,name,status,created_by) VALUES ('BAD-STATUS','X','inactive',$1)`, admin) // DB-07

	// DB-08: archived exactly when archived_at is set. Default status is active.
	expectSQLState(t, pool, "23514", "DB-08 archived without archived_at", `INSERT INTO clients(code,name,status,created_by) VALUES ('ARCH-NOTIME','X','archived',$1)`, admin)
	expectSQLState(t, pool, "23514", "DB-08 active with archived_at", `INSERT INTO clients(code,name,status,archived_at,created_by) VALUES ('ARCH-ACTIVE','X','active',now(),$1)`, admin)
	execOK(t, pool, "DB-08 archived with metadata", `INSERT INTO clients(code,name,status,archived_at,archived_by,created_by) VALUES ('ARCH-OK','X','archived',now(),$1,$1)`, admin)
	if got := scalar(t, pool, `SELECT status FROM clients WHERE code='ACME-EXAMPLE'`); got != "active" {
		t.Fatalf("default status = %q, want active", got)
	}

	// Updates that keep the code are allowed; changing the code is not.
	execOK(t, pool, "rename keeps the code", `UPDATE clients SET name='Renamed' WHERE code='ACME-EXAMPLE'`)
	expectSQLState(t, pool, "23514", "code is permanent", `UPDATE clients SET code='ACME-CHANGED' WHERE code='ACME-EXAMPLE'`)

	// DB-11 for clients: a missing creator is rejected.
	expectSQLState(t, pool, "23503", "DB-11 nonexistent created_by", `INSERT INTO clients(code,name,created_by) VALUES ('NO-USER','X','00000000-0000-4000-8000-00000000dead')`)
}

// DB-09..DB-14 for memberships: eligibility rows, one active per pair, history kept, references checked, restricted deletes.
func TestClientMembershipConstraints(t *testing.T) {
	pool := clientSchemaPool(t)
	admin := addUser(t, pool, "admin-b2m@example.com")
	member := addUser(t, pool, "member-b2m@example.com")
	clientID := scalar(t, pool, `INSERT INTO clients(code,name,created_by) VALUES ('MEMBER-CLIENT','Member Client',$1) RETURNING id::text`, admin)

	execOK(t, pool, "DB-09 valid membership", `INSERT INTO client_memberships(client_id,user_id,granted_by) VALUES ($1,$2,$3)`, clientID, member, admin) // DB-09
	if got := scalar(t, pool, `SELECT status FROM client_memberships WHERE client_id=$1 AND user_id=$2`, clientID, member); got != "active" {
		t.Fatalf("default membership status = %q, want active", got)
	}

	expectSQLState(t, pool, "23505", "DB-10 second active membership", `INSERT INTO client_memberships(client_id,user_id,granted_by) VALUES ($1,$2,$3)`, clientID, member, admin)                     // DB-10
	expectSQLState(t, pool, "23503", "DB-11 nonexistent client", `INSERT INTO client_memberships(client_id,user_id,granted_by) VALUES ('00000000-0000-4000-8000-00000000c1e0',$1,$2)`, member, admin) // DB-11
	expectSQLState(t, pool, "23503", "DB-11 nonexistent user", `INSERT INTO client_memberships(client_id,user_id,granted_by) VALUES ($1,'00000000-0000-4000-8000-000000000e4e',$2)`, clientID, admin) // DB-11
	expectSQLState(t, pool, "23514", "DB-12 invalid membership status", `INSERT INTO client_memberships(client_id,user_id,status,granted_by) VALUES ($1,$2,'pending',$3)`, clientID, admin, admin)    // DB-12
	expectSQLState(t, pool, "23514", "revoked without revoked_at", `UPDATE client_memberships SET status='revoked' WHERE client_id=$1 AND user_id=$2`, clientID, member)
}

// DB-13: revocation keeps the row, a later grant is a new row, and exactly one stays active.
func TestMembershipRevokeAndRegrantKeepHistory(t *testing.T) {
	pool := clientSchemaPool(t)
	admin := addUser(t, pool, "admin-b2r@example.com")
	member := addUser(t, pool, "member-b2r@example.com")
	clientID := scalar(t, pool, `INSERT INTO clients(code,name,created_by) VALUES ('REGRANT-CLIENT','Regrant Client',$1) RETURNING id::text`, admin)
	execOK(t, pool, "grant", `INSERT INTO client_memberships(client_id,user_id,granted_by) VALUES ($1,$2,$3)`, clientID, member, admin)

	expectSQLState(t, pool, "23514", "revoke before grant", `UPDATE client_memberships SET status='revoked', revoked_at=granted_at - interval '1 day', revoked_by=$3 WHERE client_id=$1 AND user_id=$2`, clientID, member, admin)
	execOK(t, pool, "revoke", `UPDATE client_memberships SET status='revoked', revoked_at=now(), revoked_by=$3 WHERE client_id=$1 AND user_id=$2 AND status='active'`, clientID, member, admin)
	execOK(t, pool, "regrant as a new row", `INSERT INTO client_memberships(client_id,user_id,granted_by) VALUES ($1,$2,$3)`, clientID, member, admin)

	if got := scalar(t, pool, `SELECT count(*)::text FROM client_memberships WHERE client_id=$1 AND user_id=$2`, clientID, member); got != "2" {
		t.Fatalf("rows for the pair = %s, want 2 (revoked history kept)", got)
	}
	if got := scalar(t, pool, `SELECT count(*)::text FROM client_memberships WHERE client_id=$1 AND user_id=$2 AND status='active'`, clientID, member); got != "1" {
		t.Fatalf("active rows = %s, want 1", got)
	}
}

// DB-14: a client with membership history cannot be deleted, and neither can a user who appears in it.
func TestDeletesAreRestrictedWhenMembershipHistoryExists(t *testing.T) {
	pool := clientSchemaPool(t)
	admin := addUser(t, pool, "admin-b2d@example.com")
	member := addUser(t, pool, "member-b2d@example.com")
	clientID := scalar(t, pool, `INSERT INTO clients(code,name,created_by) VALUES ('DELETE-CLIENT','Delete Client',$1) RETURNING id::text`, admin)
	execOK(t, pool, "membership", `INSERT INTO client_memberships(client_id,user_id,granted_by) VALUES ($1,$2,$3)`, clientID, member, admin)

	expectSQLState(t, pool, "23503", "DB-14 delete client with membership", `DELETE FROM clients WHERE id=$1`, clientID)
	expectSQLState(t, pool, "23503", "delete user with membership", `DELETE FROM users WHERE id=$1`, member)
	if got := scalar(t, pool, `SELECT count(*)::text FROM client_memberships WHERE client_id=$1`, clientID); got != "1" {
		t.Fatalf("membership survived restricted delete: %s rows", got)
	}
}
