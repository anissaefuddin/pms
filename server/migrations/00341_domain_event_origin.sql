-- +goose Up
-- The Space a change happened in, when the emitter knows it. Nullable so events written before this column, and emitters not yet
-- migrated, stay valid; automation resolves those by the item's owning Space or skips them when that is ambiguous.
ALTER TABLE domain_events ADD COLUMN origin_space_id uuid REFERENCES spaces(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE domain_events DROP COLUMN origin_space_id;
