-- +goose Up
-- One-time password sign-in (AUTH-08) and account ownership (SET-09). See D-97.
CREATE TABLE login_codes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash bytea NOT NULL,
    attempts smallint NOT NULL DEFAULT 0,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX login_codes_user_idx ON login_codes (user_id, created_at DESC);
CREATE INDEX login_codes_expires_idx ON login_codes (expires_at);

-- Off until an administrator turns it on: a code in the mailbox becomes a way in, so it is a deliberate choice.
ALTER TABLE access_policy ADD COLUMN otp_login_enabled boolean NOT NULL DEFAULT false;

-- The owner is the person who answers for the account. Existing instances: the oldest administrator.
ALTER TABLE account ADD COLUMN owner_id uuid REFERENCES users(id) ON DELETE SET NULL;
UPDATE account SET owner_id = (SELECT id FROM users WHERE user_type = 'admin' AND id <> '00000000-0000-4000-8000-0000000000e1' ORDER BY created_at, id LIMIT 1);
