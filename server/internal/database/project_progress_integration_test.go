package database_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/anissaefuddin/pms/server/migrations"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"
)

func TestProjectProgressMigration(t *testing.T) {
	ctx := t.Context()
	db, err := sql.Open("pgx", testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS, goose.WithTableName("schema_migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 20); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
WITH usr AS (INSERT INTO users(email) VALUES ('progress@example.test') RETURNING id)
INSERT INTO items(type,title,created_by,duration_days)
SELECT kind::item_type,title,id,2 FROM usr
CROSS JOIN (VALUES ('project','Existing project'),('task','Existing task')) v(kind,title)`); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 21); err != nil {
		t.Fatal(err)
	}
	var intact int
	err = db.QueryRowContext(ctx, `
SELECT count(*) FROM items WHERE title IN ('Existing project','Existing task')
AND duration_days=2 AND progress_method IS NULL AND progress_percent IS NULL`).Scan(&intact)
	if err != nil || intact != 2 {
		t.Fatalf("existing items changed during migration: count=%d, err=%v", intact, err)
	}
	for _, query := range []string{
		"UPDATE items SET progress_method='invalid' WHERE type='project'",
		"UPDATE items SET progress_method='task_count' WHERE type='task'",
		"UPDATE items SET progress_percent=50 WHERE type='project'",
		"UPDATE items SET progress_percent=-1 WHERE type='task'",
		"UPDATE items SET progress_percent=101 WHERE type='task'",
	} {
		_, err := db.ExecContext(ctx, query)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("%s: expected CHECK violation, got %v", query, err)
		}
	}
}
