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

func locationCaps(ctx context.Context, tx policy.DB, actor policy.Actor, loc Location) (policy.Caps, error) {
	if loc.SpaceID == uuid.Nil || (loc.ParentID != nil && *loc.ParentID == uuid.Nil) {
		return 0, Invalid("location IDs must be UUIDs")
	}
	if loc.ParentID != nil {
		return policy.ItemCapsInSpace(ctx, tx, actor, loc.ParentID.String(), loc.SpaceID.String())
	}
	access, _, err := policy.SpaceAccessFor(ctx, tx, actor, loc.SpaceID.String())
	return access.Caps, err
}

// rankAfter is the rank for a new last sibling. It steps the final character up, so the rank grows by one character only
// every ~35 appends (not on every append, which capped a parent at ~512 children).
func rankAfter(last string) string {
	const top = byte('y') // stay below the 'z' upper bound used by rankBetween
	if last == "" {
		return rankBetween("", "")
	}
	if c := last[len(last)-1]; c < top {
		return last[:len(last)-1] + string(c+1)
	}
	return last + string(byte('V'))
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
	var detail Detail
	err = inSerializable(ctx, pool, locationLockKeys(want), func(tx pgx.Tx) (err error) {
		detail, err = replaceLocations(ctx, tx, actor, id, itemID, want)
		return err
	}, func() {})
	return detail, writeError(err)
}

func replaceLocations(ctx context.Context, tx pgx.Tx, actor policy.Actor, id string, itemID uuid.UUID, want []Location) (Detail, error) {
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
	var before []byte // S-5: history keeps where the item was; written in the same transaction as the move
	if err := tx.QueryRow(ctx, locationsSnapshot, itemID).Scan(&before); err != nil {
		return Detail{}, err
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
	// An item in a locked Space stays in it: only an administrator can place it in another Space (D-95).
	// D-17.1: leaving a locked Space needs an explicit Space Admin of every locked Space the item is in, not the Workspace admin role. Read before this request removes any location, so the rule sees the item's real placement.
	lockedHere, err := policy.LockedSpacesOf(ctx, tx, id)
	if err != nil {
		return Detail{}, err
	}
	restricted := false
	for sid := range lockedHere {
		ok, err := policy.IsSpaceAdmin(ctx, tx, actor.ID, sid)
		if err != nil {
			return Detail{}, err
		}
		if !ok {
			restricted = true
		}
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
		if restricted && !lockedHere[loc.SpaceID.String()] {
			return Detail{}, ErrForbidden
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
		rank := rankAfter(last)
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
	var after []byte
	if err := tx.QueryRow(ctx, locationsSnapshot, itemID).Scan(&after); err != nil {
		return Detail{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO activity(item_id,actor_id,action,changes)
SELECT $1, $2::uuid, 'update', jsonb_build_object('locations', jsonb_build_object('from', $3::jsonb, 'to', $4::jsonb))
WHERE $3::jsonb IS DISTINCT FROM $4::jsonb`, itemID, actor.ID, before, after); err != nil {
		return Detail{}, err
	}
	return readDetail(ctx, tx, actor, id)
}

// locationsSnapshot lists an item's live locations as JSON, the form the history stores for a move.
const locationsSnapshot = `SELECT COALESCE(jsonb_agg(jsonb_build_object('space_id', space_id, 'parent_item_id', parent_item_id)
ORDER BY space_id, parent_item_id NULLS FIRST), '[]'::jsonb) FROM item_locations WHERE item_id=$1 AND deleted_root_id IS NULL`

func countExisting(want []Location, existing map[[2]uuid.UUID]bool, key func(Location) [2]uuid.UUID) int {
	n := 0
	for _, loc := range want {
		if existing[key(loc)] {
			n++
		}
	}
	return n
}
