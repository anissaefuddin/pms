-- +goose Up
-- File versions (FIL-01, D-S11.1.2). attachment_versions is the immutable history; attachments keeps mirroring the
-- newest version so list/download/cover stay compatible. No FK on storage: a version row is the only thing that keeps its file alive.
ALTER TABLE attachments ADD COLUMN current_version integer NOT NULL DEFAULT 1;
CREATE TABLE attachment_versions (
  attachment_id uuid NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
  version integer NOT NULL CHECK (version >= 1),
  uploader_id uuid NOT NULL REFERENCES users(id),
  filename text NOT NULL,
  content_type text NOT NULL,
  size_bytes bigint NOT NULL CHECK (size_bytes > 0),
  storage_key text NOT NULL UNIQUE,
  sha256 bytea NOT NULL CHECK (octet_length(sha256)=32),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (attachment_id, version)
);
INSERT INTO attachment_versions(attachment_id,version,uploader_id,filename,content_type,size_bytes,storage_key,sha256,created_at)
SELECT id,1,uploader_id,filename,content_type,size_bytes,storage_key,sha256,created_at FROM attachments;
-- +goose StatementBegin
CREATE FUNCTION attachment_versions_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  -- Rows go away only with their attachment (purged item, cascade); never by a direct UPDATE/DELETE.
  IF TG_OP = 'DELETE' AND pg_trigger_depth() > 1 THEN RETURN OLD; END IF;
  RAISE EXCEPTION 'attachment versions are immutable' USING ERRCODE = 'restrict_violation';
END $$;
-- +goose StatementEnd
CREATE TRIGGER attachment_versions_no_change BEFORE UPDATE OR DELETE ON attachment_versions FOR EACH ROW EXECUTE FUNCTION attachment_versions_immutable();

-- +goose Down
DROP TABLE attachment_versions;
DROP FUNCTION attachment_versions_immutable();
ALTER TABLE attachments DROP COLUMN current_version;
