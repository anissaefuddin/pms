-- +goose Up
-- Workspace automation (AUT-02, design approved). Existing rules stay Space rules; nothing about them changes.

-- M1: a rule is a Space rule (space_id set) or a Workspace rule (space_id NULL, targets listed).
ALTER TABLE automation_rules
    ADD COLUMN scope text NOT NULL DEFAULT 'space' CHECK (scope IN ('space', 'workspace')),
    ADD COLUMN target_space_ids uuid[] NOT NULL DEFAULT '{}';

-- M2: space_id is optional only for workspace rules.
ALTER TABLE automation_rules ALTER COLUMN space_id DROP NOT NULL;
ALTER TABLE automation_rules ADD CONSTRAINT automation_rules_scope_space
    CHECK ((scope = 'space') = (space_id IS NOT NULL));

-- M3: configurable limits (decision D3). Changes are audited by the application.
CREATE TABLE automation_limits (
    id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    rule_per_hour integer NOT NULL DEFAULT 50 CHECK (rule_per_hour BETWEEN 1 AND 100000),
    workspace_per_hour integer NOT NULL DEFAULT 500 CHECK (workspace_per_hour BETWEEN 1 AND 1000000),
    space_per_hour integer NOT NULL DEFAULT 500 CHECK (space_per_hour BETWEEN 1 AND 1000000),
    updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO automation_limits (id) VALUES (1);

-- Targets are looked up by overlap (&&) when a run looks for rules.
CREATE INDEX automation_rules_workspace_targets_idx ON automation_rules USING gin (target_space_ids);

-- Runs remember the Space they acted in, so a Space's hourly limit counts workspace rules too.
ALTER TABLE automation_runs ADD COLUMN space_id uuid REFERENCES spaces(id) ON DELETE CASCADE;
UPDATE automation_runs a SET space_id = r.space_id FROM automation_rules r WHERE r.id = a.rule_id AND r.space_id IS NOT NULL;
CREATE INDEX automation_runs_space_time_idx ON automation_runs (space_id, created_at) WHERE space_id IS NOT NULL;

-- +goose Down
DROP INDEX automation_runs_space_time_idx;
ALTER TABLE automation_runs DROP COLUMN space_id;
DROP INDEX automation_rules_workspace_targets_idx;
DROP TABLE automation_limits;
DELETE FROM automation_rules WHERE scope = 'workspace';
ALTER TABLE automation_rules DROP CONSTRAINT automation_rules_scope_space;
ALTER TABLE automation_rules ALTER COLUMN space_id SET NOT NULL;
ALTER TABLE automation_rules DROP COLUMN target_space_ids, DROP COLUMN scope;
