-- +goose Up
-- The workspace's time zone (IANA name). Automation evaluates deadlines on the local calendar day in this zone; UTC when it is not set.
ALTER TABLE work_schedules ADD COLUMN timezone text NOT NULL DEFAULT 'UTC' CHECK (char_length(timezone) BETWEEN 1 AND 64);

-- +goose Down
ALTER TABLE work_schedules DROP COLUMN timezone;
