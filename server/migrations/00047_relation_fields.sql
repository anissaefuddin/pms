-- +goose Up
-- Typed relation custom fields (CUST-05, ADR-REG). The relation table is the source of truth for both directions;
-- items.fields keeps the target ids of the field for display and for the existing value plumbing.
ALTER TABLE custom_fields DROP CONSTRAINT custom_fields_type_check,
    ADD CONSTRAINT custom_fields_type_check CHECK (type IN (
        'single_select', 'multi_select', 'text', 'number', 'currency', 'percent', 'people', 'date', 'duration', 'checkbox', 'relation'
    ));
CREATE TABLE item_relations (
    field_id uuid NOT NULL REFERENCES custom_fields(id) ON DELETE CASCADE,
    from_item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    to_item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (field_id, from_item_id, to_item_id),
    CHECK (from_item_id <> to_item_id)
);
CREATE INDEX item_relations_to_idx ON item_relations (to_item_id, field_id);
