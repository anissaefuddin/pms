-- +goose Up
-- Scope lookups of the analytics engine start from a Space; status, assignee and time lookups already have indexes.
CREATE INDEX item_locations_space_item_idx ON item_locations (space_id, item_id) WHERE deleted_root_id IS NULL;
CREATE INDEX items_due_live_idx ON items (due_date) WHERE deleted_at IS NULL AND due_date IS NOT NULL;
