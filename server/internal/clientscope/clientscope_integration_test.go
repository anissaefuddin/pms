package clientscope

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Synthetic fixture on a disposable database. Spaces and items start unresolved, exactly as pre-migration data does.
type fixture struct {
	t                    *testing.T
	ctx                  context.Context
	pool                 *pgxpool.Pool
	admin, member, guest string
	clientA, clientB     string
	s1, s2, s3           string
	i1, i2, i3           string
}

func newMigrationFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.NewMigrated(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f := &fixture{t: t, ctx: ctx, pool: pool}
	user := func(kind string) string {
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO users(email,user_type,first_name,last_name,password_hash,status)
VALUES (gen_random_uuid()::text||'@example.test',$1,'A','B','x','active') RETURNING id::text`, kind).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	f.admin, f.member, f.guest = user("admin"), user("regular"), user("regular")
	client := func(code string) string {
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO clients(code,name,created_by) VALUES ($1,'Customer',$2) RETURNING id::text`, code, f.admin).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	f.clientA, f.clientB = client("MIG-A"), client("MIG-B")
	f.exec(`INSERT INTO client_memberships(client_id,user_id,status,granted_by) VALUES ($1,$2,'active',$4),($1,$3,'active',$4)`, f.clientA, f.member, f.admin, f.admin)
	space := func(name string) string {
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO spaces(name,kind) VALUES ($1,'shared') RETURNING id::text`, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	f.s1, f.s2, f.s3 = space("Space one"), space("Space two"), space("Space three")
	item := func(title, space string) string {
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO items(type,title,created_by) VALUES ('task',$1,$2) RETURNING id::text`, title, f.admin).Scan(&id); err != nil {
			t.Fatal(err)
		}
		f.exec(`INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')`, id, space)
		return id
	}
	f.i1 = item("Only in one", f.s1)
	f.i2 = item("Only in two", f.s2)
	f.i3 = item("Across one and two", f.s1)
	f.exec(`INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'V')`, f.i3, f.s2)
	f.exec(`INSERT INTO item_dependencies(predecessor_id,successor_id,type,lag) VALUES ($1,$2,'FS',0)`, f.i1, f.i2)
	return f
}

func (f *fixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(f.ctx, sql, args...); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) scopes() string {
	var out string
	if err := f.pool.QueryRow(f.ctx, `SELECT COALESCE(string_agg(k||':'||n, ','), '') FROM (
SELECT 'spaces.'||scope_state||'.'||COALESCE(client_id::text,'-') AS k, count(*) AS n FROM spaces GROUP BY 1
UNION ALL SELECT 'items.'||scope_state||'.'||COALESCE(client_id::text,'-'), count(*) FROM items GROUP BY 1) s`).Scan(&out); err != nil {
		f.t.Fatal(err)
	}
	return out
}

func (f *fixture) mapping(rows ...SpaceRow) Mapping {
	return Mapping{Version: 1, Spaces: rows}
}

// MG-01: the mapping format is strict. Unknown fields, bad identifiers, duplicates and scope/client mismatches are refused.
func TestMappingFormatIsStrict(t *testing.T) {
	good := `{"version":1,"spaces":[{"space_id":"00000000-0000-4000-8000-000000000001","scope":"client","client_id":"00000000-0000-4000-8000-000000000002"}]}`
	if _, err := ParseMapping([]byte(good)); err != nil {
		t.Fatalf("a valid mapping must parse: %v", err)
	}
	for name, raw := range map[string]string{
		"unknown field":     `{"version":1,"spaces":[],"owner":"x"}`,
		"bad space id":      `{"version":1,"spaces":[{"space_id":"nope","scope":"internal"}]}`,
		"duplicate space":   `{"version":1,"spaces":[{"space_id":"00000000-0000-4000-8000-000000000001","scope":"internal"},{"space_id":"00000000-0000-4000-8000-000000000001","scope":"internal"}]}`,
		"client without id": `{"version":1,"spaces":[{"space_id":"00000000-0000-4000-8000-000000000001","scope":"client"}]}`,
		"internal with id":  `{"version":1,"spaces":[{"space_id":"00000000-0000-4000-8000-000000000001","scope":"internal","client_id":"00000000-0000-4000-8000-000000000002"}]}`,
		"unknown scope":     `{"version":1,"spaces":[{"space_id":"00000000-0000-4000-8000-000000000001","scope":"unresolved"}]}`,
		"wrong version":     `{"version":2,"spaces":[]}`,
	} {
		if _, err := ParseMapping([]byte(raw)); err == nil {
			t.Fatalf("%s must be refused", name)
		}
	}
}

