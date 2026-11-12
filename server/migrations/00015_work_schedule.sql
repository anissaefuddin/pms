-- +goose Up
-- One schedule per workspace (D-39). The default row is configuration, not demo data, so it lives here
-- and not in `pms bootstrap`: every install needs it before the first item is scheduled.
CREATE TABLE work_schedules (
    id           smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    working_days smallint[] NOT NULL DEFAULT '{1,2,3,4,5}' CHECK (
        cardinality(working_days) BETWEEN 1 AND 7 AND working_days <@ ARRAY[0,1,2,3,4,5,6]::smallint[]),
    daily_hours  smallint NOT NULL DEFAULT 8 CHECK (daily_hours BETWEEN 1 AND 24),
    version      integer NOT NULL DEFAULT 1,
    updated_at   timestamptz NOT NULL DEFAULT now()
);
INSERT INTO work_schedules DEFAULT VALUES;

CREATE TABLE work_schedule_exceptions (
    day  date PRIMARY KEY,
    name text NOT NULL CHECK (char_length(name) <= 120)
);
