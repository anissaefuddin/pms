-- +goose Up
CREATE TABLE reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id uuid NOT NULL REFERENCES users(id),
    space_id uuid REFERENCES spaces(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    -- {"layout": "table"|"column", "query": <analytics query>}; the API validates it with the same schema the engine runs.
    definition jsonb NOT NULL CHECK (jsonb_typeof(definition) = 'object' AND pg_column_size(definition) <= 8192),
    sharing text NOT NULL DEFAULT 'private' CHECK (sharing IN ('private', 'space')),
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (sharing = 'private' OR space_id IS NOT NULL)
);
CREATE INDEX reports_owner_idx ON reports(owner_id, id);
CREATE INDEX reports_space_idx ON reports(space_id, id) WHERE space_id IS NOT NULL;
CREATE TRIGGER reports_set_updated_at BEFORE UPDATE ON reports FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One subscription per user and report; delivery (RPT-04) reads weekday (0 = Sunday) and hour in the subscriber's time zone.
CREATE TABLE report_subscriptions (
    report_id uuid NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id),
    weekday smallint NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    hour smallint NOT NULL CHECK (hour BETWEEN 0 AND 23),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (report_id, user_id)
);
CREATE INDEX report_subscriptions_user_idx ON report_subscriptions(user_id);
