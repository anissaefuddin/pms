package policy

import (
	"context"
	"errors"
)

// ErrLockedSpace means a locked Space refuses the change; the text is shown to the person.
var ErrLockedSpace = errors.New("this is in a locked space: only people who already have access to the space may be added, and only an administrator can change the space's own sharing or move its items out")

// LockedAllows reports whether the principal may be given access to the item. An item inside a locked Space can only be shared with
// someone who already reaches that Space (a share, a group in its share list, or an explicit Space Admin), so locking a Space
// keeps its content from leaking through item sharing. Every locked Space the item sits in must allow it.
func LockedAllows(ctx context.Context, db DB, itemID, principalType, principalID string) (bool, error) {
	rows, err := db.Query(ctx, `
SELECT EXISTS (
  SELECT 1 FROM item_locations l JOIN spaces s ON s.id=l.space_id AND s.locked AND s.deleted_at IS NULL
  WHERE l.item_id=$1::uuid AND l.deleted_root_id IS NULL AND NOT (
    ($2='user' AND (
       EXISTS (SELECT 1 FROM space_admins sa WHERE sa.space_id=s.id AND sa.user_id=$3::uuid)
       OR EXISTS (SELECT 1 FROM shares sh WHERE sh.target_type='space' AND sh.target_id=s.id AND (
            (sh.principal_type='user' AND sh.principal_id=$3::uuid)
         OR (sh.principal_type='group' AND sh.principal_id IN (SELECT group_id FROM group_members WHERE user_id=$3::uuid))))))
    OR ($2='group' AND EXISTS (SELECT 1 FROM shares sh WHERE sh.target_type='space' AND sh.target_id=s.id AND sh.principal_type='group' AND sh.principal_id=$3::uuid))))`,
		itemID, principalType, principalID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	var blocked bool
	if rows.Next() {
		if err := rows.Scan(&blocked); err != nil {
			return false, err
		}
	}
	return !blocked, rows.Err()
}

// LockedSpacesOf lists the locked Spaces an item currently sits in.
func LockedSpacesOf(ctx context.Context, db DB, itemID string) (map[string]bool, error) {
	rows, err := db.Query(ctx, `SELECT s.id::text FROM item_locations l JOIN spaces s ON s.id=l.space_id AND s.locked AND s.deleted_at IS NULL
WHERE l.item_id=$1::uuid AND l.deleted_root_id IS NULL`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// IsSpaceAdmin reports whether the actor is an explicit Space Admin of the Space (D-05). A Workspace admin is not one unless named.
func IsSpaceAdmin(ctx context.Context, db DB, actorID, spaceID string) (bool, error) {
	rows, err := db.Query(ctx, "SELECT 1 FROM space_admins WHERE space_id=$1::uuid AND user_id=$2::uuid", spaceID, actorID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	return rows.Next(), rows.Err()
}
