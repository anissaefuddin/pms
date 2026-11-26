package database_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/anissaefuddin/pms/server/migrations"
	"github.com/pressly/goose/v3"
)

// DEF-050: only the creator may be inferred as Space Admin. A self-created Full share also arises when someone joins a public Space
// and is later upgraded, or adds a direct share for themself; the creator's own share is the one written with the Space.
func TestSpaceAdminBackfillPromotesOnlyTheCreator(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("pgx", testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS, goose.WithTableName("schema_migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 354); err != nil {
		t.Fatal(err)
	}
	exec := func(stmt string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%v\n%s", err, stmt)
		}
	}
	exec(`INSERT INTO users(email,first_name,last_name,password_hash,user_type,status) VALUES
 ('owner@x.test','O','O','x','admin','active'),('maker@x.test','M','M','x','regular','active'),
 ('later@x.test','L','L','x','regular','active'),('gone@x.test','G','G','x','regular','active')`)
	exec(`INSERT INTO account(id,name,owner_id) SELECT 1,'X',id FROM users WHERE email='owner@x.test'`)
	// Each Space in its own statement, so that its creator's share carries the same moment, as the application writes them.
	space := func(name, creator string) {
		exec(`WITH s AS (INSERT INTO spaces(name,kind) VALUES ('` + name + `','shared') RETURNING id)
 INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,created_by) SELECT 'space',s.id,'user',u.id,'full',u.id FROM s, users u WHERE u.email='` + creator + `'`)
	}
	later := func(name, who string) { // a self-created Full share that appeared afterwards
		exec(`INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,created_by)
 SELECT 'space',s.id,'user',u.id,'full',u.id FROM spaces s, users u WHERE s.name='` + name + `' AND u.email='` + who + `'`)
	}
	space("Kept", "maker@x.test")
	later("Kept", "later@x.test")
	space("CreatorGone", "gone@x.test")
	later("CreatorGone", "later@x.test")
	space("CreatorLeft", "maker@x.test")
	later("CreatorLeft", "later@x.test")
	exec(`DELETE FROM shares sh USING spaces s, users u WHERE sh.target_id=s.id AND sh.principal_id=u.id AND s.name='CreatorLeft' AND u.email='maker@x.test'`)
	exec(`UPDATE users SET status='deactivated' WHERE email='gone@x.test'`)
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"Kept": "maker@x.test", "CreatorGone": "", "CreatorLeft": ""} {
		var got string
		if err := db.QueryRowContext(ctx, `SELECT COALESCE(string_agg(u.email::text,','),'') FROM space_admins sa JOIN spaces s ON s.id=sa.space_id JOIN users u ON u.id=sa.user_id WHERE s.name=$1`, name).Scan(&got); err != nil || got != want {
			t.Errorf("%s: admin %q, want %q (%v)", name, got, want, err)
		}
	}
}
