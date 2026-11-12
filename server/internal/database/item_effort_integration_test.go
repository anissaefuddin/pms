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

func TestItemEffortMigration(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 18); err != nil {
		t.Fatal(err)
	}
	var itemID string
	if err := db.QueryRowContext(ctx, `
WITH usr AS (
    INSERT INTO users(email) VALUES ('prior@example.test') RETURNING id
), item AS (
    INSERT INTO items(type,title,created_by,duration_days)
    SELECT 'task','Existing task',id,2 FROM usr RETURNING id,created_by
)
INSERT INTO item_assignees(item_id,user_id) SELECT id,created_by FROM item
RETURNING item_id::text`).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 19); err != nil {
		t.Fatal(err)
	}
	var intact bool
	err = db.QueryRowContext(ctx, `
SELECT i.title='Existing task' AND i.duration_days=2 AND i.effort_minutes IS NULL
    AND a.user_id=i.created_by AND a.allocation_unit IS NULL AND a.allocation_value IS NULL
FROM items i JOIN item_assignees a ON a.item_id=i.id WHERE i.id=$1`, itemID).Scan(&intact)
	if err != nil || !intact {
		t.Fatalf("prior item or assignment changed during migration: intact=%v, err=%v", intact, err)
	}
	for _, statement := range []string{
		"UPDATE items SET effort_minutes=-1 WHERE id=$1",
		"UPDATE item_assignees SET allocation_unit='percent' WHERE item_id=$1",
		"UPDATE item_assignees SET allocation_value=5 WHERE item_id=$1",
		"UPDATE item_assignees SET allocation_unit='percent',allocation_value=-1 WHERE item_id=$1",
		"UPDATE item_assignees SET allocation_unit='unknown',allocation_value=5 WHERE item_id=$1",
	} {
		_, err := db.ExecContext(ctx, statement, itemID)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("%s: expected CHECK violation, got %v", statement, err)
		}
	}
}
