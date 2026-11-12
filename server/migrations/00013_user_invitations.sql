-- +goose Up
ALTER TABLE users
  ADD COLUMN invitation_generation bigint NOT NULL DEFAULT 0,
  ADD COLUMN invitation_message text NOT NULL DEFAULT '',
  ADD COLUMN invited_by uuid REFERENCES users(id) ON DELETE SET NULL;
