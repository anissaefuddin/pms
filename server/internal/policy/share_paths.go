package policy

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// SharedOnEveryPath reports, for Items placed in one Space, whether the actor holds Share on every path from the Item up to the
// Space: on the Item itself, or, for each of its places there, on the Space or somewhere up that chain. ItemCaps adds all places
// together, which is right for reading and editing; handing work to someone else must not be gained by adding one more place under
// a folder the actor controls.
func SharedOnEveryPath(ctx context.Context, db DB, actor Actor, spaceID string, itemIDs []string) (map[string]bool, error) {
	out := map[string]bool{}
	if actor.Form != nil || len(itemIDs) == 0 {
		return out, nil
	}
	rows, err := db.Query(ctx, `
WITH RECURSIVE edges(item_id, parent_id) AS (
  SELECT item_id, parent_item_id FROM item_locations
  WHERE space_id=$3::uuid AND item_id=ANY(ARRAY(SELECT unnest($2::text[])::uuid)) AND deleted_root_id IS NULL
  UNION
  SELECT l.item_id, l.parent_item_id FROM edges e
  JOIN item_locations l ON l.item_id=e.parent_id AND l.space_id=$3::uuid AND l.deleted_root_id IS NULL
), nodes(target_type, target_id) AS (
  SELECT 'item'::share_target, item_id FROM edges UNION SELECT 'space'::share_target, $3::uuid
)
SELECT 'edge', item_id::text, COALESCE(parent_id::text,'') FROM edges
UNION ALL
SELECT sh.target_type::text, sh.target_id::text, CASE WHEN cr.id IS NOT NULL THEN 'mask:' || cr.caps_mask ELSE sh.role::text END
FROM shares sh JOIN nodes n ON n.target_type=sh.target_type AND n.target_id=sh.target_id
LEFT JOIN custom_roles cr ON cr.id=sh.custom_role_id
WHERE (sh.principal_type='user' AND sh.principal_id=$1::uuid)
   OR (sh.principal_type='group' AND sh.principal_id IN (SELECT group_id FROM group_members WHERE user_id=$1::uuid))
UNION ALL
SELECT 'space', sa.space_id::text, 'full' FROM space_admins sa JOIN users u ON u.id=sa.user_id
WHERE sa.space_id=$3::uuid AND sa.user_id=$1::uuid AND u.user_type IN ('admin','regular')`, actor.ID, itemIDs, spaceID)
	if err != nil {
		return nil, err
	}
	type row struct{ Kind, ID, Value string }
	found, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
	if err != nil {
		return nil, err
	}
	parents, granted, space := map[string][]string{}, map[string]bool{}, false
	for _, r := range found {
		switch {
		case r.Kind == "edge":
			parents[r.ID] = append(parents[r.ID], r.Value)
		case !Effective(actor.Type, Role(r.Value)).Has(Share):
		case r.Kind == "space":
			space = true
		default:
			granted[r.ID] = true
		}
	}
	memo := map[string]bool{}
	var every func(string) bool
	every = func(id string) bool {
		if space || granted[id] {
			return true
		}
		if v, ok := memo[id]; ok {
			return v
		}
		memo[id] = false // a cycle, which the database forbids, would not grant
		ok := len(parents[id]) > 0
		for _, parent := range parents[id] {
			ok = ok && parent != "" && every(parent)
		}
		memo[id] = ok
		return ok
	}
	for _, id := range itemIDs {
		out[id] = every(id)
	}
	return out, nil
}
