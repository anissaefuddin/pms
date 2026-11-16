-- +goose Up
-- WA-D05: each user's expanded rows and Gantt collapse flag per view, keyed by view id. Own data only; never shared.
ALTER TABLE users ADD COLUMN view_expansion jsonb NOT NULL DEFAULT '{}'::jsonb
  CHECK (jsonb_typeof(view_expansion) = 'object');

-- +goose Down
ALTER TABLE users DROP COLUMN view_expansion;
