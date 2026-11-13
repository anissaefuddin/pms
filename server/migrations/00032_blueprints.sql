-- +goose Up
-- A blueprint is a copy of an item and its descendants, kept as a tree in `tree`, never linked to the source.
-- Nodes are parent-first; dates are day offsets from the earliest date so they can be re-anchored on use.
CREATE TABLE blueprints (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
  name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
  tree jsonb NOT NULL,
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX blueprints_name_idx ON blueprints (space_id, lower(name));
