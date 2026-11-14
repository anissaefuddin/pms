-- +goose Up
CREATE TABLE api_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    scope text NOT NULL CHECK (scope IN ('read','write')),
    prefix text NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz,
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX api_keys_user_idx ON api_keys (user_id, created_at DESC);

-- +goose Down
DROP TABLE api_keys;
