package items

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReorderInput struct {
	SpaceID  *uuid.UUID `json:"space_id"`
	ParentID *uuid.UUID `json:"parent_item_id"`
	AfterID  *uuid.UUID `json:"after_item_id"`
}

func Reorder(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, id string, input ReorderInput) error {
	itemID, err := uuid.Parse(id)
	if err != nil {
		return ErrNotFound
	}
	if input.AfterID != nil && *input.AfterID == itemID {
		return Invalid("after_item_id must name a sibling")
	}
	var keys []string
	if input.SpaceID != nil || input.ParentID != nil { // a root reorder without a Space is ambiguous; the retry covers it
		space := uuid.Nil
		if input.SpaceID != nil {
			space = *input.SpaceID
		}
		keys = locationLockKeys([]Location{{SpaceID: space, ParentID: input.ParentID}})
	}
	return writeError(inSerializable(ctx, pool, keys, func(tx pgx.Tx) error {
		return reorder(ctx, tx, actor, itemID, input)
	}, func() {}))
}

func reorder(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID, input ReorderInput) error {
	caps, err := policy.ItemCaps(ctx, tx, actor, id.String())
	if err != nil {
		return err
	}
	if !caps.Has(policy.View) {
		return ErrNotFound
	}
	rows, err := tx.Query(ctx, `
SELECT space_id FROM item_locations WHERE item_id=$1
AND parent_item_id IS NOT DISTINCT FROM $2::uuid AND ($3::uuid IS NULL OR space_id=$3)
AND deleted_root_id IS NULL`, id, input.ParentID, input.SpaceID)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	var space uuid.UUID
	for _, candidate := range ids {
		placement, err := locationCaps(ctx, tx, actor, Location{SpaceID: candidate, ParentID: input.ParentID})
		if err != nil {
			return err
		}
		if !placement.Has(policy.View) {
			continue
		}
		if space != uuid.Nil {
			return Invalid("space_id is required for an ambiguous location")
		}
		space = candidate
		if !placement.Has(policy.Edit) {
			return ErrForbidden
		}
	}
	if space == uuid.Nil {
		return ErrNotFound
	}
	caps, err = policy.ItemCapsInSpace(ctx, tx, actor, id.String(), space.String())
	if err != nil {
		return err
	}
	if !caps.Has(policy.Edit) {
		return ErrForbidden
	}
	var old, left, right string
	err = tx.QueryRow(ctx, `
SELECT rank FROM item_locations WHERE item_id=$1 AND space_id=$2
AND parent_item_id IS NOT DISTINCT FROM $3::uuid AND deleted_root_id IS NULL FOR UPDATE`,
		id, space, input.ParentID).Scan(&old)
	if err != nil {
		return err
	}
	if input.AfterID != nil {
		sibling, err := policy.ItemCapsInSpace(ctx, tx, actor, input.AfterID.String(), space.String())
		if err != nil {
			return err
		}
		if !sibling.Has(policy.View) {
			return ErrNotFound
		}
		err = tx.QueryRow(ctx, `
SELECT l.rank FROM item_locations l JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
WHERE l.item_id=$1 AND l.space_id=$2 AND l.parent_item_id IS NOT DISTINCT FROM $3::uuid
AND l.deleted_root_id IS NULL`, *input.AfterID, space, input.ParentID).Scan(&left)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
	}
	err = tx.QueryRow(ctx, `
SELECT COALESCE((
 SELECT l.rank FROM item_locations l JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
 WHERE l.item_id<>$1 AND l.space_id=$2 AND l.parent_item_id IS NOT DISTINCT FROM $3::uuid
 AND l.deleted_root_id IS NULL
 AND ($4::uuid IS NULL OR (l.rank COLLATE "C",l.item_id)>($5::text COLLATE "C",$4::uuid))
 ORDER BY l.rank COLLATE "C",l.item_id LIMIT 1
),'')`, id, space, input.ParentID, input.AfterID, left).Scan(&right)
	if err != nil {
		return err
	}
	if right != "" && left == right {
		return ErrConflict
	}
	rank := rankBetween(left, right)
	if len(rank) > 512 || rank <= left || (right != "" && rank >= right) {
		return ErrConflict
	}
	if _, err := tx.Exec(ctx, `
UPDATE item_locations SET rank=$4 WHERE item_id=$1 AND space_id=$2
AND parent_item_id IS NOT DISTINCT FROM $3::uuid AND deleted_root_id IS NULL`,
		id, space, input.ParentID, rank); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "UPDATE items SET version=version+1 WHERE id=$1", id); err != nil {
		return err
	}
	before, _ := json.Marshal(map[string]string{"rank": old})
	after, _ := json.Marshal(map[string]string{"rank": rank})
	return recordChange(ctx, tx, id.String(), actor.ID, "reorder", before, after)
}
