-- +goose Up
-- Account blueprints (instance scope): they belong to their owner, not to a Space. Only the owner and instance admins
-- can see, use, edit, or delete them. Existing Space blueprints keep their Space; existing account rows (kept in a
-- Personal Space until now) move to their owner and lose the Space link, so no data is deleted.
ALTER TABLE blueprints ALTER COLUMN space_id DROP NOT NULL;
ALTER TABLE blueprints ADD COLUMN owner_id uuid REFERENCES users(id);
UPDATE blueprints b SET owner_id = s.owner_id, space_id = NULL FROM spaces s WHERE s.id = b.space_id AND b.scope = 'account';
ALTER TABLE blueprints ADD CONSTRAINT blueprints_scope_owner_check CHECK (
  (scope = 'space' AND space_id IS NOT NULL AND owner_id IS NULL)
  OR (scope = 'account' AND space_id IS NULL AND owner_id IS NOT NULL));
CREATE UNIQUE INDEX blueprints_account_name_idx ON blueprints (owner_id, lower(name)) WHERE scope = 'account';

-- +goose Down
DROP INDEX blueprints_account_name_idx;
ALTER TABLE blueprints DROP CONSTRAINT blueprints_scope_owner_check;
UPDATE blueprints b SET space_id = (SELECT s.id FROM spaces s WHERE s.kind = 'personal' AND s.owner_id = b.owner_id LIMIT 1) WHERE b.scope = 'account' AND b.space_id IS NULL;
ALTER TABLE blueprints DROP COLUMN owner_id;
ALTER TABLE blueprints ALTER COLUMN space_id SET NOT NULL;
