-- +goose Up
-- Blueprint edits (B5): every change bumps version; an edit states the version it read, so a stale edit gets 409 instead of
-- silently overwriting another change. Existing rows start at version 1.
ALTER TABLE blueprints ADD COLUMN version integer NOT NULL DEFAULT 1 CHECK (version > 0);
ALTER TABLE blueprints ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE blueprints DROP COLUMN updated_at;
ALTER TABLE blueprints DROP COLUMN version;
