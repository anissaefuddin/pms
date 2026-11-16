-- +goose Up
-- WA-D07: each user's hidden views and personal tab order, per workspace (Space, Project or Folder id). Own data only; never shared.
ALTER TABLE users ADD COLUMN view_prefs jsonb NOT NULL DEFAULT '{}'::jsonb
  CHECK (jsonb_typeof(view_prefs) = 'object');

-- +goose Down
ALTER TABLE users DROP COLUMN view_prefs;
