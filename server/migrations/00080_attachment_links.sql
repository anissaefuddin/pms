-- +goose Up
-- Link attachments (TASK-08B, D-83): an attachment may point to a URL instead of a stored file.
-- Such a row still has version 1 (storage_key 'link:<id>', size = URL length, sha256 = digest of the URL) so the immutable history stays uniform.
ALTER TABLE attachments ADD COLUMN link_url text CHECK (link_url IS NULL OR (length(link_url) <= 2048 AND link_url ~ '^https?://'));

-- +goose Down
ALTER TABLE attachments DROP COLUMN link_url;
