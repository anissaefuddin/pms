package clientscope

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

// B3-21 rehearsal on a disposable database: back up before the migration, apply it, then restore the backup into a fresh database. The
// restored copy must match the pre-migration state, and the same approved mapping must reproduce the migrated state there.
// Requires the PostgreSQL client tools (pg_dump, pg_restore); the test is skipped when they are missing.
func TestRecoveryRehearsalRestoresThenReappliesTheSameMapping(t *testing.T) {
	for _, tool := range []string{"pg_dump", "pg_restore"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed; the recovery rehearsal did not run", tool)
		}
	}
	f := newMigrationFixture(t)
	m := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA})
	before := scopeCounts(t, f.ctx, f.pool)

	dump := filepath.Join(t.TempDir(), "pre-migration.dump")
	run(t, "pg_dump", "--format=custom", "--no-owner", "--file="+dump, "--dbname="+f.url)

	rep, err := DryRun(f.ctx, f.pool, m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(f.ctx, f.pool, m, rep.Digest); err != nil {
		t.Fatalf("migration: %v", err)
	}
	migrated := scopeCounts(t, f.ctx, f.pool)
	if migrated == before {
		t.Fatal("the rehearsal migration must change something to be a meaningful test")
	}

	target := testdb.New(t)
	restoreSQL(t, dump, target)
	restored, err := database.Open(f.ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if got := scopeCounts(t, f.ctx, restored); got != before {
		t.Fatalf("restored backup differs from the pre-migration state:\nwant %s\ngot  %s", before, got)
	}
	rrep, err := DryRun(f.ctx, restored, m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(f.ctx, restored, m, rrep.Digest); err != nil {
		t.Fatalf("re-apply on the restored copy: %v", err)
	}
	if got := scopeCounts(t, f.ctx, restored); got != migrated {
		t.Fatalf("re-applying the approved mapping did not reproduce the migrated state:\nwant %s\ngot  %s", migrated, got)
	}
}

// restoreSQL loads the custom-format dump through psql with ON_ERROR_STOP. A pg_dump 17+ header sets transaction_timeout, which a
// PostgreSQL 16 server (the CI and production image) does not know, so that one SET line is removed. Nothing else is changed.
func restoreSQL(t *testing.T, dump, target string) {
	t.Helper()
	sqlPath := filepath.Join(filepath.Dir(dump), "restore.sql")
	run(t, "pg_restore", "--no-owner", "--file="+sqlPath, dump)
	raw, err := os.ReadFile(sqlPath)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "SET transaction_timeout") {
			kept = append(kept, line)
		}
	}
	if err := os.WriteFile(sqlPath, []byte(strings.Join(kept, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	run(t, "psql", "--set=ON_ERROR_STOP=1", "--quiet", "--dbname="+target, "--file="+sqlPath)
}

func run(t *testing.T, name string, args ...string) {
	t.Helper()
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", name, err, out)
	}
}

// scopeCounts is the same ownership summary as fixture.scopes, read from any pool.
func scopeCounts(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	var out string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(string_agg(k||':'||n, ','), '') FROM (
SELECT 'spaces.'||scope_state||'.'||COALESCE(client_id::text,'-') AS k, count(*) AS n FROM spaces GROUP BY 1
UNION ALL SELECT 'items.'||scope_state||'.'||COALESCE(client_id::text,'-'), count(*) FROM items GROUP BY 1) s ORDER BY 1`).Scan(&out); err != nil {
		t.Fatal(err)
	}
	return out
}
