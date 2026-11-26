-- +goose Up
-- Native Customer Master (B2-01) and client eligibility (B2-02), docs/architecture/ADR-CLIENT-SCOPE-MVP.md OWN-01/02.
-- Additive only: no existing table, column, or row changes. A client is a data scope, not a tenant (D-01).
-- Membership is eligibility, never a resource permission: it grants no Full/Editor/Read/Limited, no Space access, no share.

CREATE TABLE clients (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Canonical code: trimmed and uppercase, unique, never changed after creation (trigger below).
    -- Blank means no non-whitespace character. btrim() only strips spaces, so whitespace classes are checked by regex.
    code text NOT NULL UNIQUE CHECK (code ~ '^[^[:space:]]' AND code ~ '[^[:space:]]$' AND code = upper(code)),
    name text NOT NULL CHECK (name ~ '[^[:space:]]'),
    short_name text CHECK (short_name IS NULL OR short_name ~ '[^[:space:]]'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    archived_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    -- Archived exactly when archived_at is set. Archiving keeps the row and its history (Archive, not delete).
    CONSTRAINT clients_archive_matches_status CHECK ((status = 'archived') = (archived_at IS NOT NULL))
);

-- +goose StatementBegin
CREATE FUNCTION clients_code_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.code IS DISTINCT FROM OLD.code THEN
        RAISE EXCEPTION 'client code % is permanent and cannot change', OLD.code USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER clients_code_no_change BEFORE UPDATE OF code ON clients
    FOR EACH ROW EXECUTE FUNCTION clients_code_immutable();

-- Membership history: one row per grant. Revoking sets status and revoked_at; granting again inserts a new row.
-- Rows are never deleted, so the full grant and revoke history survives without relying on audit_events.
CREATE TABLE client_memberships (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id uuid NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    granted_at timestamptz NOT NULL DEFAULT now(),
    granted_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    revoked_at timestamptz,
    revoked_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT client_memberships_revoke_matches_status CHECK ((status = 'revoked') = (revoked_at IS NOT NULL)),
    CONSTRAINT client_memberships_revoke_after_grant CHECK (revoked_at IS NULL OR revoked_at >= granted_at)
);

-- At most one active membership per client and user. Revoked rows are kept as history.
CREATE UNIQUE INDEX client_memberships_one_active_idx ON client_memberships (client_id, user_id) WHERE status = 'active';
-- Eligible clients for one user (picker and listings).
CREATE INDEX client_memberships_active_user_idx ON client_memberships (user_id) WHERE status = 'active';
-- Supports the ON DELETE RESTRICT check on clients, which must scan revoked rows too.
CREATE INDEX client_memberships_client_idx ON client_memberships (client_id);

-- +goose Down
DROP TABLE client_memberships;
DROP TRIGGER clients_code_no_change ON clients;
DROP TABLE clients;
DROP FUNCTION clients_code_immutable();
