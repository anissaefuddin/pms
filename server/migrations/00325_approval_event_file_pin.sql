-- +goose Up
-- WA-D08: history rows keep the file version they were about, so a withdrawn review can be shown as "Cancelled" for that version.
-- Existing rows stay NULL: a withdrawal recorded before this migration cannot be tied to a version (append-only history).
ALTER TABLE approval_events ADD COLUMN attachment_id uuid, ADD COLUMN attachment_version integer;

-- +goose Down
ALTER TABLE approval_events DROP COLUMN attachment_version, DROP COLUMN attachment_id;
