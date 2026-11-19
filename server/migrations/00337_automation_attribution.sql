-- +goose Up
-- Automation attribution (A1 decision): a change a rule makes is recorded as made by the Automation Bot on the rule's behalf.
-- Permissions still follow the rule owner; actor_id keeps the owner, so the history shows who the rule acted for.
ALTER TABLE activity
    ADD COLUMN via text NOT NULL DEFAULT '' CHECK (via IN ('', 'automation_bot')),
    ADD COLUMN rule_id uuid REFERENCES automation_rules(id) ON DELETE SET NULL;
ALTER TABLE comments
    ADD COLUMN via text NOT NULL DEFAULT '' CHECK (via IN ('', 'automation_bot')),
    ADD COLUMN rule_id uuid REFERENCES automation_rules(id) ON DELETE SET NULL;

-- Why a run line is not "done": a stable code (forbidden, not_found, field_invalid, guard, invalid, owner_inactive, ...).
ALTER TABLE automation_runs ADD COLUMN reason text NOT NULL DEFAULT '' CHECK (char_length(reason) <= 60);

-- +goose Down
ALTER TABLE automation_runs DROP COLUMN reason;
ALTER TABLE comments DROP COLUMN rule_id, DROP COLUMN via;
ALTER TABLE activity DROP COLUMN rule_id, DROP COLUMN via;
