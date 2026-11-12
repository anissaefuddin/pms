-- +goose Up
CREATE TABLE notifications (
  id uuid PRIMARY KEY,
  recipient_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
  kind text NOT NULL CHECK (kind IN ('mention','assigned','status_changed','comment','shared')),
  item_id uuid REFERENCES items(id) ON DELETE CASCADE,
  space_id uuid REFERENCES spaces(id) ON DELETE CASCADE,
  snippet text,
  read_at timestamptz,
  archived_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (num_nonnulls(item_id,space_id)=1)
);
CREATE INDEX notifications_inbox_idx ON notifications(recipient_id,id DESC) WHERE archived_at IS NULL;
