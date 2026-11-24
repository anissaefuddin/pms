-- +goose Up
-- Workspace branding is stored with the database so normal backups include every uploaded asset.
CREATE TABLE workspace_brand_assets (
  kind text PRIMARY KEY CHECK (kind IN ('icon', 'logo_light', 'logo_dark')),
  content_type text NOT NULL CHECK (content_type IN ('image/png', 'image/jpeg', 'image/webp')),
  data bytea NOT NULL CHECK (octet_length(data) BETWEEN 1 AND 2097152),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE workspace_brand_assets;
