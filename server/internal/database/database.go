package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"strconv"
	"strings"

	"github.com/anissaefuddin/pms/server/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := migrateLocked(ctx, pool, databaseURL); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// migrateKey is the advisory-lock id that serializes migrations: two processes starting together (the app and `pms bootstrap`)
// otherwise both apply the same migration and fail with "already exists", leaving the database half migrated (DR-05).
const migrateKey = 7_201_127

// migrateLocked holds a session advisory lock while migrating; a second process waits, then finds nothing left to apply.
// The lock dies with the connection, so a crashed process cannot leave it held.
func migrateLocked(ctx context.Context, pool *pgxpool.Pool, databaseURL string) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrateKey); err != nil {
		return fmt.Errorf("wait for migration lock: %w", err)
	}
	defer func() { _, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", migrateKey) }()
	if err := migrate(ctx, databaseURL); err != nil {
		return err
	}
	return migrateRiver(ctx, pool)
}

func migrate(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	// The baseline was rewritten (decision D-13): goose tracks version numbers, not file contents, so a database made by
	// the old baseline would fail halfway with a confusing "already exists" error. Say what to do instead.
	var oldBaseline bool
	if err := db.QueryRowContext(ctx, "SELECT to_regclass('public.accounts') IS NOT NULL").Scan(&oldBaseline); err != nil {
		return err
	}
	if oldBaseline {
		return fmt.Errorf("this database was created by the old pre-release schema (table accounts); recreate it and run `pms bootstrap` again, do not restore it under this version")
	}
	goose.SetBaseFS(migrations.FS)
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS, goose.WithTableName("schema_migrations"))
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	for _, result := range results {
		slog.Info("migration applied", "source", result.Source.Path)
	}
	return nil
}

// Ready reports whether every embedded migration version is recorded as applied; /readyz uses it.
// Versions are compared by number, so a database created by an older, rewritten baseline is not "ready".
func Ready(ctx context.Context, pool *pgxpool.Pool) error {
	files, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return err
	}
	versions := make([]int64, 0, len(files))
	for _, f := range files {
		v, _, found := strings.Cut(f, "_")
		n, err := strconv.ParseInt(v, 10, 64)
		if !found || err != nil {
			return fmt.Errorf("migration file %q does not start with a version number", f)
		}
		versions = append(versions, n)
	}
	var applied int
	if err := pool.QueryRow(ctx, "SELECT count(DISTINCT version_id) FROM schema_migrations WHERE is_applied AND version_id = ANY($1)", versions).Scan(&applied); err != nil {
		return err
	}
	if applied < len(versions) {
		return fmt.Errorf("%d of %d migrations applied", applied, len(versions))
	}
	return nil
}

func migrateRiver(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create River migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("migrate River: %w", err)
	}
	return nil
}
