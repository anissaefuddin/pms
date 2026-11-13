-- +goose Up
ALTER TABLE statuses
  ADD COLUMN auto_assign boolean NOT NULL DEFAULT false,
  ADD COLUMN lock_transitions boolean NOT NULL DEFAULT false;

-- When a status has lock_transitions, an Item may only leave it for the listed statuses.
-- The API checks that both ends belong to the same workflow.
CREATE TABLE status_transitions (
  from_status_id uuid NOT NULL REFERENCES statuses(id) ON DELETE CASCADE,
  to_status_id uuid NOT NULL REFERENCES statuses(id) ON DELETE CASCADE,
  PRIMARY KEY (from_status_id, to_status_id),
  CHECK (from_status_id <> to_status_id)
);
CREATE INDEX status_transitions_to_idx ON status_transitions (to_status_id);
CREATE UNIQUE INDEX workflows_name_ci ON workflows (lower(name));
