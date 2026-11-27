package database_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/anissaefuddin/pms/server/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

// B3-01/B3-02 (migration 00364_client_scope.sql). Each rejection runs in its own transaction and is checked by SQLSTATE.
// 23514 = check_violation, 23503 = foreign_key_violation, 23502 = not_null_violation.

const scopeVersion = 364

func insertUser(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	return scalar(t, pool, `INSERT INTO users(email,first_name,last_name,password_hash,user_type,status)
		VALUES ('u-'||gen_random_uuid()||'@example.com','T','U','x','admin','active') RETURNING id::text`)
}

func insertClient(t *testing.T, pool *pgxpool.Pool, actor string) string {
	t.Helper()
	return scalar(t, pool, `INSERT INTO clients(code,name,created_by) VALUES ('C-'||upper(substr(gen_random_uuid()::text,1,8)),'Customer',$1) RETURNING id::text`, actor)
}

// OWN-DB-02..12 and OWN-DB-17 (constraints): applied to both spaces and items with the same rules.
func TestOwnershipRowConstraints(t *testing.T) {
	pool := clientSchemaPool(t)
	actor := insertUser(t, pool)
	client := insertClient(t, pool, actor)
	const spaceIns = "INSERT INTO spaces(name,kind,client_id,scope_state) VALUES ('S',$1::space_kind,$2::uuid,$3)"
	const itemIns = "INSERT INTO items(type,title,created_by,client_id,scope_state) VALUES ('task','T',$1::uuid,$2::uuid,$3)"

	t.Run("OWN-DB-02 both tables carry client_id and scope_state", func(t *testing.T) {
		for _, table := range []string{"spaces", "items"} {
			for _, col := range []string{"client_id", "scope_state"} {
				if n := scalar(t, pool, `SELECT count(*)::text FROM information_schema.columns WHERE table_name=$1 AND column_name=$2`, table, col); n != "1" {
					t.Fatalf("%s.%s missing", table, col)
				}
			}
		}
	})

	t.Run("OWN-DB-05 valid client ownership is accepted", func(t *testing.T) {
		execOK(t, pool, "client space", spaceIns, "shared", client, "client")
		execOK(t, pool, "client item", itemIns, actor, client, "client")
	})

	t.Run("OWN-DB-06 client ownership without client_id is rejected", func(t *testing.T) {
		expectSQLState(t, pool, "23514", "space", "INSERT INTO spaces(name,kind,scope_state) VALUES ('S','shared','client')")
		expectSQLState(t, pool, "23514", "item", "INSERT INTO items(type,title,created_by,scope_state) VALUES ('task','T',$1,'client')", actor)
	})

	t.Run("OWN-DB-07 internal ownership without client_id is accepted", func(t *testing.T) {
		execOK(t, pool, "internal space", "INSERT INTO spaces(name,kind,scope_state) VALUES ('I','shared','internal')")
		execOK(t, pool, "internal item", "INSERT INTO items(type,title,created_by,scope_state) VALUES ('task','I',$1,'internal')", actor)
	})

	t.Run("OWN-DB-08 internal ownership with client_id is rejected", func(t *testing.T) {
		expectSQLState(t, pool, "23514", "space", spaceIns, "shared", client, "internal")
		expectSQLState(t, pool, "23514", "item", itemIns, actor, client, "internal")
	})

	t.Run("OWN-DB-09 unresolved ownership without client_id is accepted", func(t *testing.T) {
		execOK(t, pool, "unresolved space", "INSERT INTO spaces(name,kind,scope_state) VALUES ('U','shared','unresolved')")
		execOK(t, pool, "unresolved item", "INSERT INTO items(type,title,created_by,scope_state) VALUES ('task','U',$1,'unresolved')", actor)
	})

	t.Run("OWN-DB-10 unresolved ownership with client_id is rejected", func(t *testing.T) {
		expectSQLState(t, pool, "23514", "space", spaceIns, "shared", client, "unresolved")
		expectSQLState(t, pool, "23514", "item", itemIns, actor, client, "unresolved")
	})

	t.Run("OWN-DB-11 unknown or null scope_state is rejected", func(t *testing.T) {
		expectSQLState(t, pool, "23514", "unknown space state", spaceIns, "shared", nil, "shared-by-default")
		expectSQLState(t, pool, "23514", "unknown item state", itemIns, actor, nil, "guessed")
		expectSQLState(t, pool, "23502", "null space state", "INSERT INTO spaces(name,kind,scope_state) VALUES ('N','shared',NULL)")
		expectSQLState(t, pool, "23502", "null item state", "INSERT INTO items(type,title,created_by,scope_state) VALUES ('task','N',$1,NULL)", actor)
	})

	t.Run("OWN-DB-12 nonexistent client_id is rejected", func(t *testing.T) {
		missing := "00000000-0000-4000-8000-00000000dead"
		expectSQLState(t, pool, "23503", "space", spaceIns, "shared", missing, "client")
		expectSQLState(t, pool, "23503", "item", itemIns, actor, missing, "client")
	})

	t.Run("OWN-DB-17 a new Personal Space is internal and cannot be client-owned", func(t *testing.T) {
		// Application code inserts Personal Spaces without ownership fields; the trigger classifies them.
		person := insertUser(t, pool)
		if got := scalar(t, pool, `INSERT INTO spaces(name,kind,owner_id) VALUES ('Personal','personal',$1) RETURNING scope_state`, person); got != "internal" {
			t.Fatalf("new personal space scope_state = %q, want internal", got)
		}
		expectSQLState(t, pool, "23514", "personal with client", "INSERT INTO spaces(name,kind,owner_id,client_id,scope_state) VALUES ('P2','personal',$1,$2,'client')", insertUser(t, pool), client)
		expectSQLState(t, pool, "23514", "personal set to client", "UPDATE spaces SET scope_state='client', client_id=$2 WHERE kind='personal' AND owner_id=$1", person, client)
	})
}

