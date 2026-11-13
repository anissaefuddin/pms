-- +goose Up
-- The login second step reuses user_tokens: a short-lived single-use challenge issued after the password is correct.
ALTER TYPE token_purpose ADD VALUE 'mfa';

-- One row per user who started TOTP setup. The secret is encrypted by the application (internal/twofactor);
-- confirmed_at stays NULL until the user proves the app works, and only a confirmed row turns 2FA on.
-- last_step is the newest time step already accepted, so a code cannot be used twice.
CREATE TABLE user_totp (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  secret_enc text NOT NULL,
  confirmed_at timestamptz,
  last_step bigint NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_totp_backup_codes (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_hash bytea NOT NULL,
  used_at timestamptz,
  PRIMARY KEY (user_id, code_hash)
);

-- Who changed a security setting, for the procedure "an admin resets someone's 2FA" and for the user's own changes.
CREATE TABLE security_events (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
  target_id uuid REFERENCES users(id) ON DELETE SET NULL,
  action text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX security_events_target_idx ON security_events (target_id, id DESC);
