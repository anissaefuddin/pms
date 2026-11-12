-- +goose Up
CREATE TYPE user_type AS ENUM ('admin', 'regular', 'external', 'collaborator', 'viewer');
CREATE TYPE user_status AS ENUM ('invited', 'active', 'deactivated');
CREATE TYPE token_purpose AS ENUM ('invite', 'reset');

-- Exactly one row per instance (D-01, D-13): workspace metadata only, never referenced by domain tables.
CREATE TABLE account (
  id smallint PRIMARY KEY CHECK (id = 1),
  name text NOT NULL,
  settings jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email citext NOT NULL UNIQUE,
  first_name text NOT NULL DEFAULT '',
  last_name text NOT NULL DEFAULT '',
  password_hash text,
  user_type user_type NOT NULL DEFAULT 'regular',
  status user_status NOT NULL DEFAULT 'invited',
  job_title text NOT NULL DEFAULT '',
  department text NOT NULL DEFAULT '',
  locale text NOT NULL DEFAULT 'en',
  timezone text NOT NULL DEFAULT 'UTC',
  date_format text NOT NULL DEFAULT 'yyyy-mm-dd' CHECK (date_format IN ('dd/mm/yyyy', 'mm/dd/yyyy', 'yyyy-mm-dd')),
  time_format text NOT NULL DEFAULT '24h' CHECK (time_format IN ('12h', '24h')),
  week_start text NOT NULL DEFAULT 'monday' CHECK (week_start IN ('monday', 'sunday', 'saturday')),
  last_login_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  -- an invited user has no password yet; an active user has a password and a name
  CONSTRAINT users_active_has_password CHECK (status <> 'active' OR password_hash IS NOT NULL),
  CONSTRAINT users_invited_has_no_password CHECK (status <> 'invited' OR password_hash IS NULL),
  CONSTRAINT users_active_has_name CHECK (status <> 'active' OR (first_name <> '' AND last_name <> ''))
);
CREATE INDEX users_status_idx ON users (status);
CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Only the SHA-256 of the token is stored. expires_at slides with activity (idle limit);
-- absolute_expires_at is fixed at login and never extended.
CREATE TABLE sessions (
  token_hash bytea PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  absolute_expires_at timestamptz NOT NULL,
  CONSTRAINT sessions_idle_within_absolute CHECK (expires_at <= absolute_expires_at)
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at); -- expires_at <= absolute_expires_at, so this serves cleanup of both limits

-- Invitations and password resets share one table; single use is enforced with UPDATE ... WHERE used_at IS NULL.
CREATE TABLE user_tokens (
  token_hash bytea PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  purpose token_purpose NOT NULL,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_tokens_user_id_idx ON user_tokens (user_id, purpose);
CREATE INDEX user_tokens_expires_at_idx ON user_tokens (expires_at);
