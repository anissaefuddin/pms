-- +goose Up
CREATE TABLE space_bookmarks (
    id uuid PRIMARY KEY,
    space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    url text NOT NULL CHECK (char_length(url) BETWEEN 1 AND 2000 AND url ~* '^https?://'),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 1000),
    position integer NOT NULL DEFAULT 0,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX space_bookmarks_space_idx ON space_bookmarks (space_id, position, created_at);

-- D-NAV-09: feedback lives in this table only; admins read it through the API. No e-mail, IP or user agent is stored.
CREATE TABLE feedback (
    id uuid PRIMARY KEY,
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    subject text NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 200),
    rating smallint CHECK (rating BETWEEN 1 AND 5),
    comment text NOT NULL DEFAULT '' CHECK (char_length(comment) <= 4000),
    page text NOT NULL DEFAULT '' CHECK (char_length(page) <= 200),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX feedback_created_idx ON feedback (created_at DESC);

-- +goose Down
DROP TABLE feedback;
DROP TABLE space_bookmarks;
