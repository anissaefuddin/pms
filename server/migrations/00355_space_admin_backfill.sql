-- +goose Up
-- Spaces created after 00346 got no Space Admin, and only a Space Admin can name one (D-SA1). The creator is promoted while they
-- still hold the Full share written with the Space (same transaction, so the same moment). A self-created Full share added later
-- (a public Space joined and then upgraded, a direct share added to a group grant) is not the creator and is never promoted
-- (DEF-050). A Space without an identifiable creator stays without an admin.
INSERT INTO space_admins (space_id, user_id, granted_by)
SELECT DISTINCT ON (sh.target_id) sh.target_id, sh.principal_id, sh.principal_id
FROM shares sh
JOIN spaces s ON s.id = sh.target_id AND s.kind = 'shared'
JOIN users u ON u.id = sh.principal_id AND u.status = 'active' AND u.user_type IN ('admin', 'regular')
WHERE sh.target_type = 'space' AND sh.principal_type = 'user' AND sh.role = 'full' AND sh.custom_role_id IS NULL
  AND sh.created_by = sh.principal_id AND sh.created_at = s.created_at
  AND NOT EXISTS (SELECT 1 FROM space_admins sa WHERE sa.space_id = s.id)
ORDER BY sh.target_id, sh.created_at;

-- The bootstrap "General" Space had no member at all. Only a Space that never held anything goes to the account owner, so
-- no content is disclosed to someone who was not shared it (D-17.1).
WITH orphan AS (
  SELECT s.id, a.owner_id FROM spaces s
  JOIN account a ON a.id = 1
  JOIN users u ON u.id = a.owner_id AND u.user_type = 'admin' AND u.status <> 'deactivated'
  WHERE s.kind = 'shared' AND s.deleted_at IS NULL
    AND NOT EXISTS (SELECT 1 FROM space_admins sa WHERE sa.space_id = s.id)
    AND NOT EXISTS (SELECT 1 FROM shares sh WHERE sh.target_type = 'space' AND sh.target_id = s.id)
    AND NOT EXISTS (SELECT 1 FROM item_locations l WHERE l.space_id = s.id)
), shared AS (
  INSERT INTO shares (target_type, target_id, principal_type, principal_id, role, created_by)
  SELECT 'space', id, 'user', owner_id, 'full', owner_id FROM orphan
)
INSERT INTO space_admins (space_id, user_id, granted_by) SELECT id, owner_id, owner_id FROM orphan;

-- +goose Down
-- Data only: the granted rows look like any other Space Admin, so they stay.
SELECT 1;
