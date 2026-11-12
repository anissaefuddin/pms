-- +goose Up
ALTER TABLE items ADD COLUMN importance text NOT NULL DEFAULT 'normal' CHECK (importance IN ('high', 'normal', 'low'));
