package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/bootstrap"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestItemLocationRejectsCycle(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	admin, err := bootstrap.Run(ctx, pool, "Test", "admin@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	exec := func(sql string) error {
		_, err := pool.Exec(ctx, sql)
		return err
	}
	// Raw fixture rows are internal by intent: they sit in the internal General Space (an unresolved row would be hidden).
	if err := exec("INSERT INTO items(type,title,created_by,scope_state) SELECT 'task',name,id,'internal' FROM users CROSS JOIN (VALUES ('A'), ('B')) AS titles(name)"); err != nil {
		t.Fatal(err)
	}
	const link = "INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) SELECT child.id,s.id,parent.id,'a' FROM items child CROSS JOIN items parent CROSS JOIN spaces s WHERE "
	if err := exec(link + "child.title='A' AND parent.title='B' AND s.name='General'"); err != nil {
		t.Fatal(err)
	}
	var itemID string
	if err := pool.QueryRow(ctx, "SELECT id::text FROM items WHERE title='A'").Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	// D-17 A1: the admin edits through the explicit grant bootstrap gives on General.
	caps, err := policy.ItemCaps(ctx, pool, policy.Actor{ID: admin.AdminID, Type: policy.Admin}, itemID)
	if err != nil || !caps.Has(policy.Edit) {
		t.Fatalf("admin access through shared Space: caps=%v err=%v", caps, err)
	}
	err = exec(link + "child.title='B' AND parent.title='A' AND s.name='General'")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("reverse edge: got %v, want check violation", err)
	}
}
