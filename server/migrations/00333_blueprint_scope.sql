-- +goose Up
-- Blueprint scope (B5): a blueprint lives either in a shared Space ('space') or in an account's Personal Space ('account',
-- owner only). Existing rows keep their Space; the scope follows the Space kind, so nothing becomes visible to anyone new.
ALTER TABLE blueprints ADD COLUMN scope text NOT NULL DEFAULT 'space' CHECK (scope IN ('space', 'account'));
UPDATE blueprints b SET scope = 'account' FROM spaces s WHERE s.id = b.space_id AND s.kind = 'personal';

-- Blueprint rights are appended to the capability bits (policy.AllCapabilities: bit 8 use, 9 create, 10 manage).
-- An existing custom role keeps exactly the rights it had: creating items from a blueprint used Create, saving and
-- deleting used Edit. The mask limit grows from 8 to 11 bits.
ALTER TABLE custom_roles DROP CONSTRAINT custom_roles_caps_mask_check;
UPDATE custom_roles SET caps_mask = caps_mask
  | (CASE WHEN caps_mask & 32 <> 0 THEN 256 ELSE 0 END)
  | (CASE WHEN caps_mask & 16 <> 0 THEN 512 | 1024 ELSE 0 END);
ALTER TABLE custom_roles ADD CONSTRAINT custom_roles_caps_mask_check CHECK (caps_mask BETWEEN 1 AND 2047 AND caps_mask & 1 = 1);

-- +goose Down
UPDATE custom_roles SET caps_mask = caps_mask & 255 WHERE caps_mask > 255;
ALTER TABLE custom_roles DROP CONSTRAINT custom_roles_caps_mask_check;
ALTER TABLE custom_roles ADD CONSTRAINT custom_roles_caps_mask_check CHECK (caps_mask BETWEEN 1 AND 255 AND caps_mask & 1 = 1);
ALTER TABLE blueprints DROP COLUMN scope;
