-- +goose Up
-- Custom roles (ENT-03): a named set of the eight capabilities that can be used in a share next to read, limited, editor and full.
-- caps_mask holds one bit per capability in policy.AllCapabilities order (bit 0 = view); the order is append-only. Every role must
-- include view, otherwise a user would hold rights on something they cannot see.
CREATE TABLE custom_roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 60),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 300),
    caps_mask integer NOT NULL CHECK (caps_mask BETWEEN 1 AND 255 AND caps_mask & 1 = 1),
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX custom_roles_name_idx ON custom_roles (lower(btrim(name)));

-- A share with a custom role keeps role = 'read' as a fail-safe: any reader that ignores custom_role_id sees "read", never more.
ALTER TABLE shares ADD COLUMN custom_role_id uuid REFERENCES custom_roles(id) ON DELETE RESTRICT;
ALTER TABLE shares ADD CONSTRAINT shares_custom_role_fallback CHECK (custom_role_id IS NULL OR role = 'read');
CREATE INDEX shares_custom_role_idx ON shares (custom_role_id) WHERE custom_role_id IS NOT NULL;
