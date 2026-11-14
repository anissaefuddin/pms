-- +goose Up
-- Locked spaces (ENT-05): content stays inside the people who already have the space. See D-95.
ALTER TABLE spaces ADD COLUMN locked boolean NOT NULL DEFAULT false;
CREATE INDEX spaces_locked_idx ON spaces (id) WHERE locked;

-- Account-wide access policy (ENT-05): who may reach the application from where. One row.
CREATE TABLE access_policy (
    id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    ip_allowlist_enabled boolean NOT NULL DEFAULT false,
    ip_allowlist text[] NOT NULL DEFAULT '{}' CHECK (cardinality(ip_allowlist) <= 50),
    version integer NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid REFERENCES users(id) ON DELETE SET NULL
);
INSERT INTO access_policy (id) VALUES (1);
