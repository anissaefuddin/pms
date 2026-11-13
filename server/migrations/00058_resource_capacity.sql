-- +goose Up
CREATE TABLE resource_schedules (
 user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 working_days smallint[], daily_hours smallint CHECK (daily_hours BETWEEN 1 AND 24),
 leave_dates date[] NOT NULL DEFAULT '{}',
 CHECK ((working_days IS NULL) = (daily_hours IS NULL))
);
CREATE INDEX items_workload_dates_idx ON items (due_date, start_date) WHERE deleted_at IS NULL AND type='task';

-- +goose Down
DROP INDEX items_workload_dates_idx;
DROP TABLE resource_schedules;
