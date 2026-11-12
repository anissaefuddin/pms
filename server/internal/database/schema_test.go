package database_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestSchemaRules checks the constraints DATABASE.md promises (decision D-13): single account, no account_id,
// case-insensitive email, enum-valued status, one default workflow, one Personal space per user.
func TestSchemaRules(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Ready(ctx, pool); err != nil {
		t.Fatalf("a freshly migrated database must be ready: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM schema_migrations WHERE version_id=1"); err != nil {
		t.Fatal(err)
	}
	if database.Ready(ctx, pool) == nil {
		t.Fatal("Ready must fail when a migration is missing from schema_migrations")
	}
	// a duplicate row for another version must not hide the missing one
	if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations(version_id,is_applied) VALUES (2,true)"); err != nil {
		t.Fatal(err)
	}
	if database.Ready(ctx, pool) == nil {
		t.Fatal("Ready counted a duplicate row for version 2 as if version 1 were applied")
	}
	if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations(version_id,is_applied) VALUES (1,true)"); err != nil {
		t.Fatal(err)
	}

	var accountIDColumns int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND column_name='account_id'").Scan(&accountIDColumns); err != nil || accountIDColumns != 0 {
		t.Fatalf("columns named account_id: %d (%v), single-tenant tables must not have any", accountIDColumns, err)
	}

	exec := func(sql string, args ...any) error { _, err := pool.Exec(ctx, sql, args...); return err }
	// mustFail expects the statement to be rejected; when constraint is not empty it must be that exact constraint.
	mustFail := func(name, constraint, sql string, args ...any) {
		t.Helper()
		err := exec(sql, args...)
		if err == nil {
			t.Fatalf("%s: statement succeeded but must be rejected", name)
		}
		var pgErr *pgconn.PgError
		if constraint != "" && (!errors.As(err, &pgErr) || pgErr.ConstraintName != constraint) {
			t.Fatalf("%s: rejected by the wrong rule, want constraint %q: %v", name, constraint, err)
		}
	}
	mustWork := func(name, sql string, args ...any) {
		t.Helper()
		if err := exec(sql, args...); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	mustWork("first account", "INSERT INTO account(id,name) VALUES (1,'Acme')")
	mustFail("second account row", "account_id_check", "INSERT INTO account(id,name) VALUES (2,'Other')")
	mustFail("duplicate account id", "account_pkey", "INSERT INTO account(id,name) VALUES (1,'Again')")

	mustWork("user", "INSERT INTO users(email,user_type,status) VALUES ('A@Example.com','admin','invited')")
	mustFail("email differing only by case", "users_email_key", "INSERT INTO users(email) VALUES ('a@example.com')")
	mustFail("unknown status", "", "INSERT INTO users(email,status) VALUES ('x@example.com','banned')")
	mustFail("unknown user type", "", "INSERT INTO users(email,user_type) VALUES ('y@example.com','owner')")
	mustFail("active user without a password", "users_active_has_password", "INSERT INTO users(email,status,first_name,last_name) VALUES ('z@example.com','active','Z','Z')")
	mustFail("active user without a name", "users_active_has_name", "INSERT INTO users(email,status,password_hash) VALUES ('z@example.com','active','h')")
	mustFail("invited user with a password", "users_invited_has_no_password", "INSERT INTO users(email,status,password_hash) VALUES ('z@example.com','invited','h')")
	mustWork("active user with a password and a name", "INSERT INTO users(email,status,password_hash,first_name,last_name) VALUES ('ok@example.com','active','h','Ok','User')")
	mustFail("bad date format", "users_date_format_check", "UPDATE users SET date_format='dd-mm-yy' WHERE email='ok@example.com'")

	var created, updated bool
	mustWork("touch user", "UPDATE users SET locale='id' WHERE email='ok@example.com'")
	if err := pool.QueryRow(ctx, "SELECT updated_at > created_at, true FROM users WHERE email='ok@example.com'").Scan(&updated, &created); err != nil || !updated {
		t.Fatalf("updated_at trigger did not move updated_at (%v)", err)
	}

	mustWork("default workflow", "INSERT INTO workflows(name,is_default) VALUES ('Default',true)")
	mustFail("second default workflow", "workflows_one_default", "INSERT INTO workflows(name,is_default) VALUES ('Other',true)")
	mustWork("non-default workflow", "INSERT INTO workflows(name,is_default) VALUES ('Other',false)")
	mustWork("status", "INSERT INTO statuses(workflow_id,name,grp,color,position) SELECT id,'New','active','slate',0 FROM workflows WHERE is_default")
	mustFail("duplicate status name", "statuses_workflow_id_name_key", "INSERT INTO statuses(workflow_id,name,grp,color,position) SELECT id,'New','active','blue',1 FROM workflows WHERE is_default")
	mustFail("duplicate status position", "statuses_workflow_id_position_key", "INSERT INTO statuses(workflow_id,name,grp,color,position) SELECT id,'Other','active','blue',0 FROM workflows WHERE is_default")
	mustFail("unknown status group", "", "INSERT INTO statuses(workflow_id,name,grp,color,position) SELECT id,'X','paused','blue',9 FROM workflows WHERE is_default")

	mustWork("shared space", "INSERT INTO spaces(name,kind) VALUES ('General','shared')")
	mustFail("personal space without an owner", "spaces_personal_has_owner", "INSERT INTO spaces(name,kind) VALUES ('P','personal')")
	mustWork("personal space", "INSERT INTO spaces(name,kind,owner_id) SELECT 'Personal','personal',id FROM users WHERE email='ok@example.com'")
	mustFail("second personal space for the same user", "spaces_one_personal_per_owner", "INSERT INTO spaces(name,kind,owner_id) SELECT 'Personal 2','personal',id FROM users WHERE email='ok@example.com'")

	mustWork("session", "INSERT INTO sessions(token_hash,user_id,expires_at,absolute_expires_at) SELECT '\\x01',id,now()+interval '1 day',now()+interval '2 days' FROM users WHERE email='ok@example.com'")
	mustFail("idle limit after the absolute limit", "sessions_idle_within_absolute", "INSERT INTO sessions(token_hash,user_id,expires_at,absolute_expires_at) SELECT '\\x09',id,now()+interval '3 days',now()+interval '2 days' FROM users WHERE email='ok@example.com'")
	mustWork("token", "INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) SELECT '\\x02',id,'reset',now()+interval '1 hour' FROM users WHERE email='ok@example.com'")
	mustFail("unknown token purpose", "", "INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) SELECT '\\x03',id,'login',now() FROM users WHERE email='ok@example.com'")
	mustWork("deleting a user cascades to sessions and tokens", "DELETE FROM spaces WHERE kind='personal'")
	mustWork("delete user", "DELETE FROM users WHERE email='ok@example.com'")
	var left int
	if err := pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM sessions)+(SELECT count(*) FROM user_tokens)").Scan(&left); err != nil || left != 0 {
		t.Fatalf("sessions/tokens left after deleting the user: %d (%v)", left, err)
	}
}

// A database made by the pre-release baseline must be refused with a clear message, not fail halfway through.
func TestOpenRefusesOldBaselineDatabase(t *testing.T) {
	ctx := context.Background()
	url := testdb.New(t)
	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "CREATE TABLE accounts (id uuid PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	pool.Close()
	if _, err := database.Open(ctx, url); err == nil || !strings.Contains(err.Error(), "recreate") {
		t.Fatalf("expected a message telling the operator to recreate the database, got %v", err)
	}
}
