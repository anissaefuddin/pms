package seed_test

import (
	"context"
	"sync"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/seed"
	"github.com/anissaefuddin/pms/server/internal/testdb"
)

func TestParse(t *testing.T) {
	got, err := seed.Parse(" project-master , ,project-master")
	if err != nil || len(got) != 1 || got[0] != "project-master" {
		t.Fatalf("Parse = %v, %v; want [project-master]", got, err)
	}
	if got, err := seed.Parse(""); err != nil || len(got) != 0 {
		t.Fatalf("empty PMS_SEED = %v, %v; want nothing", got, err)
	}
	if _, err := seed.Parse("project-mastr"); err == nil {
		t.Fatal("a typo must fail the start")
	}
}

// MVP-03: the project-master seed creates the contract fields once, restricts the value to its group, and never
// re-imposes itself after an admin changes things.
func TestProjectMasterSeed(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.NewMigrated(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	count := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return n
	}
	// An admin already made one of the fields by hand: it is reused, not duplicated.
	if _, err := pool.Exec(ctx, "INSERT INTO custom_fields(space_id,name,type,options,position,config) VALUES (NULL,'contract/po number','text','[]',0,'{}')"); err != nil {
		t.Fatal(err)
	}

	// Two instances starting together apply it once.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- seed.Apply(ctx, pool, []string{"project-master"}) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	if n := count("SELECT count(*) FROM custom_fields WHERE space_id IS NULL AND lower(name)='contract/po number'"); n != 1 {
		t.Fatalf("Contract/PO Number fields = %d, want the hand-made one only", n)
	}
	if n := count("SELECT count(*) FROM custom_fields WHERE space_id IS NULL AND name='Contract Value' AND type='currency' AND config->>'currency'='IDR' AND (config->>'decimals')::int=0 AND config->>'project_budget' IS NULL"); n != 1 {
		t.Fatalf("Contract Value fields = %d, want one IDR currency field that is not a budget", n)
	}
	if n := count(`SELECT count(*) FROM custom_field_grants g JOIN custom_fields f ON f.id=g.field_id JOIN groups gr ON gr.id=g.subject_id
WHERE f.name='Contract Value' AND g.subject_kind='group' AND gr.name='Contract Managers' AND g.level='edit'`); n != 1 {
		t.Fatalf("Contract Value grants = %d, want edit for Contract Managers", n)
	}
	if n := count("SELECT count(*) FROM custom_field_grants g JOIN custom_fields f ON f.id=g.field_id WHERE lower(f.name) IN ('contract/po number','pydio folder')"); n != 0 {
		t.Fatalf("Contract/PO Number and Pydio Folder grants = %d, want none (visible with the item)", n)
	}
	if n := count("SELECT count(*) FROM custom_fields WHERE space_id IS NULL AND name='Pydio Folder' AND type='text'"); n != 1 {
		t.Fatalf("Pydio Folder fields = %d, want one text field", n)
	}
	if n := count("SELECT count(*) FROM audit_events WHERE actor_name='seed' AND action='custom_field.grants_changed'"); n != 1 {
		t.Fatalf("audit rows = %d, want one for the restriction", n)
	}

	// After an admin renames the field, a restart does not bring the original back.
	if _, err := pool.Exec(ctx, "UPDATE custom_fields SET name='Nilai Kontrak' WHERE name='Contract Value'"); err != nil {
		t.Fatal(err)
	}
	if err := seed.Apply(ctx, pool, []string{"project-master"}); err != nil {
		t.Fatal(err)
	}
	if n := count("SELECT count(*) FROM custom_fields WHERE space_id IS NULL AND type='currency'"); n != 1 {
		t.Fatalf("currency fields after restart = %d, want 1", n)
	}
	if n := count("SELECT count(*) FROM seed_runs WHERE name='project-master'"); n != 1 {
		t.Fatalf("seed_runs = %d, want 1", n)
	}
}