// OWN-DB-13, OWN-DB-14: a Customer cannot be deleted while ownership refers to it, and archiving keeps ownership intact.
func TestCustomerLifecycleDoesNotErasePlannedOwnership(t *testing.T) {
	pool := clientSchemaPool(t)
	actor := insertUser(t, pool)
	client := insertClient(t, pool, actor)
	execOK(t, pool, "owned space", "INSERT INTO spaces(name,kind,client_id,scope_state) VALUES ('Owned','shared',$1,'client')", client)

	t.Run("OWN-DB-13 deleting a referenced Customer is refused", func(t *testing.T) {
		expectSQLState(t, pool, "23503", "delete referenced client", "DELETE FROM clients WHERE id=$1", client)
	})

	t.Run("OWN-DB-14 archiving keeps the ownership reference", func(t *testing.T) {
		execOK(t, pool, "archive", "UPDATE clients SET status='archived', archived_at=now(), archived_by=$2 WHERE id=$1", client, actor)
		if got := scalar(t, pool, `SELECT count(*)::text FROM spaces WHERE client_id=$1 AND scope_state='client'`, client); got != "1" {
			t.Fatalf("archive changed ownership: %s owned spaces remain", got)
		}
	})
}

// OWN-DB-01, OWN-DB-03, OWN-DB-04, OWN-DB-15, OWN-DB-16, OWN-DB-17 (legacy), OWN-DB-18 (down and up).
// A 00363 database with ordinary legacy data is migrated; hierarchy, owners, locations and grants must not change.
func TestOwnershipMigrationFromTheClientSchema(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, clientsVersion); err != nil { // OWN-DB-01 starts from 00363
		t.Fatalf("migrate to 00363: %v", err)
	}
	var hasScope bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='spaces' AND column_name='scope_state')`).Scan(&hasScope); err != nil || hasScope {
		t.Fatalf("scope_state must not exist before 00364 (exists=%v err=%v)", hasScope, err)
	}

	// Legacy data written before 00364: a user, a Personal Space, a shared Space, a parent item with a child, a
	// space admin row, and a location for each item.
	for _, q := range []string{
		`INSERT INTO users(email,first_name,last_name,password_hash,user_type,status) VALUES ('legacy@example.com','L','G','x','admin','active')`,
		`INSERT INTO spaces(name,kind,owner_id) SELECT 'Personal','personal',id FROM users WHERE email='legacy@example.com'`,
		`INSERT INTO spaces(name,kind) VALUES ('Legacy shared','shared')`,
		`INSERT INTO items(type,title,created_by) SELECT 'project','Parent',id FROM users WHERE email='legacy@example.com'`,
		`INSERT INTO items(type,title,created_by) SELECT 'task','Child',id FROM users WHERE email='legacy@example.com'`,
		`INSERT INTO item_locations(item_id,space_id,rank) SELECT i.id, s.id, 'a' FROM items i, spaces s WHERE i.title='Parent' AND s.name='Legacy shared'`,
		`INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) SELECT c.id, s.id, p.id, 'a' FROM items c, items p, spaces s WHERE c.title='Child' AND p.title='Parent' AND s.name='Legacy shared'`,
		`INSERT INTO space_admins(space_id,user_id,granted_by) SELECT s.id, u.id, u.id FROM spaces s, users u WHERE s.name='Legacy shared' AND u.email='legacy@example.com'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("legacy setup: %v", err)
		}
	}
	const fingerprint = `SELECT
		(SELECT string_agg(id::text||':'||COALESCE(owner_id::text,'-')||':'||title, ',' ORDER BY id) FROM items)||'|'||
		(SELECT string_agg(item_id::text||':'||space_id::text||':'||COALESCE(parent_item_id::text,'-')||':'||rank, ',' ORDER BY item_id, space_id) FROM item_locations)||'|'||
		(SELECT string_agg(space_id::text||':'||user_id::text, ',' ORDER BY space_id) FROM space_admins)||'|'||
		(SELECT string_agg(id::text||':'||name||':'||kind::text, ',' ORDER BY id) FROM spaces)`
	var before string
	if err := db.QueryRow(fingerprint).Scan(&before); err != nil {
		t.Fatal(err)
	}

	if _, err := provider.Up(ctx); err != nil { // OWN-DB-01: migrate 00364 over legacy data
		t.Fatalf("up 00364: %v", err)
	}

	t.Run("OWN-DB-03 and OWN-DB-04 legacy shared records stay unresolved", func(t *testing.T) {
		if got := sqlScalar(t, db, `SELECT scope_state||'/'||COALESCE(client_id::text,'null') FROM spaces WHERE name='Legacy shared'`); got != "unresolved/null" {
			t.Fatalf("legacy shared space = %q, want unresolved/null", got)
		}
		if got := sqlScalar(t, db, `SELECT count(*)::text FROM items WHERE scope_state='unresolved' AND client_id IS NULL`); got != "2" {
			t.Fatalf("legacy items unresolved = %s, want 2", got)
		}
	})

	t.Run("OWN-DB-17 legacy Personal Space is internal, not inferred", func(t *testing.T) {
		if got := sqlScalar(t, db, `SELECT scope_state FROM spaces WHERE kind='personal'`); got != "internal" {
			t.Fatalf("legacy personal space = %q, want internal", got)
		}
	})

	t.Run("OWN-DB-15 and OWN-DB-16 owners, hierarchy, locations and grants are unchanged", func(t *testing.T) {
		var after string
		if err := db.QueryRow(fingerprint).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if after != before {
			t.Fatalf("legacy records changed during 00364")
		}
	})

	t.Run("OWN-DB-18 down and up on a disposable database", func(t *testing.T) {
		if _, err := provider.DownTo(ctx, clientsVersion); err != nil {
			t.Fatalf("down to 00363: %v", err)
		}
		var gone bool
		if err := db.QueryRow(`SELECT NOT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name IN ('spaces','items') AND column_name IN ('client_id','scope_state'))`).Scan(&gone); err != nil || !gone {
			t.Fatalf("down left ownership columns behind (gone=%v err=%v)", gone, err)
		}
		if _, err := provider.Up(ctx); err != nil {
			t.Fatalf("second up: %v", err)
		}
		var after string
		if err := db.QueryRow(fingerprint).Scan(&after); err != nil || after != before {
			t.Fatalf("down and up changed legacy records (err=%v)", err)
		}
	})
}

// sqlScalar reads one text value from the plain database handle used by the migration test.
func sqlScalar(t *testing.T, db *sql.DB, q string) string {
	t.Helper()
	var out string
	if err := db.QueryRow(q).Scan(&out); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return out
}
