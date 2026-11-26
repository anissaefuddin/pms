package items

import (
	"context"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// boundary names who an Item placed here is exposed to: the Space, plus every folder, project or task above it that has shares of
// its own. Two places with the same boundary give the same people the same say. Removed rows count, for restoring from the bin.
func boundary(ctx context.Context, db policy.DB, loc Location) (string, error) {
	rows, err := db.Query(ctx, `WITH RECURSIVE up(id) AS (
  SELECT $2::uuid WHERE $2::uuid IS NOT NULL
  UNION
  SELECT l.parent_item_id FROM up JOIN item_locations l ON l.item_id=up.id AND l.space_id=$1::uuid WHERE l.parent_item_id IS NOT NULL)
SELECT $1::text || ':' || COALESCE(string_agg(id::text, ',' ORDER BY id), '') FROM up
WHERE EXISTS (SELECT 1 FROM shares s WHERE s.target_type='item' AND s.target_id=up.id)`, loc.SpaceID, loc.ParentID)
	if err != nil {
		return "", err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowTo[string])
}

// carriesOthers says whether the Item, or something below it, brings along what could be used to hand out work: someone else's
// assignment, or a direct share that includes sharing (it would go on granting wherever the Item goes).
func carriesOthers(ctx context.Context, tx pgx.Tx, itemID uuid.UUID, actorID string) (found bool, err error) {
	err = tx.QueryRow(ctx, `WITH RECURSIVE sub(id) AS (SELECT $1::uuid
UNION SELECT l.item_id FROM item_locations l JOIN sub ON l.parent_item_id=sub.id)
SELECT EXISTS(SELECT 1 FROM item_assignees a JOIN sub ON a.item_id=sub.id WHERE a.user_id<>$2::uuid)
 OR EXISTS(SELECT 1 FROM shares sh JOIN sub ON sh.target_type='item' AND sh.target_id=sub.id LEFT JOIN custom_roles cr ON cr.id=sh.custom_role_id
   WHERE (sh.custom_role_id IS NULL AND sh.role='full') OR cr.caps_mask & 128 <> 0)`, itemID, actorID).Scan(&found)
	return found, err
}

// mayCarryInto is DEC-10: other people's assignments enter a place with a new boundary only with someone who may assign there.
func mayCarryInto(ctx context.Context, tx pgx.Tx, actor policy.Actor, to Location, from map[string]bool) error {
	b, err := boundary(ctx, tx, to)
	if err != nil || from[b] {
		return err
	}
	ok, err := CanAssignOthers(ctx, tx, actor, []Location{to})
	if err == nil && !ok {
		err = ErrAssignForbidden
	}
	return err
}
