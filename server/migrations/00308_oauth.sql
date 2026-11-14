-- +goose Up
CREATE TABLE oauth_clients (
    id uuid PRIMARY KEY,
    client_id text NOT NULL UNIQUE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    secret_hash bytea NOT NULL,
    redirect_uris text[] NOT NULL CHECK (cardinality(redirect_uris) BETWEEN 1 AND 10),
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);

-- One row per user approval; the token hashes rotate in place on refresh.
CREATE TABLE oauth_grants (
    id uuid PRIMARY KEY,
    client_uuid uuid NOT NULL REFERENCES oauth_clients(id),
    user_id uuid NOT NULL REFERENCES users(id),
    scopes text[] NOT NULL,
    access_hash bytea NOT NULL UNIQUE,
    refresh_hash bytea NOT NULL UNIQUE,
    access_expires_at timestamptz NOT NULL,
    refresh_expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);
CREATE INDEX oauth_grants_user_idx ON oauth_grants (user_id, created_at DESC);

CREATE TABLE oauth_codes (
    code_hash bytea PRIMARY KEY,
    client_uuid uuid NOT NULL REFERENCES oauth_clients(id),
    user_id uuid NOT NULL REFERENCES users(id),
    redirect_uri text NOT NULL,
    scopes text[] NOT NULL,
    code_challenge text NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    grant_id uuid
);

-- +goose Down
DROP TABLE oauth_codes;
DROP TABLE oauth_grants;
DROP TABLE oauth_clients;
