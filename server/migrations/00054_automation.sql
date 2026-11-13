-- +goose Up
-- Automation rules (AUT-02, AUT-03, ADR-AUT). A rule lives in a Space (optionally narrowed to one project or folder) and runs as its owner.
-- Versions are immutable once published; only one draft exists per rule; publishing swaps current_version_id in one transaction.
CREATE TABLE automation_rules (
    id uuid PRIMARY KEY,
    space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    container_id uuid REFERENCES items(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    owner_id uuid NOT NULL REFERENCES users(id),
    enabled boolean NOT NULL DEFAULT false,
    current_version_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX automation_rules_space_idx ON automation_rules (space_id);

CREATE TABLE automation_rule_versions (
    id uuid PRIMARY KEY,
    rule_id uuid NOT NULL REFERENCES automation_rules(id) ON DELETE CASCADE,
    version integer NOT NULL CHECK (version > 0),
    definition jsonb NOT NULL CHECK (jsonb_typeof(definition) = 'object'),
    status text NOT NULL CHECK (status IN ('draft', 'published', 'retired')),
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    UNIQUE (rule_id, version)
);
CREATE UNIQUE INDEX automation_one_draft_idx ON automation_rule_versions (rule_id) WHERE status = 'draft';
ALTER TABLE automation_rules ADD CONSTRAINT automation_rules_current_fk FOREIGN KEY (current_version_id) REFERENCES automation_rule_versions(id);

-- One row per outcome. A terminal outcome is unique per (version, event, action), which is what makes a redelivered event
-- change nothing twice; failures are logged freely and retried. action_index -1 is the rule-level outcome (no match, blocked).
CREATE TABLE automation_runs (
    id uuid PRIMARY KEY,
    rule_id uuid NOT NULL REFERENCES automation_rules(id) ON DELETE CASCADE,
    rule_version_id uuid NOT NULL REFERENCES automation_rule_versions(id) ON DELETE CASCADE,
    event_id uuid NOT NULL,
    action_index integer NOT NULL,
    item_id uuid,
    actor_id uuid,
    status text NOT NULL CHECK (status IN ('done', 'denied', 'skipped', 'loop_blocked', 'quota_blocked', 'failed')),
    detail text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX automation_runs_once_idx ON automation_runs (rule_version_id, event_id, action_index) WHERE status <> 'failed';
CREATE INDEX automation_runs_rule_time_idx ON automation_runs (rule_id, created_at DESC);

ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
  CHECK (kind IN ('mention','assigned','status_changed','comment','shared','approval_requested','approval_decided','automation'));
