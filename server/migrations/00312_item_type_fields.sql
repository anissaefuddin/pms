-- +goose Up
-- S8.1.2: which custom fields apply to an item type, and the value a new item of that type starts with (NULL = applies, no default).
-- Deleting a type or a field removes the pairing; an archived field is skipped when defaults are applied.
CREATE TABLE item_type_fields (
    type_id       uuid NOT NULL REFERENCES item_type_defs(id) ON DELETE CASCADE,
    field_id      uuid NOT NULL REFERENCES custom_fields(id) ON DELETE CASCADE,
    default_value jsonb,
    PRIMARY KEY (type_id, field_id)
);
CREATE INDEX item_type_fields_field_idx ON item_type_fields (field_id);

-- +goose Down
DROP TABLE item_type_fields;
