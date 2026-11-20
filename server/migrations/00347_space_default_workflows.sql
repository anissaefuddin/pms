-- +goose Up
-- Space-level default workflow per base type (B): Task and Project. Root items use it when neither the request
-- nor their Item Type names a workflow. Additive; existing items and statuses are not touched.
CREATE TABLE space_default_workflows (
  space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
  base_type text NOT NULL CHECK (base_type IN ('task', 'project')),
  workflow_id uuid NOT NULL REFERENCES workflows(id),
  set_by uuid REFERENCES users(id) ON DELETE SET NULL,
  set_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (space_id, base_type)
);
CREATE INDEX space_default_workflows_workflow_idx ON space_default_workflows (workflow_id);
