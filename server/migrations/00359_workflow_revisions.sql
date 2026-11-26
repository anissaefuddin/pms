-- +goose Up
-- Which workflow rules a status change was made under (DEF-018). A revision is a frozen copy of a workflow's statuses, guards and
-- transitions, written the first time a change happens under that definition. Changes made before this have none.
CREATE TABLE workflow_revisions (
  workflow_id uuid NOT NULL,
  revision integer NOT NULL,
  definition jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workflow_id, revision)
);
ALTER TABLE activity
  ADD COLUMN workflow_id uuid,
  ADD COLUMN workflow_revision integer,
  ADD COLUMN request_id text NOT NULL DEFAULT '';

-- +goose StatementBegin
CREATE FUNCTION activity_workflow_revision() RETURNS trigger LANGUAGE plpgsql AS $$
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
  PERFORM pg_advisory_xact_lock(hashtextextended(flow::text, 359));
  SELECT revision, definition INTO last FROM workflow_revisions WHERE workflow_id = flow ORDER BY revision DESC LIMIT 1;
  IF last.revision IS NULL OR last.definition IS DISTINCT FROM body THEN
    INSERT INTO workflow_revisions(workflow_id, revision, definition) VALUES (flow, COALESCE(last.revision, 0) + 1, body);
    NEW.workflow_revision := COALESCE(last.revision, 0) + 1;
  ELSE
    NEW.workflow_revision := last.revision;
  END IF;
  NEW.workflow_id := flow;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER activity_workflow_revision BEFORE INSERT ON activity FOR EACH ROW
  WHEN (NEW.changes ? 'status_id') EXECUTE FUNCTION activity_workflow_revision();

-- +goose Down
DROP TRIGGER activity_workflow_revision ON activity;
DROP FUNCTION activity_workflow_revision();
ALTER TABLE activity DROP COLUMN workflow_id, DROP COLUMN workflow_revision, DROP COLUMN request_id;
DROP TABLE workflow_revisions;
