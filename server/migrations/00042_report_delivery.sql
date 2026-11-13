-- +goose Up
-- The delivery job claims a subscription by stamping this; it sends at most once per slot (weekly, so a 23 hour guard is enough).
ALTER TABLE report_subscriptions ADD COLUMN last_sent_at timestamptz;
