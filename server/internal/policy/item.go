package policy

import (
	"context"

	"github.com/google/uuid"
)

// requesterRole marks the access a person has to the one Item their form submission created. It is not a share role:
// it cannot be granted, and it never reaches the Space, the project around the Item, or any sibling.
const requesterRole = "requester"

var requesterCaps = setOf(View, Comment, Attach)

// ItemCaps resolves live locations and ancestors in the same Space. A foreign
// Personal space cannot contribute roles, even when the actor is an admin.
func ItemCaps(ctx context.Context, db DB, actor Actor, itemID string) (Caps, error) {
	return itemCaps(ctx, db, actor, itemID, nil)
}

// ItemCapsInSpace checks an operation tied to a specific placement.
func ItemCapsInSpace(ctx context.Context, db DB, actor Actor, itemID, spaceID string) (Caps, error) {
	return itemCaps(ctx, db, actor, itemID, &spaceID)
}

// DeletedItemCapsInSpace authorizes a notification for an Item's own removed placement.
func DeletedItemCapsInSpace(ctx context.Context, db DB, actor Actor, itemID, spaceID string) (Caps, error) {
	id, err := uuid.Parse(itemID)
	if err != nil {
		return 0, err
	}
	all, err := itemsCaps(ctx, db, actor, []string{id.String()}, &spaceID, true, false)
	return all[id.String()], err
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
	return itemsCaps(ctx, db, actor, itemIDs, spaceID, false, false)
}

// RecycleItemsCaps resolves the permissions of an Item's original placements.
func RecycleItemsCaps(ctx context.Context, db DB, actor Actor, itemIDs []string, includeDeletedSpaces bool) (map[string]Caps, error) {
	return itemsCaps(ctx, db, actor, itemIDs, nil, true, includeDeletedSpaces)
}

func itemsCaps(ctx context.Context, db DB, actor Actor, itemIDs []string, spaceID *string, deleted, includeDeletedSpaces bool) (map[string]Caps, error) {
	rows, err := db.Query(ctx, `
WITH RECURSIVE ancestry(root_id, space_id, item_id, parent_id) AS (
  SELECT l.item_id, l.space_id, l.item_id, l.parent_item_id
  FROM item_locations l
  JOIN items i ON i.id = l.item_id
  JOIN spaces s ON s.id = l.space_id AND (s.deleted_at IS NULL OR $6::boolean)
  WHERE i.id = ANY(ARRAY(SELECT unnest($2::text[])::uuid))
    AND (s.kind = 'shared' OR s.owner_id = $1::uuid)
    AND (CASE WHEN $5::boolean THEN l.deleted_root_id=i.id AND i.deleted_at IS NOT NULL
         ELSE l.deleted_root_id IS NULL AND i.deleted_at IS NULL END)
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
UNION
SELECT fs.item_id::text, 'requester' FROM form_submissions fs JOIN items i ON i.id = fs.item_id AND i.deleted_at IS NULL
WHERE fs.requester_id = $1::uuid AND fs.item_id = ANY(ARRAY(SELECT unnest($2::text[])::uuid))
  AND NOT $5::boolean AND $4::uuid IS NULL
`, actor.ID, itemIDs, actor.Type == Admin, spaceID, deleted, includeDeletedSpaces)
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
		if role == requesterRole {
			caps[id] |= requesterCaps & Ceiling(actor.Type)
			continue
		}
		caps[id] |= Effective(actor.Type, Role(role))
	}
	return caps, rows.Err()
}
