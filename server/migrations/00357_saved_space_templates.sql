-- +goose Up
-- A Space saved as a template (DEF-012). body is a spacetemplates.Template: structure and configuration only, no people, dates,
-- statuses, field values, comments, files, time, forms, automation or membership.
CREATE TABLE saved_space_templates (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  created_by uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_space_id uuid REFERENCES spaces(id) ON DELETE SET NULL,
  body jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX saved_space_templates_created_by_idx ON saved_space_templates (created_by);
CREATE INDEX saved_space_templates_source_idx ON saved_space_templates (source_space_id);

-- +goose Down
DROP TABLE saved_space_templates;
