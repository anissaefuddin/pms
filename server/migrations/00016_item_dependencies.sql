-- +goose Up
CREATE TABLE item_dependencies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    predecessor_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    successor_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    type text NOT NULL CHECK (type IN ('FS', 'SS', 'FF', 'SF')),
    lag integer NOT NULL DEFAULT 0,
    UNIQUE (predecessor_id, successor_id),
    CHECK (predecessor_id <> successor_id)
);
CREATE INDEX item_dependencies_successor_idx ON item_dependencies (successor_id);
