package testdb_test

import (
	"context"
	"io/fs"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/anissaefuddin/pms/server/migrations"
	"github.com/jackc/pgx/v5"
)

func count(t *testing.T, url, sql string) int {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	var n int
	if err := c.QueryRow(ctx, sql).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// The template gives every test the full schema (ours and River's), and the copies do not share rows.
func TestMigratedDatabasesAreCompleteAndIndependent(t *testing.T) {
	a, b := testdb.NewMigrated(t), testdb.NewMigrated(t)
	if a == b {
		t.Fatal("two tests must not share a database")
	}
	files, _ := fs.Glob(migrations.FS, "*.sql")
	if applied := count(t, a, "SELECT count(*) FROM schema_migrations WHERE version_id>0"); applied != len(files) || count(t, a, "SELECT count(*) FROM information_schema.tables WHERE table_name='river_job'") != 1 {
		t.Fatal("a migrated database must hold every migration and River's tables")
	}
	if _, err := func() (any, error) {
		ctx := context.Background()
		c, err := pgx.Connect(ctx, a)
		if err != nil {
			return nil, err
		}
		defer c.Close(ctx)
		return c.Exec(ctx, "INSERT INTO groups(name) VALUES ('only here')")
	}(); err != nil {
		t.Fatal(err)
	}
	if count(t, b, "SELECT count(*) FROM groups WHERE name='only here'") != 0 {
		t.Fatal("a row written in one copy leaked into another")
	}
	if count(t, testdb.New(t), "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'") != 0 {
		t.Fatal("New must still return an empty database")
	}
}
