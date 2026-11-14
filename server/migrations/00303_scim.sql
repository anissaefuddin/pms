-- +goose Up
-- SCIM provisioning (ENT-02). A token lets an identity provider create, change and deprovision users and groups; only its hash is stored.
CREATE TABLE scim_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 80),
    token_hash bytea NOT NULL UNIQUE,
    prefix text NOT NULL,
    default_user_type user_type NOT NULL DEFAULT 'regular' CHECK (default_user_type <> 'admin'),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at timestamptz
);

-- A provisioned user may have no password: they sign in through single sign-on. Only provisioning can create such a user, and it stays
-- possible to give one a password later (the reset link works for them like for anyone).
ALTER TABLE users ADD COLUMN scim_external_id text CHECK (char_length(scim_external_id) <= 255);
ALTER TABLE users ADD COLUMN scim_provisioned boolean NOT NULL DEFAULT false;
CREATE UNIQUE INDEX users_scim_external_idx ON users (scim_external_id) WHERE scim_external_id IS NOT NULL;
ALTER TABLE users DROP CONSTRAINT users_active_has_password;
ALTER TABLE users ADD CONSTRAINT users_active_has_password CHECK (status <> 'active' OR password_hash IS NOT NULL OR scim_provisioned);

ALTER TABLE groups ADD COLUMN scim_external_id text CHECK (char_length(scim_external_id) <= 255);
CREATE UNIQUE INDEX groups_scim_external_idx ON groups (scim_external_id) WHERE scim_external_id IS NOT NULL;
