-- +goose Up
-- Public dashboard link (DASH-07): one link per dashboard, stored as a token hash, with an expiry and an optional password.
-- The link is viewed with the rights of the person who made it (created_by); if that person is gone, so is the link.
CREATE TABLE dashboard_public_links (
    dashboard_id uuid PRIMARY KEY REFERENCES dashboards(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    password_hash text,
    expires_at timestamptz NOT NULL,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE dashboard_public_links;
