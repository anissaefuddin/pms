-- +goose Up
-- 00359 took a per-workflow lock on every status change and kept it until commit, so one import queued every other change in that
-- workflow. The lock is now taken only when the definition differs from the latest revision and a new one has to be written.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION activity_workflow_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  flow uuid;
  body jsonb;
  last record;
BEGIN
  SELECT workflow_id INTO flow FROM statuses WHERE id::text = NEW.changes->'status_id'->>'to';
  IF flow IS NULL THEN
    RETURN NEW;
  END IF;
  SELECT jsonb_build_object(
    'statuses', (SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id) FROM statuses s WHERE s.workflow_id = flow),
    'transitions', (SELECT COALESCE(jsonb_agg(jsonb_build_array(t.from_status_id, t.to_status_id) ORDER BY t.from_status_id, t.to_status_id), '[]'::jsonb)
                    FROM status_transitions t JOIN statuses s ON s.id = t.from_status_id WHERE s.workflow_id = flow)) INTO body;
  SELECT revision, definition INTO last FROM workflow_revisions WHERE workflow_id = flow ORDER BY revision DESC LIMIT 1;
  IF last.revision IS NULL OR last.definition IS DISTINCT FROM body THEN
    PERFORM pg_advisory_xact_lock(hashtextextended(flow::text, 359));
    SELECT revision, definition INTO last FROM workflow_revisions WHERE workflow_id = flow ORDER BY revision DESC LIMIT 1;
    IF last.revision IS NULL OR last.definition IS DISTINCT FROM body THEN
      -- A serializable writer that cannot see the row just committed gets a serialization failure here and is retried.
      INSERT INTO workflow_revisions(workflow_id, revision, definition) VALUES (flow, COALESCE(last.revision, 0) + 1, body)
        ON CONFLICT DO NOTHING;
      last.revision := COALESCE(last.revision, 0) + 1;
    END IF;
  END IF;
  NEW.workflow_revision := last.revision;
  NEW.workflow_id := flow;
  RETURN NEW;
END $$;
-- +goose StatementEnd

-- +goose Down
-- The earlier function is not restored: it differs only in how long it holds the lock.
SELECT 1;
