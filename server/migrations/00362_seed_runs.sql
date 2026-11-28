-- +goose Up
-- Opt-in starter configuration (PMS_SEED) runs once per instance. A row means the seed ran, so what an admin later
-- renames, archives or re-grants is never imposed again.
CREATE TABLE seed_runs (
    name text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE seed_runs;
