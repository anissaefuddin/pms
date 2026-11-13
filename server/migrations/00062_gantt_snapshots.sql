-- +goose Up
CREATE TABLE gantt_snapshots (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    token_hash bytea NOT NULL UNIQUE,
    created_by uuid,
    show_assignees boolean NOT NULL DEFAULT false,
    expires_at timestamptz NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX gantt_snapshots_project_idx ON gantt_snapshots (project_id, created_at DESC);
CREATE INDEX gantt_snapshots_expiry_idx ON gantt_snapshots (expires_at);

-- +goose Down
DROP TABLE gantt_snapshots;
