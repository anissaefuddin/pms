-- +goose Up
CREATE TABLE comments (
  id uuid PRIMARY KEY,
  item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  author_id uuid NOT NULL REFERENCES users(id),
  body jsonb NOT NULL CHECK (jsonb_typeof(body)='object'),
  body_text text NOT NULL,
  version integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  edited_at timestamptz,
  deleted_at timestamptz
);
CREATE INDEX comments_live_item_idx ON comments(item_id,created_at,id) WHERE deleted_at IS NULL;

CREATE TABLE comment_mentions (
  comment_id uuid NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
  mention_type text NOT NULL CHECK (mention_type IN ('user','group','followers')),
  target_id uuid,
  CHECK ((mention_type='followers') = (target_id IS NULL)),
  UNIQUE NULLS NOT DISTINCT (comment_id,mention_type,target_id)
);
