-- +goose Up
-- Color theme (SET-06): the per-user accent; light, dark and system stay in users.theme.
ALTER TABLE users ADD COLUMN accent text NOT NULL DEFAULT 'blue' CHECK (accent IN ('blue', 'teal', 'violet', 'green'));
