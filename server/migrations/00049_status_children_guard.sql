-- +goose Up
-- Cross-record validation (WF-05): a status can require every subitem to be completed first.
ALTER TABLE statuses ADD COLUMN require_children_done boolean NOT NULL DEFAULT false;
