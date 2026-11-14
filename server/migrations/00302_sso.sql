-- +goose Up
-- Single sign-on (ENT-01). One provider per kind (oidc, saml). Local passwords keep working: SSO adds a way in, it does not remove one.
CREATE TABLE sso_providers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL UNIQUE CHECK (kind IN ('oidc', 'saml')),
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 60),
    enabled boolean NOT NULL DEFAULT false,
    config jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(config) = 'object' AND pg_column_size(config) <= 65536),
    secret_enc text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- A person at the identity provider (subject) linked to a user here. Matching is by subject, never by email after the first link.
CREATE TABLE user_identities (
    provider_id uuid NOT NULL REFERENCES sso_providers(id) ON DELETE CASCADE,
    subject text NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 255),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email text NOT NULL DEFAULT '' CHECK (char_length(email) <= 320),
    created_at timestamptz NOT NULL DEFAULT now(),
    last_login_at timestamptz,
    PRIMARY KEY (provider_id, subject)
);
CREATE INDEX user_identities_user_idx ON user_identities (user_id);

-- One row per started sign-in. state_hash is what the identity provider sends back; browser_hash is the cookie that started the flow,
-- so a state copied into another browser is useless. Rows are single use and live ten minutes.
CREATE TABLE sso_states (
    state_hash bytea PRIMARY KEY,
    browser_hash bytea NOT NULL,
    provider_id uuid NOT NULL REFERENCES sso_providers(id) ON DELETE CASCADE,
    nonce text NOT NULL,
    code_verifier text NOT NULL DEFAULT '',
    purpose text NOT NULL CHECK (purpose IN ('login', 'link')),
    user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    next text NOT NULL DEFAULT '/' CHECK (char_length(next) <= 500),
    expires_at timestamptz NOT NULL
);
CREATE INDEX sso_states_expires_idx ON sso_states (expires_at);
