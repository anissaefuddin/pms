-- +goose Up
-- Report schedule (RPT-04): any weekday or every day (weekday NULL), at an hour in the time zone the subscriber picks
-- (timezone NULL follows their profile).
ALTER TABLE report_subscriptions ALTER COLUMN weekday DROP NOT NULL;
ALTER TABLE report_subscriptions ADD COLUMN timezone text;

-- +goose Down
DELETE FROM report_subscriptions WHERE weekday IS NULL;
ALTER TABLE report_subscriptions DROP COLUMN timezone;
ALTER TABLE report_subscriptions ALTER COLUMN weekday SET NOT NULL;
