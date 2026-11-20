-- +goose Up
-- CIT-D05: Quick Access shows an active custom type as a direct entry in the Create menu. Off by default for existing types.
-- It never grants or removes permission; built-in types have no Quick Access setting.
ALTER TABLE item_type_defs ADD COLUMN quick_access boolean NOT NULL DEFAULT false;
ALTER TABLE item_type_defs ADD CONSTRAINT item_type_quick_access_custom CHECK (NOT quick_access OR NOT is_builtin);

-- +goose Down
ALTER TABLE item_type_defs DROP CONSTRAINT item_type_quick_access_custom;
ALTER TABLE item_type_defs DROP COLUMN quick_access;
