package items

import (
	"context"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Purge permanently deletes every Recycle bin entry the actor may delete (BIN-03) and returns how many entries went.
// Child rows (comments, attachments, ...) cascade; stored files are removed later by housekeeping.SweepFiles.
func Purge(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor) (int, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id::text FROM items WHERE deleted_at IS NOT NULL AND deleted_root_id=id ORDER BY id`)
	if err != nil {
		return 0, err
	}
	roots, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return 0, err
	}
	allowed := []string{}
	for start := 0; start < len(roots); start += 500 {
		chunk := roots[start:min(start+500, len(roots))]
		caps, err := policy.RecycleItemsCaps(ctx, tx, actor, chunk, true)
		if err != nil {
			return 0, err
		}
		for _, id := range chunk {
			if caps[id].Has(policy.View) && caps[id].Has(policy.Delete) {
				allowed = append(allowed, id)
			}
		}
	}
	if len(allowed) == 0 {
		return 0, nil
	}
	for _, q := range []string{
		`UPDATE item_locations SET parent_item_id=NULL WHERE parent_item_id IN (SELECT id FROM items WHERE deleted_root_id=ANY(ARRAY(SELECT unnest($1::text[])::uuid)))`,
		`DELETE FROM item_locations WHERE deleted_root_id=ANY(ARRAY(SELECT unnest($1::text[])::uuid))`,
		`DELETE FROM items WHERE deleted_root_id=ANY(ARRAY(SELECT unnest($1::text[])::uuid))`,
	} {
		if _, err := tx.Exec(ctx, q, allowed); err != nil {
			return 0, writeError(err)
		}
	}
	return len(allowed), writeError(tx.Commit(ctx))
}
