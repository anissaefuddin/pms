-- +goose Up
CREATE TABLE custom_fields (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    type text NOT NULL CHECK (type IN ('single_select', 'multi_select')),
    options jsonb NOT NULL CHECK (jsonb_typeof(options) = 'array' AND jsonb_array_length(options) BETWEEN 1 AND 100),
    position integer NOT NULL CHECK (position >= 0),
    archived_at timestamptz,
    UNIQUE (space_id, position) DEFERRABLE INITIALLY DEFERRED
);
