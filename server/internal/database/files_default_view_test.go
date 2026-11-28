package database_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/anissaefuddin/pms/server/migrations"
	"github.com/pressly/goose/v3"
)

// Migration 00369: Files becomes the default of Spaces, projects and folders. Existing scopes move only while they open on Table.
func TestFilesBecomesTheDefaultView(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 368); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	one := func(q string, args ...any) (out string) {
		t.Helper()
		if err := db.QueryRow(q, args...).Scan(&out); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return out
	}
	exec(`INSERT INTO users(email,first_name,last_name,password_hash,user_type,status) VALUES ('a@example.com','A','B','x','admin','active')`)
	space := func(name string) string {
		return one(`INSERT INTO spaces(name,kind,scope_state) VALUES ($1,'shared','internal') RETURNING id::text`, name)
	}
	item := func(kind, title string, view any) string {
		return one(`INSERT INTO items(type,title,default_view_type,created_by,scope_state) SELECT $1::item_type,$2,$3,id,'internal' FROM users WHERE email='a@example.com' RETURNING id::text`, kind, title, view)
	}
	plain, onBoard := space("plain"), space("board")
	exec(`UPDATE views SET is_default=false WHERE scope_space_id=$1`, onBoard)
	exec(`UPDATE views SET is_default=true WHERE scope_space_id=$1 AND type='board'`, onBoard)
	table, folder := item("project", "table", nil), item("folder", "folder", "table")
	gantt, files := item("project", "gantt", "gantt"), item("project", "files", "files")
	// A personal Files view cannot be a default: the scope gets a public one and the personal view stays.
	personal := item("folder", "personal", nil)
	exec(`INSERT INTO views(scope_item_id,type,name,position,visibility,owner_id) SELECT $1,'files','Mine',5,'personal',id FROM users WHERE email='a@example.com'`, personal)
	task := item("task", "task", nil)
	count := one(`SELECT count(*)::text FROM views`)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up 00369: %v", err)
	}
	opens := func(scope string) string {
		return one(`SELECT COALESCE(string_agg(type::text||':'||visibility::text, ','),'') FROM views WHERE is_default AND (scope_space_id=$1 OR scope_item_id=$1)`, scope)
	}
	for scope, want := range map[string]string{plain: "files:public", onBoard: "board:public", table: "files:public", folder: "files:public",
		gantt: "gantt:public", files: "files:public", personal: "files:public", task: ""} {
		if got := opens(scope); got != want {
			t.Errorf("scope %s opens on %q, want %q", scope, got, want)
		}
	}
	// Only the missing Files views were added: plain, table, folder and personal; nothing was removed or duplicated.
	if got := one(`SELECT (count(*)-$1::int)::text FROM views`, count); got != "4" {
		t.Errorf("views added = %s, want 4", got)
	}
	// New containers.
	fresh := space("fresh")
	for scope, want := range map[string]string{fresh: "files:public", item("project", "new", nil): "files:public", item("folder", "new folder", nil): "files:public",
		item("project", "chosen", "board"): "board:public", item("project", "chart", "chart"): "chart:public"} {
		if got := opens(scope); got != want {
			t.Errorf("new scope %s opens on %q, want %q", scope, got, want)
		}
	}
	if got := one(`SELECT string_agg(type::text, ',' ORDER BY position) FROM views WHERE scope_space_id=$1`, fresh); got != "table,board,gantt,calendar,files" {
		t.Errorf("new Space views = %s", got)
	}
	if _, err := provider.DownTo(ctx, 368); err != nil {
		t.Fatalf("down 00369: %v", err)
	}
	if got := opens(space("after down")); got != "table:public" {
		t.Errorf("after down a new Space opens on %q", got)
	}
}
