-- +goose Up
-- Timesheet submit and approve (DEF-011). Time logged before this has no review step and already counts in the actual cost, so it
-- is taken as approved; new time starts as a draft and costs nothing until a reviewer approves it.
ALTER TABLE time_entries
  ADD COLUMN status text NOT NULL DEFAULT 'approved' CHECK (status IN ('draft', 'submitted', 'approved', 'rejected')),
  ADD COLUMN submitted_at timestamptz,
  ADD COLUMN reviewed_by uuid REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN reviewed_at timestamptz,
  ADD COLUMN review_note text NOT NULL DEFAULT '' CHECK (char_length(review_note) <= 1000);
ALTER TABLE time_entries ALTER COLUMN status SET DEFAULT 'draft';
CREATE INDEX time_entries_submitted_idx ON time_entries (entry_date, id) WHERE status = 'submitted';

-- +goose Down
ALTER TABLE time_entries DROP COLUMN review_note, DROP COLUMN reviewed_at, DROP COLUMN reviewed_by, DROP COLUMN submitted_at, DROP COLUMN status;
