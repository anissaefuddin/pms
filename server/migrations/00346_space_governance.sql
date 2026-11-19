-- +goose Up
-- Space governance (D-01 visibility, D-03 archive, D-04 icon, D-05 explicit Space Admin). Additive: no share row changes,
-- and existing Spaces start as private, which keeps their effective permissions.
ALTER TABLE spaces
  ADD COLUMN visibility text NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'public')),
  ADD COLUMN archived_at timestamptz,
  ADD COLUMN archived_by uuid REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN icon_key text CHECK (icon_key IS NULL OR icon_key ~ '^[a-z][a-z0-9-]{1,39}$'),
  ADD CONSTRAINT spaces_personal_is_private CHECK (kind <> 'personal' OR visibility = 'private');
CREATE INDEX spaces_public_live_idx ON spaces (name) WHERE deleted_at IS NULL AND visibility = 'public' AND kind = 'shared';
CREATE INDEX spaces_archived_idx ON spaces (id) WHERE archived_at IS NOT NULL;

-- Explicit Space Admins. A Full share does not make someone an admin (D-05).
CREATE TABLE space_admins (
  space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  granted_by uuid REFERENCES users(id) ON DELETE SET NULL,
  granted_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (space_id, user_id)
);
CREATE INDEX space_admins_user_idx ON space_admins (user_id);

-- Initial admin: the creator of each existing shared Space, when they created it with a Full share on themselves.
-- Spaces without such a row get no admin here; Workspace admins still manage them and can name one.
INSERT INTO space_admins (space_id, user_id, granted_by)
SELECT DISTINCT ON (sh.target_id) sh.target_id, sh.principal_id, sh.principal_id
FROM shares sh JOIN spaces s ON s.id = sh.target_id AND s.kind = 'shared'
WHERE sh.target_type = 'space' AND sh.principal_type = 'user' AND sh.role = 'full' AND sh.custom_role_id IS NULL
  AND sh.created_by = sh.principal_id
ORDER BY sh.target_id, sh.created_at
ON CONFLICT DO NOTHING;
