-- +goose Up
-- B3-01 ownership schema and B3-02 row-level invariants (docs/architecture/ADR-CLIENT-SCOPE-MVP.md, K-23, K-24, K-25).
-- Additive only: no existing column, row, ID, owner_id, item_locations row, grant, or custom-field value changes,
-- except that Personal Spaces get scope_state = 'internal' (K-25, not inferred from a name).
-- Ownership is a data scope. It is NOT an authorization: no grant, share, role, or policy decision reads these columns yet.

-- Ownership columns. Existing rows default to 'unresolved', so no client is ever guessed for them.
ALTER TABLE spaces
    ADD COLUMN client_id uuid REFERENCES clients(id) ON DELETE RESTRICT,
    ADD COLUMN scope_state text NOT NULL DEFAULT 'unresolved';
ALTER TABLE items
    ADD COLUMN client_id uuid REFERENCES clients(id) ON DELETE RESTRICT,
    ADD COLUMN scope_state text NOT NULL DEFAULT 'unresolved';

-- Classify existing Personal Spaces. The updated_at trigger is paused so this data step does not change timestamps.
ALTER TABLE spaces DISABLE TRIGGER spaces_set_updated_at;
UPDATE spaces SET scope_state = 'internal' WHERE kind = 'personal';
ALTER TABLE spaces ENABLE TRIGGER spaces_set_updated_at;

-- Row-level invariants. Each one is evaluated without NULL pitfalls: scope_state is NOT NULL, and the
-- client_id tests are IS NULL / IS NOT NULL, so every CHECK is either TRUE or FALSE, never UNKNOWN.
ALTER TABLE spaces
    ADD CONSTRAINT spaces_scope_state_valid CHECK (scope_state IN ('client', 'internal', 'unresolved')),
    ADD CONSTRAINT spaces_scope_matches_client CHECK (
        (scope_state = 'client' AND client_id IS NOT NULL)
        OR (scope_state IN ('internal', 'unresolved') AND client_id IS NULL)),
    ADD CONSTRAINT spaces_personal_is_internal CHECK (kind <> 'personal' OR scope_state = 'internal');
ALTER TABLE items
    ADD CONSTRAINT items_scope_state_valid CHECK (scope_state IN ('client', 'internal', 'unresolved')),
    ADD CONSTRAINT items_scope_matches_client CHECK (
        (scope_state = 'client' AND client_id IS NOT NULL)
        OR (scope_state IN ('internal', 'unresolved') AND client_id IS NULL));

-- Personal Spaces are private and internal by definition (K-25). Existing code creates them with
-- INSERT ... kind = 'personal' and no ownership fields, so this trigger sets the state. It never sets client-owned.
-- +goose StatementBegin
CREATE FUNCTION spaces_personal_scope() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.kind = 'personal' THEN
        NEW.scope_state := 'internal';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER spaces_personal_scope_internal BEFORE INSERT OR UPDATE OF kind, scope_state ON spaces
    FOR EACH ROW EXECUTE FUNCTION spaces_personal_scope();

-- Indexes for foreign-key checks. A Customer delete is RESTRICT and must find referencing rows, and future
-- client-scoped lookups filter on client_id. Both are plain indexes, following client_memberships_client_idx.
CREATE INDEX spaces_client_idx ON spaces (client_id);
CREATE INDEX items_client_idx ON items (client_id);

-- +goose Down
DROP INDEX items_client_idx;
DROP INDEX spaces_client_idx;
DROP TRIGGER spaces_personal_scope_internal ON spaces;
DROP FUNCTION spaces_personal_scope();
ALTER TABLE items
    DROP CONSTRAINT items_scope_matches_client,
    DROP CONSTRAINT items_scope_state_valid,
    DROP COLUMN scope_state,
    DROP COLUMN client_id;
ALTER TABLE spaces
    DROP CONSTRAINT spaces_personal_is_internal,
    DROP CONSTRAINT spaces_scope_matches_client,
    DROP CONSTRAINT spaces_scope_state_valid,
    DROP COLUMN scope_state,
    DROP COLUMN client_id;
