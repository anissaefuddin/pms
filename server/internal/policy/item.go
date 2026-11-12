package policy

import (
	"context"

	"github.com/google/uuid"
)

// ItemCaps resolves live locations and ancestors in the same Space. A foreign
// Personal space cannot contribute roles, even when the actor is an admin.
func ItemCaps(ctx context.Context, db DB, actor Actor, itemID string) (Caps, error) {
	return itemCaps(ctx, db, actor, itemID, nil)
}

// ItemCapsInSpace checks an operation tied to a specific placement.
func ItemCapsInSpace(ctx context.Context, db DB, actor Actor, itemID, spaceID string) (Caps, error) {
	return itemCaps(ctx, db, actor, itemID, &spaceID)
}

func itemCaps(ctx context.Context, db DB, actor Actor, itemID string, spaceID *string) (Caps, error) {
	id, err := uuid.Parse(itemID)
	if err != nil {
		return 0, err
	}
	itemID = id.String()
	all, err := ItemsCaps(ctx, db, actor, []string{itemID}, spaceID)
	return all[itemID], err
}

// ItemsCaps resolves the same policy for a page of Items in a single query.
func ItemsCaps(ctx context.Context, db DB, actor Actor, itemIDs []string, spaceID *string) (map[string]Caps, error) {
	rows, err := db.Query(ctx, `
WITH RECURSIVE ancestry(root_id, space_id, item_id, parent_id) AS (
  SELECT l.item_id, l.space_id, l.item_id, l.parent_item_id
  FROM item_locations l
  JOIN items i ON i.id = l.item_id AND i.deleted_at IS NULL
  JOIN spaces s ON s.id = l.space_id AND s.deleted_at IS NULL
  WHERE i.id = ANY(ARRAY(SELECT unnest($2::text[])::uuid))
    AND (s.kind = 'shared' OR s.owner_id = $1::uuid)
    AND l.deleted_root_id IS NULL
    AND ($4::uuid IS NULL OR l.space_id = $4::uuid)
  UNION
  SELECT a.root_id, l.space_id, l.item_id, l.parent_item_id
  FROM ancestry a
  JOIN item_locations l ON l.item_id = a.parent_id AND l.space_id = a.space_id
  JOIN items i ON i.id = l.item_id AND i.deleted_at IS NULL
  WHERE l.deleted_root_id IS NULL
), targets(root_id, target_type, target_id) AS (
  SELECT root_id, 'space'::share_target, space_id FROM ancestry
  UNION
  SELECT root_id, 'item'::share_target, item_id FROM ancestry
)
SELECT a.root_id::text, 'full' FROM ancestry a JOIN spaces s ON s.id=a.space_id
WHERE s.kind = 'personal' OR ($3::boolean AND s.kind = 'shared')
UNION
SELECT t.root_id::text, sh.role::text FROM shares sh
JOIN targets t ON t.target_type = sh.target_type AND t.target_id = sh.target_id
WHERE (sh.principal_type = 'user' AND sh.principal_id = $1::uuid)
   OR (sh.principal_type = 'group' AND sh.principal_id IN
       (SELECT group_id FROM group_members WHERE user_id = $1::uuid))
`, actor.ID, itemIDs, actor.Type == Admin, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	caps := map[string]Caps{}
	for rows.Next() {
		var id, role string
		if err := rows.Scan(&id, &role); err != nil {
			return nil, err
		}
		caps[id] |= Effective(actor.Type, Role(role))
	}
	return caps, rows.Err()
}
