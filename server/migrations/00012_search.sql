-- +goose Up
ALTER TABLE comments ADD COLUMN search tsvector GENERATED ALWAYS AS (
  to_tsvector('simple', body_text)
) STORED;
CREATE INDEX comments_search_idx ON comments USING gin (search) WHERE deleted_at IS NULL;

ALTER TABLE users ADD COLUMN search tsvector GENERATED ALWAYS AS (
  to_tsvector('simple', first_name || ' ' || last_name)
) STORED;
CREATE INDEX users_search_idx ON users USING gin (search) WHERE status = 'active';
