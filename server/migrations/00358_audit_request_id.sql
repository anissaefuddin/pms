-- +goose Up
-- The request behind an audit event, the same id the access and application logs carry. Older events and background jobs have none.
ALTER TABLE audit_events ADD COLUMN request_id text NOT NULL DEFAULT '' CHECK (char_length(request_id) <= 64);
CREATE INDEX audit_events_request_idx ON audit_events (request_id) WHERE request_id <> '';

-- +goose Down
ALTER TABLE audit_events DROP COLUMN request_id;
