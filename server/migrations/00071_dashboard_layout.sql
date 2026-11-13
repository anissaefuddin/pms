-- +goose Up
-- Dashboard layout orientation (DASH-05): widgets side by side in columns, or stacked one above the other.
ALTER TABLE dashboards ADD COLUMN layout text NOT NULL DEFAULT 'columns' CHECK (layout IN ('columns', 'stack'));

-- +goose Down
ALTER TABLE dashboards DROP COLUMN layout;
