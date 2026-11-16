-- +goose Up
-- RF-06: a form is a draft until it is published; only published forms reach requesters and public links.
-- Forms that already exist were live, so they are published as of their creation. The trigger is off for the
-- backfill so that updated_at keeps its value.
ALTER TABLE request_forms ADD COLUMN published_at timestamptz;
ALTER TABLE request_forms DISABLE TRIGGER request_forms_set_updated_at;
UPDATE request_forms SET published_at = created_at;
ALTER TABLE request_forms ENABLE TRIGGER request_forms_set_updated_at;

-- +goose Down
ALTER TABLE request_forms DROP COLUMN published_at;
