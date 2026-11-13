-- +goose Up
-- One row per "create a Space from a template" request, so a retry with the same request_id returns the Space made the first time.
CREATE TABLE space_template_runs (
    request_id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    space_id uuid REFERENCES spaces(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);
