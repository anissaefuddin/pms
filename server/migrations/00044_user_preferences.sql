-- +goose Up
-- SET-03 default values for new tasks (null: none) and SET-05 email groups the user turned off (empty: all on).
ALTER TABLE users
    ADD COLUMN default_task_duration_days integer CHECK (default_task_duration_days BETWEEN 0 AND 3650),
    ADD COLUMN default_task_effort_minutes integer CHECK (default_task_effort_minutes BETWEEN 0 AND 525600),
    ADD COLUMN email_muted text[] NOT NULL DEFAULT '{}';
