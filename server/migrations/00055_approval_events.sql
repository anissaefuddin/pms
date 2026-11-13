-- +goose Up
-- Append-only approval history (APR-04): who asked whom, who decided what and with which comment, pinned to the item version
-- the decision was made against. No foreign keys on purpose: deleting an approval or item must not erase or be blocked by its history.
CREATE TABLE approval_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    approval_id uuid NOT NULL,
    item_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('approver_requested', 'approver_removed', 'decided', 'withdrawn')),
    actor_id uuid,
    subject_id uuid,
    decision text CHECK (decision IN ('approved', 'rejected')),
    comment text NOT NULL DEFAULT '',
    -- Pin to the item version (file version pin follows with S11.1.5).
    item_version integer,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX approval_events_item_idx ON approval_events (item_id, id DESC);

-- +goose StatementBegin
CREATE FUNCTION approval_events_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'approval_events is append-only' USING ERRCODE = 'restrict_violation';
END $$;
-- +goose StatementEnd
CREATE TRIGGER approval_events_no_change BEFORE UPDATE OR DELETE ON approval_events
    FOR EACH ROW EXECUTE FUNCTION approval_events_immutable();
CREATE TRIGGER approval_events_no_truncate BEFORE TRUNCATE ON approval_events
    FOR EACH STATEMENT EXECUTE FUNCTION approval_events_immutable();

-- +goose Down
DROP TABLE approval_events;
DROP FUNCTION approval_events_immutable();
