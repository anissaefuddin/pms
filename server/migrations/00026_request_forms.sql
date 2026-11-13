-- +goose Up
-- A form belongs to a shared Space. `definition` (questions) and `response` (what a submission creates) are
-- validated by the forms package before they are stored; their destination ids are checked again at submit time.
CREATE TABLE request_forms (
  id uuid PRIMARY KEY,
  space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000),
  definition jsonb NOT NULL,
  response jsonb NOT NULL,
  version integer NOT NULL DEFAULT 1,
  created_by uuid NOT NULL REFERENCES users(id),
  updated_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX request_forms_name_idx ON request_forms (space_id, lower(name));
CREATE TRIGGER request_forms_set_updated_at BEFORE UPDATE ON request_forms FOR EACH ROW EXECUTE FUNCTION set_updated_at();
