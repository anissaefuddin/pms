-- +goose Up
-- Blueprint usage (SPC-13 follow-up): one row per successful use, written in the same transaction as the created items.
CREATE TABLE blueprint_uses (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  blueprint_id uuid NOT NULL REFERENCES blueprints(id) ON DELETE CASCADE,
  used_by uuid REFERENCES users(id) ON DELETE SET NULL,
  root_item_id uuid REFERENCES items(id) ON DELETE SET NULL,
  used_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX blueprint_uses_blueprint_idx ON blueprint_uses (blueprint_id, used_at);
