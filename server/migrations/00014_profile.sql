-- +goose Up
ALTER TABLE users
    ADD COLUMN company text,
    ADD COLUMN country text,
    ADD COLUMN location text,
    ADD COLUMN phone text,
    ADD COLUMN density text NOT NULL DEFAULT 'compact' CHECK (density IN ('compact', 'comfortable')),
    ADD COLUMN theme text NOT NULL DEFAULT 'system' CHECK (theme IN ('light', 'dark', 'system')),
    ADD COLUMN comment_shortcut text NOT NULL DEFAULT 'mod_enter' CHECK (comment_shortcut IN ('mod_enter', 'enter'));
