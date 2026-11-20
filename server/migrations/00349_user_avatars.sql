-- +goose Up
-- Profile photo (SET-01): one image per user, kept in the database so backups and deployments need no file volume.
CREATE TABLE user_avatars (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  content_type text NOT NULL CHECK (content_type IN ('image/png', 'image/jpeg', 'image/webp')),
  data bytea NOT NULL CHECK (octet_length(data) BETWEEN 1 AND 2097152),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE user_avatars;
