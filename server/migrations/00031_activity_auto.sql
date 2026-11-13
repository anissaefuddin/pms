-- +goose Up
-- auto marks changes the system made on its own (auto-scheduling), so the Stream can hide them (NAV-04).
ALTER TABLE activity ADD COLUMN auto boolean NOT NULL DEFAULT false;
CREATE INDEX activity_stream_idx ON activity (created_at DESC, id DESC);
