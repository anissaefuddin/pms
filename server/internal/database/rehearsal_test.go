package database_test

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/anissaefuddin/pms/server/migrations"
	"github.com/pressly/goose/v3"
)

// Migration rehearsal (S12.2.1): an instance that was installed at the M0-M5 schema (version 45) and holds real data is brought to the
// current schema the way an update does it, and nothing it held is lost or broken. Forward-only migrations are what the update script
// runs, so this is the one path that matters; going back is a restore from the pre-update dump (deploy/update.sh).
func TestUpgradeFromTheEarlySchemaKeepsEveryRow(t *testing.T) {
	ctx := context.Background()
	url := testdb.New(t)
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS, goose.WithTableName("schema_migrations"))
	if err != nil {
		t.Fatal(err)
	}
	const baseline = 45
	if _, err := provider.UpTo(ctx, baseline); err != nil {
		t.Fatalf("migrate to the early schema: %v", err)
	}

	// A small instance, written with the columns the early schema had.
	for _, stmt := range []string{
		`INSERT INTO account(id,name) VALUES (1,'Rehearsal Co')`,
		`INSERT INTO users(email,first_name,last_name,password_hash,user_type,status) VALUES
		   ('owner@example.com','Olive','Owner','x','admin','active'),
		   ('member@example.com','Mia','Member','x','regular','active'),
		   ('viewer@example.com','Vic','Viewer','x','viewer','active'),
		   ('pending@example.com','','',NULL,'regular','invited')`,
		`INSERT INTO spaces(name,kind,owner_id) SELECT 'Personal','personal',id FROM users WHERE status='active'`,
		`INSERT INTO spaces(name,kind) VALUES ('General','shared'),('Secret','shared')`,
		`INSERT INTO groups(name) VALUES ('Everyone')`,
		`INSERT INTO group_members(group_id,user_id) SELECT g.id,u.id FROM groups g CROSS JOIN users u WHERE u.status='active'`,
		`INSERT INTO shares(target_type,target_id,principal_type,principal_id,role)
		   SELECT 'space',s.id,'group',g.id,'editor' FROM spaces s CROSS JOIN groups g WHERE s.name='General'`,
		`INSERT INTO shares(target_type,target_id,principal_type,principal_id,role)
		   SELECT 'space',s.id,'user',u.id,'full' FROM spaces s JOIN users u ON u.email='owner@example.com' WHERE s.name='Secret'`,
		`WITH w AS (INSERT INTO workflows(name,is_default) VALUES ('Default',true) RETURNING id)
		   INSERT INTO statuses(workflow_id,name,grp,color,position) SELECT id,'New','active'::status_group,'slate',0 FROM w UNION ALL SELECT id,'Done','completed'::status_group,'green',1 FROM w`,
		`INSERT INTO items(type,title,created_by,status_id) SELECT 'task','Task '||g,(SELECT id FROM users WHERE email='owner@example.com'),(SELECT id FROM statuses WHERE name='New') FROM generate_series(1,200) g`,
		`INSERT INTO item_locations(item_id,space_id,rank) SELECT i.id,(SELECT id FROM spaces WHERE name='General'),'A'||lpad(row_number() OVER (ORDER BY i.title)::text,5,'0') FROM items i`,
		`INSERT INTO comments(id,item_id,author_id,body,body_text) SELECT gen_random_uuid(),i.id,u.id,'{"type":"doc"}','hello' FROM items i JOIN users u ON u.email='member@example.com' WHERE i.title LIKE 'Task 1%'`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed the early schema: %v\n%s", err, stmt)
		}
	}
	counts := func() map[string]int {
		out := map[string]int{}
		for name, q := range map[string]string{
			"users": "SELECT count(*) FROM users", "spaces": "SELECT count(*) FROM spaces", "shares": "SELECT count(*) FROM shares",
			"items": "SELECT count(*) FROM items", "locations": "SELECT count(*) FROM item_locations", "comments": "SELECT count(*) FROM comments",
			"groups": "SELECT count(*) FROM group_members", "statuses": "SELECT count(*) FROM statuses",
		} {
			var n int
			if err := db.QueryRowContext(ctx, q).Scan(&n); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
			out[name] = n
		}
		return out
	}
	before := counts()
	if before["items"] != 200 || before["comments"] == 0 {
		t.Fatalf("the seed did not land: %v", before)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		t.Fatalf("upgrade to the current schema: %v", err)
	}
	files, _ := fs.Glob(migrations.FS, "*.sql")
	if len(results) == 0 || len(results) > len(files)-baseline+5 {
		t.Fatalf("applied %d migrations over a baseline of %d with %d files", len(results), baseline, len(files))
	}
	after := counts()
	for name, n := range before {
		if after[name] != n {
			t.Errorf("%s: %d rows before the upgrade, %d after", name, n, after[name])
		}
	}

	// What the upgrade must add, and what it must not break.
	var owner string
	if err := db.QueryRowContext(ctx, "SELECT COALESCE((SELECT email::text FROM users WHERE id=owner_id),'') FROM account WHERE id=1").Scan(&owner); err != nil || owner != "owner@example.com" {
		t.Errorf("the oldest administrator must become the account owner, got %q (%v)", owner, err)
	}
	var seats, locked int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FILTER (WHERE status='active'), (SELECT count(*) FROM spaces WHERE locked) FROM users").Scan(&seats, &locked); err != nil || seats != 3 || locked != 0 {
		t.Errorf("active users %d, locked spaces %d (%v)", seats, locked, err)
	}
	var itemTypes int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM items WHERE type_id IS NULL").Scan(&itemTypes); err != nil || itemTypes != 0 {
		t.Errorf("every existing item must have been given an item type: %d without (%v)", itemTypes, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM audit_events").Scan(&itemTypes); err != nil {
		t.Errorf("the audit table must exist: %v", err)
	}
	pool, err := database.Open(ctx, url) // what the server does on start: nothing left to apply
	if err != nil {
		t.Fatalf("the server must start on an upgraded database: %v", err)
	}
	defer pool.Close()
	if err := database.Ready(ctx, pool); err != nil {
		t.Fatalf("readyz after the upgrade: %v", err)
	}
	var version int
	if err := db.QueryRowContext(ctx, "SELECT max(version_id) FROM schema_migrations").Scan(&version); err != nil || version < 311 {
		t.Errorf("schema version after the upgrade: %d (%v)", version, err)
	}
}
