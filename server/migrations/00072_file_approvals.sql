-- +goose Up
-- Approval pinned to one file version (FIL-04, D-80). Both columns or neither; the version row can never change, so the pin cannot drift to v2.
ALTER TABLE approvals ADD COLUMN attachment_id uuid, ADD COLUMN attachment_version integer,
  ADD CONSTRAINT approvals_file_pin_check CHECK ((attachment_id IS NULL) = (attachment_version IS NULL)),
  ADD CONSTRAINT approvals_file_pin_fk FOREIGN KEY (attachment_id, attachment_version) REFERENCES attachment_versions(attachment_id, version) ON DELETE CASCADE;
CREATE INDEX approvals_file_idx ON approvals (attachment_id, attachment_version) WHERE attachment_id IS NOT NULL;

-- +goose Down
DROP INDEX approvals_file_idx;
ALTER TABLE approvals DROP CONSTRAINT approvals_file_pin_fk, DROP CONSTRAINT approvals_file_pin_check, DROP COLUMN attachment_version, DROP COLUMN attachment_id;
