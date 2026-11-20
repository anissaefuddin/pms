-- +goose Up
-- Additive (IMP Report Link). A notification may carry a typed reference to the record it is about, so the inbox does not
-- parse the snippet. kind and id are set together or not at all.
ALTER TABLE notifications ADD COLUMN reference_kind text CHECK (reference_kind IN ('excel_import_report'));
ALTER TABLE notifications ADD COLUMN reference_id uuid;
ALTER TABLE notifications ADD CONSTRAINT notifications_reference_pair CHECK ((reference_kind IS NULL) = (reference_id IS NULL));

-- +goose Down
-- Only the reference is dropped; the notifications themselves and their snippets stay.
ALTER TABLE notifications DROP CONSTRAINT notifications_reference_pair;
ALTER TABLE notifications DROP COLUMN reference_id;
ALTER TABLE notifications DROP COLUMN reference_kind;
