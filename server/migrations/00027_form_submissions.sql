-- +goose Up
-- 'private': only people who can manage forms in the Space see the form. 'internal': every signed-in user can submit it.
ALTER TABLE request_forms ADD COLUMN access text NOT NULL DEFAULT 'private' CHECK (access IN ('private', 'internal'));

-- One row per Item a form created. The form name is kept because the form can be deleted later.
CREATE TABLE form_submissions (
  id uuid PRIMARY KEY,
  form_id uuid REFERENCES request_forms(id) ON DELETE SET NULL,
  form_name text NOT NULL,
  item_id uuid NOT NULL UNIQUE REFERENCES items(id) ON DELETE CASCADE,
  requester_id uuid NOT NULL REFERENCES users(id),
  answers jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX form_submissions_requester_idx ON form_submissions (requester_id, created_at DESC);
CREATE INDEX form_submissions_form_idx ON form_submissions (form_id);
