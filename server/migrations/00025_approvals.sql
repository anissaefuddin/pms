-- +goose Up
CREATE TABLE approvals (
  id uuid PRIMARY KEY,
  item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000),
  due_date date,
  created_by uuid NOT NULL REFERENCES users(id),
  version integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX approvals_item_idx ON approvals (item_id, created_at DESC);
CREATE TRIGGER approvals_set_updated_at BEFORE UPDATE ON approvals FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE approval_approvers (
  approval_id uuid NOT NULL REFERENCES approvals(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id),
  decision text CHECK (decision IN ('approved', 'rejected')),
  comment text NOT NULL DEFAULT '' CHECK (char_length(comment) <= 2000),
  decided_at timestamptz,
  PRIMARY KEY (approval_id, user_id),
  CHECK ((decision IS NULL) = (decided_at IS NULL))
);
-- "Waiting for my decision" lookups (dashboards, My to-do later).
CREATE INDEX approval_approvers_pending_idx ON approval_approvers (user_id) WHERE decision IS NULL;

ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
  CHECK (kind IN ('mention','assigned','status_changed','comment','shared','approval_requested','approval_decided'));
