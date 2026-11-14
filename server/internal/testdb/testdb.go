// Package testdb gives each integration test its own database, so tests can be re-run and run in parallel.
// New returns an empty one; NewMigrated returns one cloned from a template that already holds every migration, which is what
// nearly all tests want: applying several hundred migrations per test cost seconds each.
package testdb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/migrations"
	"github.com/jackc/pgx/v5"
)

// lockKey serializes template creation and cloning across the test binaries of one `go test ./...` run: Postgres refuses to copy a
// template that another session is using, so two clones at once would fail.
const lockKey = 7_201_126

func connect(t *testing.T) (baseURL string, ctx context.Context) {
	t.Helper()
	baseURL = os.Getenv("PMS_TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("PMS_TEST_DATABASE_URL is not set")
	}
	return baseURL, context.Background()
}

func urlFor(t *testing.T, baseURL, name string) string {
	t.Helper()
	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}

func drop(ctx context.Context, baseURL, name string) {
	if c, err := pgx.Connect(ctx, baseURL); err == nil {
		_, _ = c.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		c.Close(ctx)
	}
}

// New creates an empty database on the server in PMS_TEST_DATABASE_URL and returns its URL.
// The test is skipped when the variable is unset; the database is dropped when the test ends.
func New(t *testing.T) string {
	t.Helper()
	baseURL, ctx := connect(t)
	name := fmt.Sprintf("pms_test_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(ctx, baseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { drop(ctx, baseURL, name) })
	return urlFor(t, baseURL, name)
}

// templateName changes whenever a migration, or the River version whose own migrations run after ours, changes.
func templateName() (string, error) {
	h := sha256.New()
	err := fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(migrations.FS, path)
		h.Write([]byte(path))
		h.Write(body)
		return err
	})
	if err != nil {
		return "", err
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if strings.HasPrefix(dep.Path, "github.com/riverqueue/river") {
				h.Write([]byte(dep.Path + dep.Version))
			}
		}
	}
	return "pms_tpl_" + hex.EncodeToString(h.Sum(nil))[:16], nil
}

// NewMigrated returns the URL of a fresh database that already has every migration applied (also River's).
// The first call per migration set builds the template; later ones, in any test binary, only copy it.
func NewMigrated(t *testing.T) string {
	t.Helper()
	baseURL, ctx := connect(t)
	tpl, err := templateName()
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("pms_test_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(ctx, baseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockKey) //nolint:errcheck // the session ends with the test anyway
	var ready bool
	if err := admin.QueryRow(ctx, "SELECT COALESCE((SELECT shobj_description(oid,'pg_database')='ready' FROM pg_database WHERE datname=$1), false)", tpl).Scan(&ready); err != nil {
		t.Fatal(err)
	}
	if !ready {
		if err := buildTemplate(ctx, admin, baseURL, tpl, t); err != nil {
			t.Fatalf("build the test template: %v", err)
		}
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name+" TEMPLATE "+tpl); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { drop(ctx, baseURL, name) })
	return urlFor(t, baseURL, name)
}

// buildTemplate runs under the lock. Templates for older migration sets are dropped here so they do not pile up.
func buildTemplate(ctx context.Context, admin *pgx.Conn, baseURL, tpl string, t *testing.T) error {
	rows, err := admin.Query(ctx, "SELECT datname FROM pg_database WHERE datname LIKE 'pms_tpl_%'")
	if err != nil {
		return err
	}
	var stale []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		stale = append(stale, name)
	}
	rows.Close()
	for _, name := range stale { // old sets, and a half-built one left by a crash
		if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			return err
		}
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+tpl); err != nil {
		return err
	}
	pool, err := database.Open(ctx, urlFor(t, baseURL, tpl))
	if err != nil {
		return err
	}
	pool.Close()
	_, err = admin.Exec(ctx, "COMMENT ON DATABASE "+tpl+" IS 'ready'")
	return err
}
