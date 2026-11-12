package items

import (
	"context"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Delete(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, id string) error {
	itemID, err := uuid.Parse(id)
	if err != nil {
		return ErrNotFound
	}
	id = itemID.String()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	caps, err := policy.ItemCaps(ctx, tx, actor, id)
	if err != nil {
		return err
	}
	if !caps.Has(policy.View) {
		return ErrNotFound
	}
	if !caps.Has(policy.Delete) {
		return ErrForbidden
	}
	// Work per (Space, Item) placement. Descendants keep any path to a
	// live root that does not pass through the deleted Item.
	_, err = tx.Exec(ctx, `
WITH RECURSIVE live AS MATERIALIZED (
  SELECT l.item_id,l.space_id,l.parent_item_id
  FROM item_locations l JOIN items i ON i.id=l.item_id JOIN spaces s ON s.id=l.space_id
  WHERE l.deleted_root_id IS NULL AND i.deleted_at IS NULL AND s.deleted_at IS NULL
), affected(item_id,space_id) AS (
  SELECT item_id,space_id FROM live WHERE item_id=$1::uuid
  UNION
  SELECT l.item_id,l.space_id FROM live l JOIN affected a
    ON l.space_id=a.space_id AND l.parent_item_id=a.item_id
), reachable(item_id,space_id) AS (
  SELECT item_id,space_id FROM live WHERE parent_item_id IS NULL AND item_id<>$1::uuid
  UNION
  SELECT l.item_id,l.space_id FROM live l JOIN reachable r
    ON l.space_id=r.space_id AND l.parent_item_id=r.item_id
  WHERE l.item_id<>$1::uuid
), marked AS (
  UPDATE item_locations l SET deleted_root_id=$1::uuid
  WHERE l.deleted_root_id IS NULL AND (
    l.item_id=$1::uuid OR (
      EXISTS (SELECT 1 FROM affected a WHERE a.item_id=l.item_id AND a.space_id=l.space_id)
      AND l.parent_item_id IS NOT NULL
      AND NOT EXISTS (SELECT 1 FROM reachable r WHERE r.item_id=l.parent_item_id AND r.space_id=l.space_id)
    )
  )
  RETURNING l.item_id,l.space_id,l.parent_item_id
), removed AS (
  UPDATE items i SET deleted_root_id=$1::uuid, deleted_by=$2::uuid, deleted_at=now(), version=version+1
  WHERE i.deleted_at IS NULL AND (
    i.id=$1::uuid OR (EXISTS (SELECT 1 FROM affected a WHERE a.item_id=i.id)
                      AND NOT EXISTS (SELECT 1 FROM reachable r WHERE r.item_id=i.id))
  )
  RETURNING i.id,i.deleted_at
), detached AS (
  UPDATE items i SET version=version+1
  WHERE i.deleted_at IS NULL AND i.id<>$1::uuid
    AND EXISTS (SELECT 1 FROM marked m WHERE m.item_id=i.id)
    AND EXISTS (SELECT 1 FROM reachable r WHERE r.item_id=i.id)
  RETURNING i.id
)
INSERT INTO activity(item_id,actor_id,action,changes)
SELECT id,$2::uuid,'delete',jsonb_build_object('deleted_at',jsonb_build_object('from',NULL,'to',deleted_at))
FROM removed
UNION ALL
SELECT d.id,$2::uuid,'detach',jsonb_build_object('locations',jsonb_build_object('from',
  jsonb_agg(jsonb_build_object('space_id',m.space_id,'parent_item_id',m.parent_item_id)), 'to','[]'::jsonb))
FROM detached d JOIN marked m ON m.item_id=d.id GROUP BY d.id`, id, actor.ID)
	if err != nil {
		return writeError(err)
	}
	return writeError(tx.Commit(ctx))
}
