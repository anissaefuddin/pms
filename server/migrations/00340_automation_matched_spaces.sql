-- +goose Up
-- Multi-target execution: a run records every target Space the item matched. The Space it executed in is space_id.
ALTER TABLE automation_runs ADD COLUMN matched_space_ids uuid[] NOT NULL DEFAULT '{}';
UPDATE automation_runs SET matched_space_ids = ARRAY[space_id] WHERE space_id IS NOT NULL;

-- +goose Down
ALTER TABLE automation_runs DROP COLUMN matched_space_ids;
