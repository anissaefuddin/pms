package clientscope

import (
	"context"
	"errors"
	"fmt"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DryRun plans the mapping in a read-only transaction. It writes nothing.
func DryRun(ctx context.Context, pool *pgxpool.Pool, m Mapping) (Report, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Report{}, err
	}
	defer tx.Rollback(ctx)
	return Plan(ctx, tx, m)
}

// Apply classifies the reviewed changes in one serializable transaction. Each real ownership change writes its audit row in that same
// transaction, so an audit failure rolls the change back. It refuses when the plan no longer matches the dry-run digest, or when any
// conflict remains; a refusal is recorded in its own committed transaction. Rows not 'unresolved' are never touched.
func Apply(ctx context.Context, pool *pgxpool.Pool, m Mapping, expectedDigest string) (Report, error) {
	run := uuid.NewString()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Report{}, err
	}
	defer tx.Rollback(ctx)
	rep, err := Plan(ctx, tx, m)
	if err != nil {
		return rep, err
	}
	if rep.Digest != expectedDigest {
		return rep, errors.New("the plan no longer matches the reviewed dry-run digest; run dry-run again")
	}
	if len(rep.Conflicts) > 0 {
		_ = tx.Rollback(ctx)
		if err := recordRefusal(ctx, pool, run, rep); err != nil {
			return rep, err
		}
		return rep, errors.New("conflicts remain; nothing was applied")
	}
	for _, c := range rep.changes {
		table := "spaces"
		if c.Kind == "item" {
			table = "items"
		}
		var client any
		if c.Client != "" {
			client = c.Client
		}
		tag, err := tx.Exec(ctx, fmt.Sprintf("UPDATE %s SET scope_state=$2, client_id=$3::uuid WHERE id=$1::uuid AND scope_state='unresolved'", table), c.ID, c.Scope, client)
		if err != nil {
			return rep, err
		}
		if tag.RowsAffected() != 1 {
			return rep, errors.New("a record changed during apply; nothing was committed")
		}
		if err := items.Audit(ctx, tx, policy.Actor{}, "client_scope.classify", c.Kind, c.ID, map[string]any{
			"run_id": run, "previous_scope": "unresolved", "scope": c.Scope, "client_id": c.Client,
			"mapping_digest": rep.MappingDigest, "plan_digest": rep.Digest, "operator": "pms client-scope",
		}); err != nil {
			return rep, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return rep, err
	}
	// Reconciliation is read-only and runs after the commit, so it reports what is really stored now.
	return DryRun(ctx, pool, m)
}

// recordRefusal writes the refusal as durable evidence. It is its own transaction, because the apply transaction was rolled back.
func recordRefusal(ctx context.Context, pool *pgxpool.Pool, run string, rep Report) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := items.Audit(ctx, tx, policy.Actor{}, "client_scope.refused", "migration", run, map[string]any{
		"run_id": run, "mapping_digest": rep.MappingDigest, "plan_digest": rep.Digest, "conflicts": rep.Conflicts,
	}); err != nil {
		return err
	}
	// Each record that blocked the plan gets its own durable reason, in the same committed refusal (quarantine evidence).
	for _, f := range rep.findings {
		if err := items.Audit(ctx, tx, policy.Actor{}, "client_scope.quarantine", f.Kind, f.ID, map[string]any{
			"run_id": run, "reason": f.Code, "mapping_digest": rep.MappingDigest, "status": "refused",
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
