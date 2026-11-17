-- +goose Up
-- Field-level access (CF-D03). A field with no rows here inherits item access. Rows make it restricted (see items/field_access.go).
CREATE TABLE custom_field_grants (
    field_id uuid NOT NULL REFERENCES custom_fields(id) ON DELETE CASCADE,
    subject_kind text NOT NULL CHECK (subject_kind IN ('user', 'group')),
    subject_id uuid NOT NULL,
    level text NOT NULL CHECK (level IN ('view', 'edit', 'none')),
    PRIMARY KEY (field_id, subject_kind, subject_id)
);
CREATE INDEX custom_field_grants_subject_idx ON custom_field_grants (subject_kind, subject_id);

-- +goose Down
DROP TABLE custom_field_grants;