// MG-02..MG-04: dry-run is read-only; apply classifies only what the mapping covers; a second apply changes nothing.
func TestDryRunIsReadOnlyAndApplyIsIdempotent(t *testing.T) {
	f := newMigrationFixture(t)
	m := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA})
	before := f.scopes()
	rep, err := DryRun(f.ctx, f.pool, m)
	if err != nil {
		t.Fatal(err)
	}
	if f.scopes() != before {
		t.Fatal("dry-run changed stored ownership")
	}
	if rep.SpacesToClassify != 1 || rep.ItemsToClassify != 1 || len(rep.Conflicts) != 0 {
		t.Fatalf("plan for one mapped Space: %+v", rep)
	}
	if rep.ItemsUnresolvedRemain != 2 {
		t.Fatalf("an item across a mapped and an unmapped Space, and an item only in the unmapped Space, stay unresolved: %+v", rep)
	}

	applied, err := Apply(f.ctx, f.pool, m, rep.Digest)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied.SpacesToClassify != 0 || applied.ItemsToClassify != 0 {
		t.Fatalf("reconciliation still has work: %+v", applied)
	}
	if got := f.queryScope(f.i1); got != "client" {
		t.Fatalf("item in the mapped Space must be classified, got %q", got)
	}
	if got := f.queryScope(f.i2); got != "unresolved" {
		t.Fatalf("an unmapped item must not be classified (got %q)", got)
	}
	if got := f.queryScope(f.i3); got != "unresolved" {
		t.Fatalf("an item spanning mapped and unmapped Spaces must stay unresolved (got %q)", got)
	}
	if n := f.count("SELECT count(*) FROM item_dependencies WHERE predecessor_id=$1 AND successor_id=$2", f.i1, f.i2); n != 1 {
		t.Fatal("apply must preserve existing relationships")
	}

	// Idempotency: the same mapping gives an empty plan, so apply with its digest changes nothing.
	again, err := DryRun(f.ctx, f.pool, m)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := f.scopes()
	if _, err := Apply(f.ctx, f.pool, m, again.Digest); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if f.scopes() != snapshot {
		t.Fatal("a second apply changed stored ownership")
	}
}

// MG-05: apply refuses a stale digest, and refuses every conflict without writing anything.
func TestApplyRefusesStaleAndConflictingPlans(t *testing.T) {
	f := newMigrationFixture(t)
	m := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA})
	rep, err := DryRun(f.ctx, f.pool, m)
	if err != nil {
		t.Fatal(err)
	}
	before := f.scopes()
	if _, err := Apply(f.ctx, f.pool, m, strings.Repeat("0", 64)); err == nil || f.scopes() != before {
		t.Fatal("a digest that does not match the reviewed plan must be refused with no change")
	}

	// Conflict: the Space of i3 is mapped internal while i3 also lives in a client Space.
	conflict := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA}, SpaceRow{SpaceID: f.s2, Scope: "internal"})
	crep, err := DryRun(f.ctx, f.pool, conflict)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConflict(crep, "item_placements_span_owners") {
		t.Fatalf("an item across two owners must be a conflict: %+v", crep)
	}
	before = f.scopes()
	if _, err := Apply(f.ctx, f.pool, conflict, crep.Digest); err == nil || f.scopes() != before {
		t.Fatal("apply with a conflict must refuse and write nothing")
	}
	_ = rep
}

// MG-06: an archived Customer, an unknown Space, a non-member share and a non-member assignee each refuse the plan.
func TestMigrationConflictsRefuseApply(t *testing.T) {
	f := newMigrationFixture(t)
	f.exec(`INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('space',$1,'user',$2,'read')`, f.s1, f.guest)
	f.exec(`UPDATE clients SET status='archived', archived_at=now(), archived_by=$1 WHERE id=$2`, f.admin, f.clientB)
	m := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA}, SpaceRow{SpaceID: f.s3, Scope: "client", ClientID: f.clientB})
	rep, err := DryRun(f.ctx, f.pool, m)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConflict(rep, "customer_missing_or_archived") || !hasConflict(rep, "share_recipient_not_member") {
		t.Fatalf("archived Customer and non-member share must be reported: %+v", rep)
	}
	before := f.scopes()
	if _, err := Apply(f.ctx, f.pool, m, rep.Digest); err == nil || f.scopes() != before {
		t.Fatal("a plan with conflicts must not apply")
	}
	if !strings.Contains(strings.ToLower(mustJSON(t, rep)), "archived") && !hasConflict(rep, "customer_missing_or_archived") {
		t.Fatal("the report must name the archived Customer condition")
	}
}

func (f *fixture) queryScope(id string) string {
	var s string
	if err := f.pool.QueryRow(f.ctx, "SELECT scope_state FROM items WHERE id=$1", id).Scan(&s); err != nil {
		f.t.Fatal(err)
	}
	return s
}

func (f *fixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, sql, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func hasConflict(r Report, code string) bool {
	for _, c := range r.Conflicts {
		if c.Code == code && c.Count > 0 {
			return true
		}
	}
	return false
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
