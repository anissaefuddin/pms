-- +goose Up
CREATE TABLE time_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id),
    entry_date date NOT NULL CHECK (entry_date BETWEEN DATE '0001-01-01' AND DATE '9999-12-31'),
    minutes integer NOT NULL CHECK (minutes BETWEEN 1 AND 1440),
    category text NOT NULL DEFAULT '' CHECK (char_length(category) <= 80),
    comment text NOT NULL DEFAULT '' CHECK (char_length(comment) <= 2000),
    billing_type text NOT NULL DEFAULT 'non_billable' CHECK (billing_type IN ('billable','non_billable')),
    locked boolean NOT NULL DEFAULT false,
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (user_id, item_id, entry_date)
);
CREATE INDEX time_entries_user_date_idx ON time_entries(user_id,entry_date,id);
CREATE INDEX time_entries_item_date_idx ON time_entries(item_id,entry_date,id);
