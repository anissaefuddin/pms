-- +goose Up
ALTER TABLE custom_fields
    DROP CONSTRAINT custom_fields_type_check,
    DROP CONSTRAINT custom_fields_options_check,
    ADD COLUMN config jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(config) = 'object'),
    ADD CONSTRAINT custom_fields_type_check CHECK (type IN (
        'single_select', 'multi_select', 'text', 'number', 'currency', 'percent', 'people', 'date', 'duration', 'checkbox'
    )),
    ADD CONSTRAINT custom_fields_options_check CHECK (
        CASE
            WHEN type IN ('single_select', 'multi_select') THEN
                CASE WHEN jsonb_typeof(options) = 'array' THEN jsonb_array_length(options) BETWEEN 1 AND 100 ELSE false END
            ELSE options = '[]'::jsonb
        END
    );
