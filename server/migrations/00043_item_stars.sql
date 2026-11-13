-- +goose Up
-- A personal bookmark (NAV-02 Starred, DASH-02 starred widget). Rows survive losing access but are filtered by policy when read.
CREATE TABLE item_stars (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, item_id)
);
CREATE INDEX item_stars_item_idx ON item_stars(item_id);
