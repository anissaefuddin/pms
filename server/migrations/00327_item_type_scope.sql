-- +goose Up
-- CIT-D01: an item type belongs to the Workspace (space_id NULL, usable in every Space) or to one Space.
-- Built-in types stay Workspace-wide. Custom types that exist today were account-wide, so they stay Workspace-wide.
ALTER TABLE item_type_defs ADD COLUMN space_id uuid REFERENCES spaces(id);
ALTER TABLE item_type_defs ADD CONSTRAINT item_type_defs_builtin_workspace CHECK (NOT is_builtin OR space_id IS NULL);
CREATE INDEX item_type_defs_space_idx ON item_type_defs (space_id) WHERE space_id IS NOT NULL;

-- +goose Down
DROP INDEX item_type_defs_space_idx;
ALTER TABLE item_type_defs DROP CONSTRAINT item_type_defs_builtin_workspace;
ALTER TABLE item_type_defs DROP COLUMN space_id;
