-- +goose Up
-- Workspace custom fields: a field with space_id NULL belongs to the Workspace. Admins manage them, and a Workspace item
-- type may use only these. Space fields are unchanged.
ALTER TABLE custom_fields ALTER COLUMN space_id DROP NOT NULL;

-- +goose Down
DELETE FROM item_type_fields WHERE field_id IN (SELECT id FROM custom_fields WHERE space_id IS NULL);
DELETE FROM custom_fields WHERE space_id IS NULL;
ALTER TABLE custom_fields ALTER COLUMN space_id SET NOT NULL;
