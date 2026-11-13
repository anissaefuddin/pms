-- +goose Up
-- Calculated custom fields (TASK-11B): the expression lives in custom_fields.config, results in items.fields like any other value.
ALTER TABLE custom_fields DROP CONSTRAINT custom_fields_type_check,
    ADD CONSTRAINT custom_fields_type_check CHECK (type IN (
        'single_select', 'multi_select', 'text', 'number', 'currency', 'percent', 'people', 'date', 'duration', 'checkbox', 'relation', 'formula'
    ));
