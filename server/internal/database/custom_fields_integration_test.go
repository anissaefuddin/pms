package database_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/anissaefuddin/pms/server/migrations"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"
)

func TestCustomFieldTypesMigration(t *testing.T) {
	ctx := t.Context()
	db, err := sql.Open("pgx", testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS, goose.WithTableName("schema_migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 19); err != nil {
		t.Fatal(err)
	}
	var spaceID, fieldID string
	if err := db.QueryRowContext(ctx, `
WITH scope AS (INSERT INTO spaces(name,kind) VALUES ('Existing','shared') RETURNING id)
INSERT INTO custom_fields(space_id,name,type,options,position)
SELECT id,'Old select','single_select','[{"id":"12345678-1234-4234-8234-123456789abc","label":"Old option","color":"#334455"}]',0 FROM scope
RETURNING space_id::text,id::text`).Scan(&spaceID, &fieldID); err != nil {
		t.Fatal(err)
	}
	var itemID string
	if err := db.QueryRowContext(ctx, `
WITH creator AS (INSERT INTO users(email) VALUES ('old-field@example.test') RETURNING id)
INSERT INTO items(type,title,created_by,fields)
SELECT 'task','Previously selected',id,jsonb_build_object($1::text,'12345678-1234-4234-8234-123456789abc') FROM creator
RETURNING id::text`, fieldID).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 20); err != nil {
		t.Fatal(err)
	}
	var intact bool
	err = db.QueryRowContext(ctx, `
SELECT f.name='Old select' AND f.type='single_select' AND f.options->0->>'label'='Old option' AND f.config='{}'
  AND i.fields->>f.id::text='12345678-1234-4234-8234-123456789abc'
FROM custom_fields f CROSS JOIN items i WHERE f.id=$1 AND i.id=$2`, fieldID, itemID).Scan(&intact)
	if err != nil || !intact {
		t.Fatalf("prior definition changed during migration: intact=%v, err=%v", intact, err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO custom_fields(space_id,name,type,options,position) VALUES ($1,'Text','text','[]',1)", spaceID); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"UPDATE custom_fields SET options='[]' WHERE id=$1",
		"UPDATE custom_fields SET options='{}' WHERE id=$1",
		"UPDATE custom_fields SET type='text' WHERE id=$1",
		"UPDATE custom_fields SET type='unknown' WHERE id=$1",
		"UPDATE custom_fields SET config='[]' WHERE id=$1",
	} {
		_, err := db.ExecContext(ctx, statement, fieldID)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("%s: expected CHECK violation, got %v", statement, err)
		}
	}
}
