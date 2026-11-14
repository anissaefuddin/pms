package mail

import (
	"context"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Spike for ADR-AUT (S7.3.3): a domain event row and its River job commit or roll back together, and a replayed event
// id publishes nothing twice. The table is created here on purpose; it becomes a real migration only with S9.1.1.
func TestSpikeTransactionalOutboxWithRiver(t *testing.T) {
	ctx := t.Context()
	pool, err := database.Open(ctx, testdb.NewMigrated(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `CREATE TABLE spike_domain_events (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, event_id uuid NOT NULL UNIQUE, type text NOT NULL,
  payload jsonb NOT NULL, depth int NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	queue, err := New(pool, config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	publish := func(tx pgx.Tx, eventID uuid.UUID) (bool, error) {
		tag, err := tx.Exec(ctx, `INSERT INTO spike_domain_events(event_id,type,payload) VALUES ($1,'item.status_changed','{}') ON CONFLICT (event_id) DO NOTHING`, eventID)
		if err != nil || tag.RowsAffected() == 0 {
			return false, err // replayed: the earlier publish already queued its job
		}
		return true, queue.NotifyTx(ctx, tx, uuid.NewString())
	}
	count := func(sql string) int {
		var n int
		if err := pool.QueryRow(ctx, sql).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	jobs := func() int { return count("SELECT count(*) FROM river_job WHERE kind='send_notification_email'") }
	events := func() int { return count("SELECT count(*) FROM spike_domain_events") }

	rolledBack := uuid.New()
	tx, _ := pool.Begin(ctx)
	if _, err := publish(tx, rolledBack); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil || events() != 0 || jobs() != 0 {
		t.Fatalf("rollback left %d events and %d jobs", events(), jobs())
	}
	committed := uuid.New()
	for i, want := range []bool{true, false, false} {
		tx, _ := pool.Begin(context.WithoutCancel(ctx))
		got, err := publish(tx, committed)
		if err != nil || got != want {
			t.Fatalf("publish #%d: %v %v, want %v", i, got, err, want)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if events() != 1 || jobs() != 1 {
		t.Fatalf("replayed event id produced %d events and %d jobs, want 1 and 1", events(), jobs())
	}
}
