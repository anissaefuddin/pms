-- +goose Up
-- Gantt baselines (VIEW-05C): a frozen copy of the planned dates of a project's items. Immutable on purpose (E2E-PAR-08):
-- comparing against a baseline only means something if nobody can move it afterwards. No FKs so deleting an item never rewrites history.
CREATE TABLE baselines (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX baselines_project_idx ON baselines (project_id, created_at DESC, id);
CREATE TABLE baseline_items (
    baseline_id uuid NOT NULL,
    item_id uuid NOT NULL,
    start_date date,
    due_date date,
    PRIMARY KEY (baseline_id, item_id)
);

-- +goose StatementBegin
CREATE FUNCTION baselines_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'baselines are immutable' USING ERRCODE = 'restrict_violation';
END $$;
-- +goose StatementEnd
CREATE TRIGGER baselines_no_change BEFORE UPDATE OR DELETE ON baselines FOR EACH ROW EXECUTE FUNCTION baselines_immutable();
CREATE TRIGGER baseline_items_no_change BEFORE UPDATE OR DELETE ON baseline_items FOR EACH ROW EXECUTE FUNCTION baselines_immutable();
CREATE TRIGGER baselines_no_truncate BEFORE TRUNCATE ON baselines FOR EACH STATEMENT EXECUTE FUNCTION baselines_immutable();
CREATE TRIGGER baseline_items_no_truncate BEFORE TRUNCATE ON baseline_items FOR EACH STATEMENT EXECUTE FUNCTION baselines_immutable();

-- +goose Down
DROP TABLE baseline_items;
DROP TABLE baselines;
DROP FUNCTION baselines_immutable();
