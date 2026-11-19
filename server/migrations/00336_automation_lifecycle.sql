-- +goose Up
-- Automation lifecycle (A1): a rule can be paused (its owner cannot run it any more) or invalid (something it depends on is gone),
-- and it can be archived instead of deleted. Each state keeps a reason code so the rule can say why it does not run.
ALTER TABLE automation_rules
    ADD COLUMN health text NOT NULL DEFAULT 'ok' CHECK (health IN ('ok', 'paused', 'invalid')),
    ADD COLUMN health_reason text NOT NULL DEFAULT '' CHECK (char_length(health_reason) <= 60),
    ADD COLUMN archived_at timestamptz;

-- "stopped" is the rule-level outcome of an action refused by policy: the actions before it stay done, the ones after it do not run.
ALTER TABLE automation_runs DROP CONSTRAINT automation_runs_status_check;
ALTER TABLE automation_runs ADD CONSTRAINT automation_runs_status_check
    CHECK (status IN ('done', 'denied', 'skipped', 'loop_blocked', 'quota_blocked', 'failed', 'stopped'));

-- +goose Down
ALTER TABLE automation_runs DROP CONSTRAINT automation_runs_status_check;
ALTER TABLE automation_runs ADD CONSTRAINT automation_runs_status_check
    CHECK (status IN ('done', 'denied', 'skipped', 'loop_blocked', 'quota_blocked', 'failed'));
ALTER TABLE automation_rules DROP COLUMN archived_at, DROP COLUMN health_reason, DROP COLUMN health;
