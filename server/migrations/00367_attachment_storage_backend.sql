-- +goose Up
-- Where a file's bytes are. NULL = written before this column (or a link attachment): the backend in PMS_STORAGE. Adding a column
-- without a default rewrites no row, so the immutable version history is untouched.
ALTER TABLE attachments ADD COLUMN storage_backend text CHECK (storage_backend IN ('local', 's3', 'pydio'));
ALTER TABLE attachment_versions ADD COLUMN storage_backend text CHECK (storage_backend IN ('local', 's3', 'pydio'));

-- +goose Down
ALTER TABLE attachment_versions DROP COLUMN storage_backend;
ALTER TABLE attachments DROP COLUMN storage_backend;
