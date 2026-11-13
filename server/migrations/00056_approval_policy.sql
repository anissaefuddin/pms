-- +goose Up
ALTER TABLE approvals ADD COLUMN policy text NOT NULL DEFAULT 'all' CHECK (policy IN ('any','all'));

-- +goose Down
ALTER TABLE approvals DROP COLUMN policy;
