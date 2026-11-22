-- +goose Up
-- Dashboards become the first default pin. Only users still on the old default are moved; anyone who changed their pins keeps them.
ALTER TABLE users ALTER COLUMN sidebar_pinned SET DEFAULT '{dashboards,request_forms,stream}';
UPDATE users SET sidebar_pinned = '{dashboards,request_forms,stream}' WHERE sidebar_pinned = '{request_forms,stream}';

-- +goose Down
ALTER TABLE users ALTER COLUMN sidebar_pinned SET DEFAULT '{request_forms,stream}';
UPDATE users SET sidebar_pinned = '{request_forms,stream}' WHERE sidebar_pinned = '{dashboards,request_forms,stream}';
