package clientscope

import (
	"context"
	"errors"
	"fmt"

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

// Apply classifies the reviewed changes in one serializable transaction. It refuses when the plan no longer matches the dry-run
// digest, when any conflict remains, or when a row is no longer 'unresolved'. Nothing else is touched. Returns the reconciliation.
func Apply(ctx context.Context, pool *pgxpool.Pool, m Mapping, expectedDigest string) (Report, error) {
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
	}
	if err := tx.Commit(ctx); err != nil {
		return rep, err
	}
	// Reconciliation is read-only and runs after the commit, so it reports what is really stored now.
	return DryRun(ctx, pool, m)
}
