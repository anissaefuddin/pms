package spaces

import (
	"context"
	"errors"

	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ItemShares reads the direct list, or replaces it when version is provided.
func (s *Service) ItemShares(ctx context.Context, actor policy.Actor, id string, version *int, want []ShareInput, batch *notifications.Batch) ([]Share, int, error) {
	if !IsID(id) {
		return nil, 0, ErrNotFound
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)
	caps, err := policy.ItemCaps(ctx, tx, actor, id)
	if err != nil {
		return nil, 0, err
	}
	if !caps.Has(policy.View) {
		return nil, 0, ErrNotFound
	}
	var current int
	if err := tx.QueryRow(ctx, "SELECT version FROM items WHERE id=$1 AND deleted_at IS NULL", id).Scan(&current); err != nil {
		return nil, 0, err
	}
	if version != nil {
		var personal bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(
SELECT 1 FROM item_locations l JOIN spaces s ON s.id=l.space_id
WHERE l.item_id=$1 AND l.deleted_root_id IS NULL AND s.kind='personal')`, id).Scan(&personal)
		if err != nil {
			return nil, 0, err
		}
		if personal || !caps.Has(policy.Share) {
			return nil, 0, ErrForbidden
		}
		if current != *version {
			return nil, 0, ErrVersionConflict
		}
		if err = tx.QueryRow(ctx, "UPDATE items SET version=version+1 WHERE id=$1 AND version=$2 RETURNING version", id, current).Scan(&current); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, 0, ErrVersionConflict
			}
			return nil, 0, itemShareConflict(err)
		}
		if err = writeTargetShares(ctx, tx, id, "item", actor, want, caps, batch); err != nil {
			return nil, 0, itemShareConflict(err)
		}
	}
	rows, err := readTargetShares(ctx, tx, actor, id, "item")
	if err != nil {
		return nil, 0, err
	}
	return rows, current, itemShareConflict(tx.Commit(ctx))
}

func itemShareConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01") {
		return ErrVersionConflict
	}
	return err
}
