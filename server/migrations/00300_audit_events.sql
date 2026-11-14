-- +goose Up
-- Security audit trail (ENT-04): who did what to whom, from where. One table for every area (login, users, shares, roles, SSO, API keys);
-- the action is "area.verb". actor_name is a snapshot (the email) so an entry stays readable after the user changes or is removed.
CREATE TABLE audit_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    at timestamptz NOT NULL DEFAULT now(),
    actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
    actor_name text NOT NULL DEFAULT '',
    action text NOT NULL CHECK (action ~ '^[a-z0-9_]+(\.[a-z0-9_]+)+$' AND char_length(action) <= 80),
    target_type text NOT NULL DEFAULT '' CHECK (char_length(target_type) <= 40),
    target_id text NOT NULL DEFAULT '' CHECK (char_length(target_id) <= 80),
    ip text NOT NULL DEFAULT '' CHECK (char_length(ip) <= 64),
    details jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(details) = 'object' AND pg_column_size(details) <= 4096)
);
CREATE INDEX audit_events_at_idx ON audit_events (id DESC);
CREATE INDEX audit_events_action_idx ON audit_events (action, id DESC);
CREATE INDEX audit_events_actor_idx ON audit_events (actor_id, id DESC);

-- The 2FA changes recorded so far move into the trail; security_events stays as history and no longer receives rows.
INSERT INTO audit_events (at, actor_id, actor_name, action, target_type, target_id)
SELECT e.created_at, e.actor_id, COALESCE(a.email::text, ''), 'auth.' || regexp_replace(e.action, '[^a-z0-9_]', '_', 'g'), 'user', COALESCE(e.target_id::text, '')
FROM security_events e LEFT JOIN users a ON a.id = e.actor_id ORDER BY e.id;
