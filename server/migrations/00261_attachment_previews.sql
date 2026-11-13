-- +goose Up
-- File previews and proofing annotations (FIL-02, D-80). Both hang off (attachment_id, version) so they stay with the version they were made on.
CREATE TABLE attachment_previews (
  attachment_id uuid NOT NULL,
  version integer NOT NULL,
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','ready','skipped','failed')),
  -- [{"page":1,"key":"...","width":800,"height":600}]; the keys are generated PNGs, never the upload itself.
  pages jsonb NOT NULL DEFAULT '[]',
  note text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (attachment_id, version),
  FOREIGN KEY (attachment_id, version) REFERENCES attachment_versions(attachment_id, version) ON DELETE CASCADE
);
CREATE TABLE attachment_annotations (
  id uuid PRIMARY KEY,
  attachment_id uuid NOT NULL,
  version integer NOT NULL,
  author_id uuid NOT NULL REFERENCES users(id),
  page integer NOT NULL DEFAULT 1 CHECK (page >= 1),
  x numeric(7,6) NOT NULL CHECK (x BETWEEN 0 AND 1),
  y numeric(7,6) NOT NULL CHECK (y BETWEEN 0 AND 1),
  body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 4000),
  resolved_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (attachment_id, version) REFERENCES attachment_versions(attachment_id, version) ON DELETE CASCADE
);
CREATE INDEX attachment_annotations_version ON attachment_annotations(attachment_id, version, created_at);

-- +goose Down
DROP TABLE attachment_annotations;
DROP TABLE attachment_previews;
