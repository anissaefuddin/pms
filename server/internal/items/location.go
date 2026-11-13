package items

import (
	"context"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Location struct {
	SpaceID  uuid.UUID  `json:"space_id"`
	ParentID *uuid.UUID `json:"parent_item_id"`
}

func locationCaps(ctx context.Context, tx pgx.Tx, actor policy.Actor, loc Location) (policy.Caps, error) {
	if loc.SpaceID == uuid.Nil || (loc.ParentID != nil && *loc.ParentID == uuid.Nil) {
		return 0, Invalid("location IDs must be UUIDs")
	}
	if loc.ParentID != nil {
		return policy.ItemCapsInSpace(ctx, tx, actor, loc.ParentID.String(), loc.SpaceID.String())
	}
	access, _, err := policy.SpaceAccessFor(ctx, tx, actor, loc.SpaceID.String())
	return access.Caps, err
}

// rankBetween uses bytewise order. Generated ranks never end at the smallest
// digit, so another rank can always be inserted before an existing one.
func rankBetween(left, right string) string {
	const low, high = byte('0'), byte('z')
	i := 0
	for ; i < len(left) && i < len(right) && left[i] == right[i]; i++ {
	}
	prefix := left[:i]
	lo, hi := low, high
	if i < len(left) {
		lo = left[i]
	}
	if i < len(right) {
		hi = right[i]
	}
	if lo+1 < hi {
		return prefix + string(lo+(hi-lo)/2)
	}
	if i < len(left) {
		return left + "U"
	}
	return prefix + string(low) + rankBetween("", right[i+1:])
}

// ReplaceLocations sets the locations the actor can see. Locations hidden from the actor stay, so the Item always keeps at least one.
// New locations need Create there, removed ones need Edit, and the Item keeps a single workflow.
func ReplaceLocations(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, id string, want []Location) (Detail, error) {
	itemID, err := uuid.Parse(id)
	if err != nil {
		return Detail{}, ErrNotFound
	}
	if len(want) == 0 || len(want) > 50 {
		return Detail{}, Invalid("an item needs between 1 and 50 locations")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback(ctx)
	caps, err := policy.ItemCaps(ctx, tx, actor, id)
	if err != nil {
		return Detail{}, err
	}
	if !caps.Has(policy.View) {
		return Detail{}, ErrNotFound
	}
	if !caps.Has(policy.Edit) {
		return Detail{}, ErrForbidden
	}
	var kind string
	var workflow uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT i.type::text, COALESCE(s.workflow_id,(SELECT id FROM workflows WHERE is_default))
FROM items i LEFT JOIN statuses s ON s.id=i.status_id WHERE i.id=$1 AND i.deleted_at IS NULL`, itemID).Scan(&kind, &workflow); err != nil {
		return Detail{}, ErrNotFound
	}
	rows, err := tx.Query(ctx, "SELECT space_id, parent_item_id FROM item_locations WHERE item_id=$1 AND deleted_root_id IS NULL", itemID)
	if err != nil {
		return Detail{}, err
	}
	current, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Location])
	if err != nil {
		return Detail{}, err
	}
	key := func(loc Location) [2]uuid.UUID {
		parent := uuid.Nil
		if loc.ParentID != nil {
			parent = *loc.ParentID
		}
		return [2]uuid.UUID{loc.SpaceID, parent}
	}
	wanted := map[[2]uuid.UUID]bool{}
	for _, loc := range want {
		if wanted[key(loc)] {
			return Detail{}, Invalid("duplicate location")
		}
		wanted[key(loc)] = true
	}
	existing := map[[2]uuid.UUID]bool{}
	var remove, kept int
	for _, loc := range current {
		existing[key(loc)] = true
		placement, err := locationCaps(ctx, tx, actor, loc)
		if err != nil {
			return Detail{}, err
		}
		switch {
		case !placement.Has(policy.View) || wanted[key(loc)]:
			kept++ // hidden from the actor, or kept
		default:
			if !placement.Has(policy.Edit) {
				return Detail{}, ErrForbidden
			}
			if _, err = tx.Exec(ctx, "DELETE FROM item_locations WHERE item_id=$1 AND space_id=$2 AND parent_item_id IS NOT DISTINCT FROM $3::uuid AND deleted_root_id IS NULL",
				itemID, loc.SpaceID, loc.ParentID); err != nil {
				return Detail{}, err
			}
			remove++
		}
	}
	for _, loc := range want {
		if existing[key(loc)] {
			continue
		}
		placement, err := locationCaps(ctx, tx, actor, loc)
		if err != nil {
			return Detail{}, err
		}
		if !placement.Has(policy.View) {
			return Detail{}, ErrNotFound
		}
		if !placement.Has(policy.Create) {
			return Detail{}, ErrForbidden
		}
		resolved := uuid.Nil
		if loc.ParentID != nil {
			var parentKind string
			if err = tx.QueryRow(ctx, `SELECT i.type::text, COALESCE(i.default_workflow_id,s.workflow_id,(SELECT id FROM workflows WHERE is_default))
FROM items i LEFT JOIN statuses s ON s.id=i.status_id WHERE i.id=$1 AND i.deleted_at IS NULL`, *loc.ParentID).Scan(&parentKind, &resolved); err != nil {
				return Detail{}, Invalid("unknown parent item")
			}
			if parentKind == "task" && kind != "task" {
				return Detail{}, Invalid("a task can contain only tasks")
			}
			if err := checkAllowedUnder(ctx, tx, *loc.ParentID, itemID); err != nil { // S8.1.3
				return Detail{}, err
			}
		} else if err = tx.QueryRow(ctx, "SELECT id FROM workflows WHERE is_default").Scan(&resolved); err != nil {
			return Detail{}, err
		}
		if resolved != workflow {
			return Detail{}, Invalid("locations must use the same workflow")
		}
		var last string
		if err = tx.QueryRow(ctx, `SELECT COALESCE(max(rank COLLATE "C"),'') FROM item_locations
WHERE space_id=$1 AND parent_item_id IS NOT DISTINCT FROM $2::uuid AND deleted_root_id IS NULL`, loc.SpaceID, loc.ParentID).Scan(&last); err != nil {
			return Detail{}, err
		}
		rank := rankBetween(last, "")
		if len(rank) > 512 || rank <= last {
			return Detail{}, ErrConflict
		}
		if _, err = tx.Exec(ctx, "INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES($1,$2,$3,$4)", itemID, loc.SpaceID, loc.ParentID, rank); err != nil {
			if pgCode(err) == "23514" {
				return Detail{}, Invalid("this location would put the item inside itself")
			}
			return Detail{}, writeError(err)
		}
	}
	if kept+len(want)-countExisting(want, existing, key) < 1 {
		return Detail{}, Invalid("an item needs at least one location")
	}
	if _, err = tx.Exec(ctx, "UPDATE items SET updated_at=now() WHERE id=$1", itemID); err != nil {
		return Detail{}, err
	}
	detail, err := readDetail(ctx, tx, actor, id)
	if err == nil {
		err = tx.Commit(ctx)
	}
	return detail, writeError(err)
}

func countExisting(want []Location, existing map[[2]uuid.UUID]bool, key func(Location) [2]uuid.UUID) int {
	n := 0
	for _, loc := range want {
		if existing[key(loc)] {
			n++
		}
	}
	return n
}
