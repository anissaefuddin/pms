-- +goose Up
ALTER TABLE items ADD COLUMN status_changed_at timestamptz NOT NULL DEFAULT now();
UPDATE items SET status_changed_at = updated_at;
