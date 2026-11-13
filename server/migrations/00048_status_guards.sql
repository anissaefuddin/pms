-- +goose Up
-- Transition guards (WF-04): entering a status can require fields to be filled and restrict who may do it.
ALTER TABLE statuses
    ADD COLUMN required_fields text[] NOT NULL DEFAULT '{}',
    ADD COLUMN actor_rule text NOT NULL DEFAULT 'anyone' CHECK (actor_rule IN ('anyone', 'assignee', 'admin'));
