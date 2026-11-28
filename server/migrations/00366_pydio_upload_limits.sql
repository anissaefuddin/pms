-- +goose Up
-- Limits for uploads to Pydio storage. An empty extension list allows every file type. PMS does not upload to Pydio yet, and local
-- attachments keep PMS_MAX_UPLOAD_MB.
ALTER TABLE pydio_settings
    ADD COLUMN max_upload_mb integer NOT NULL DEFAULT 100 CHECK (max_upload_mb BETWEEN 1 AND 1024),
    ADD COLUMN allowed_extensions text[] NOT NULL DEFAULT '{}' CHECK (cardinality(allowed_extensions) <= 100);

-- +goose Down
ALTER TABLE pydio_settings DROP COLUMN max_upload_mb, DROP COLUMN allowed_extensions;
