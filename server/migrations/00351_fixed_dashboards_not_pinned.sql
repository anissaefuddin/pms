-- +goose Up
-- Dashboards now sit above Inbox and Search at all times, so it is no longer a pin. Users on the old default lose the pin; custom pins stay.
ALTER TABLE users ALTER COLUMN sidebar_pinned SET DEFAULT '{request_forms,stream}';
UPDATE users SET sidebar_pinned = '{request_forms,stream}' WHERE sidebar_pinned = '{dashboards,request_forms,stream}';

-- +goose Down
ALTER TABLE users ALTER COLUMN sidebar_pinned SET DEFAULT '{dashboards,request_forms,stream}';
UPDATE users SET sidebar_pinned = '{dashboards,request_forms,stream}' WHERE sidebar_pinned = '{request_forms,stream}';
