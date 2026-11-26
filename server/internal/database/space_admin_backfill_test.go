package database_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/anissaefuddin/pms/server/migrations"
	"github.com/pressly/goose/v3"
)

// 00355 (DEF-001): Spaces made while creation did not name an admin get their creator; a plain Full share is not promoted; only an
// empty orphan goes to the account owner.
func TestSpaceAdminBackfill(t *testing.T) {
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
	// One transaction, so a share written with its Space carries the same moment, as the application writes them.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO users(email,first_name,last_name,password_hash,user_type,status) VALUES
 ('owner@x.test','O','O','x','admin','active'),('maker@x.test','M','M','x','regular','active'),('gone@x.test','G','G','x','regular','active')`,
		`INSERT INTO account(id,name,owner_id) SELECT 1,'X',id FROM users WHERE email='owner@x.test'`,
		`INSERT INTO spaces(name,kind) SELECT n,'shared' FROM unnest(ARRAY['General','Made','OnlyFull','GoneMaker','OrphanWithItems','HasAdmin']) n`,
		`INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,created_by)
 SELECT 'space',s.id,'user',p.id,'full',c.id FROM (VALUES ('Made','maker@x.test','maker@x.test'),('OnlyFull','maker@x.test','owner@x.test'),('GoneMaker','gone@x.test','gone@x.test'),('HasAdmin','maker@x.test','maker@x.test')) v(s,p,c)
 JOIN spaces s ON s.name=v.s JOIN users p ON p.email=v.p JOIN users c ON c.email=v.c`,
		`UPDATE users SET status='deactivated' WHERE email='gone@x.test'`,
		`INSERT INTO space_admins(space_id,user_id) SELECT s.id,u.id FROM spaces s, users u WHERE s.name='HasAdmin' AND u.email='owner@x.test'`,
		`WITH i AS (INSERT INTO items(type,title,created_by) SELECT 'task','T',id FROM users WHERE email='owner@x.test' RETURNING id)
 INSERT INTO item_locations(item_id,space_id,rank) SELECT i.id,s.id,'A' FROM i, spaces s WHERE s.name='OrphanWithItems'`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%v\n%s", err, stmt)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 355); err != nil {
		t.Fatal(err)
	}
	for space, want := range map[string]string{"General": "owner@x.test", "Made": "maker@x.test", "OnlyFull": "", "GoneMaker": "", "OrphanWithItems": "", "HasAdmin": "owner@x.test"} {
		var got string
		if err := db.QueryRowContext(ctx, `SELECT COALESCE(string_agg(u.email::text,','),'') FROM space_admins sa JOIN spaces s ON s.id=sa.space_id JOIN users u ON u.id=sa.user_id WHERE s.name=$1`, space).Scan(&got); err != nil || got != want {
			t.Errorf("%s: admin %q, want %q (%v)", space, got, want, err)
		}
	}
	var shares int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM shares sh JOIN spaces s ON s.id=sh.target_id AND s.name='General' AND sh.role='full'").Scan(&shares); err != nil || shares != 1 {
		t.Errorf("General shares = %d (%v)", shares, err)
	}
}
