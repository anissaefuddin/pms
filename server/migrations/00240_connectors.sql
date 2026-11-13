-- +goose Up
CREATE TABLE connector_installs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    config jsonb NOT NULL DEFAULT '{}',
    secrets_enc text,
    enabled boolean NOT NULL DEFAULT true,
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- One row per Item <-> external record. synced_* is the state both sides had at the last sync; NULL title = never synced.
CREATE TABLE connector_links (
    connector_id uuid NOT NULL REFERENCES connector_installs(id) ON DELETE CASCADE,
    external_id text NOT NULL,
    item_id uuid NOT NULL REFERENCES items(id),
    synced_title text,
    synced_remote_version text NOT NULL DEFAULT '',
    synced_item_version integer,
    synced_at timestamptz,
    PRIMARY KEY (connector_id, external_id),
    UNIQUE (connector_id, item_id)
);

CREATE TABLE connector_conflicts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connector_id uuid NOT NULL REFERENCES connector_installs(id) ON DELETE CASCADE,
    external_id text NOT NULL,
    item_id uuid NOT NULL REFERENCES items(id),
    item_title text NOT NULL,
    remote_title text NOT NULL,
    remote_version text NOT NULL,
    base_title text,
    resolution text CHECK (resolution IN ('item', 'remote')),
    resolved_by uuid REFERENCES users(id),
    resolved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
-- At most one open conflict per link: re-running a sync does not pile up duplicates.
CREATE UNIQUE INDEX connector_conflicts_open_idx ON connector_conflicts (connector_id, external_id) WHERE resolution IS NULL;

-- +goose Down
DROP TABLE connector_conflicts;
DROP TABLE connector_links;
DROP TABLE connector_installs;
