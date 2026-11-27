package policy

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// itemGate is the B3-05 eligibility predicate on one location, evaluated before any permission is unioned. It expects the
// item as i, its Space as s, and the actor as $1. A client item needs an active membership of its own Customer, and then
// only locations in Spaces of that same Customer contribute. An internal item only contributes through internal Spaces.
// Unresolved or inconsistent ownership matches neither branch, so it contributes nothing (fail closed).
const itemGate = `((i.scope_state = 'internal' AND i.client_id IS NULL AND s.scope_state = 'internal' AND s.client_id IS NULL)
    OR (i.scope_state = 'client' AND i.client_id IS NOT NULL AND s.scope_state = 'client' AND s.client_id = i.client_id AND EXISTS (
          SELECT 1 FROM client_memberships m JOIN users u ON u.id = m.user_id
          WHERE m.client_id = i.client_id AND m.user_id = $1::uuid AND m.status = 'active'
            AND u.status = 'active' AND u.user_type IN ('admin', 'regular'))))`

// itemsCapsSQL is the one statement that unions every permission source on live locations. /*ITEM_GATE*/ is replaced by itemGate.
const itemsCapsSQL = `
WITH RECURSIVE ancestry(root_id, space_id, item_id, parent_id) AS (
  SELECT l.item_id, l.space_id, l.item_id, l.parent_item_id
  FROM item_locations l
  JOIN items i ON i.id = l.item_id
  JOIN spaces s ON s.id = l.space_id AND (s.deleted_at IS NULL OR $5::boolean)
  WHERE i.id = ANY(ARRAY(SELECT unnest($2::text[])::uuid))
    AND (s.kind = 'shared' OR s.owner_id = $1::uuid)
    AND /*ITEM_GATE*/
    AND (CASE WHEN $4::boolean THEN l.deleted_root_id=i.id AND i.deleted_at IS NOT NULL
         ELSE l.deleted_root_id IS NULL AND i.deleted_at IS NULL END)
    AND ($3::uuid IS NULL OR l.space_id = $3::uuid)
  UNION
  SELECT a.root_id, l.space_id, l.item_id, l.parent_item_id
  FROM ancestry a
  JOIN items ri ON ri.id = a.root_id
  JOIN item_locations l ON l.item_id = a.parent_id AND l.space_id = a.space_id
  JOIN items i ON i.id = l.item_id AND i.deleted_at IS NULL
    AND i.scope_state = ri.scope_state AND i.client_id IS NOT DISTINCT FROM ri.client_id
  WHERE l.deleted_root_id IS NULL
), targets(root_id, target_type, target_id) AS (
  SELECT root_id, 'space'::share_target, space_id FROM ancestry
  UNION
  SELECT root_id, 'item'::share_target, item_id FROM ancestry
)
SELECT a.root_id::text, 'full' FROM ancestry a JOIN spaces s ON s.id=a.space_id
WHERE s.kind = 'personal' OR EXISTS (SELECT 1 FROM space_admins sa JOIN users au ON au.id = sa.user_id WHERE sa.space_id = a.space_id AND sa.user_id = $1::uuid AND au.user_type IN ('admin','regular'))
UNION
SELECT t.root_id::text, CASE WHEN cr.id IS NOT NULL THEN 'mask:' || cr.caps_mask ELSE sh.role::text END FROM shares sh
JOIN targets t ON t.target_type = sh.target_type AND t.target_id = sh.target_id
LEFT JOIN custom_roles cr ON cr.id = sh.custom_role_id
WHERE (sh.principal_type = 'user' AND sh.principal_id = $1::uuid)
   OR (sh.principal_type = 'group' AND sh.principal_id IN
       (SELECT group_id FROM group_members WHERE user_id = $1::uuid))
`

func itemsCaps(ctx context.Context, db DB, actor Actor, itemIDs []string, spaceID *string, deleted, includeDeletedSpaces bool) (map[string]Caps, error) {
	if actor.Form != nil {
		return formItemsCaps(ctx, db, actor, itemIDs, spaceID, deleted)
	}
	rows, err := db.Query(ctx, strings.Replace(itemsCapsSQL, "/*ITEM_GATE*/", itemGate, 1), actor.ID, itemIDs, spaceID, deleted, includeDeletedSpaces)
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

// formItemsCaps gives a form submission formCaps on items placed in the form's Space, and nothing elsewhere or in the bin.
func formItemsCaps(ctx context.Context, db DB, actor Actor, itemIDs []string, spaceID *string, deleted bool) (map[string]Caps, error) {
	out := make(map[string]Caps, len(itemIDs))
	if deleted || (spaceID != nil && *spaceID != actor.Form.SpaceID) {
		return out, nil
	}
	// B3-05 applies to the requester: a form submission reaches only what the requester could reach in that Space.
	rows, err := db.Query(ctx, strings.Replace(`SELECT DISTINCT l.item_id::text FROM item_locations l JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
JOIN spaces s ON s.id=l.space_id
WHERE l.space_id=$2::uuid AND l.deleted_root_id IS NULL AND l.item_id=ANY($3::uuid[]) AND /*ITEM_GATE*/`, "/*ITEM_GATE*/", itemGate, 1),
		actor.ID, actor.Form.SpaceID, itemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = formCaps
	}
	return out, rows.Err()
}

// VisibleItemSpace names the first Space, in the item's location order, that the actor can see; "" when they see none. Use it
// for anything sent to a person: seeing a multi-location item does not disclose its other Spaces.
func VisibleItemSpace(ctx context.Context, db DB, actor Actor, itemID string) (string, error) {
	spaces, err := VisibleSpaces(ctx, db, actor, "")
	if err != nil {
		return "", err
	}
	names := map[string]string{}
	for _, s := range spaces {
		if s.Caps.Has(View) {
			names[s.ID] = s.Name
		}
	}
	rows, err := db.Query(ctx, "SELECT space_id::text FROM item_locations WHERE item_id=$1::uuid AND deleted_root_id IS NULL ORDER BY rank, space_id", itemID)
	if err != nil {
		return "", err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return "", err
	}
	for _, id := range ids {
		if name, ok := names[id]; ok {
			return name, nil
		}
	}
	return "", nil
}
