-- +goose Up
ALTER TABLE users ADD COLUMN sidebar_hidden text[] NOT NULL DEFAULT '{}';
