-- +goose Up
-- Body and open/closed state join the title in the sync snapshot; Google Calendar keeps its event fingerprint in synced_remote_version.
ALTER TABLE connector_links ADD COLUMN synced_body text, ADD COLUMN synced_done boolean;
ALTER TABLE connector_conflicts ADD COLUMN differs text NOT NULL DEFAULT 'title';
-- Scheduling and status: next_sync_at drives the polling job (a webhook sets it to now(), a failure pushes it out).
ALTER TABLE connector_installs
    ADD COLUMN next_sync_at timestamptz,
    ADD COLUMN failures integer NOT NULL DEFAULT 0,
    ADD COLUMN last_sync_at timestamptz,
    ADD COLUMN last_sync_ok boolean,
    ADD COLUMN last_sync_message text;

-- +goose Down
ALTER TABLE connector_installs DROP COLUMN next_sync_at, DROP COLUMN failures, DROP COLUMN last_sync_at, DROP COLUMN last_sync_ok, DROP COLUMN last_sync_message;
ALTER TABLE connector_conflicts DROP COLUMN differs;
ALTER TABLE connector_links DROP COLUMN synced_body, DROP COLUMN synced_done;
