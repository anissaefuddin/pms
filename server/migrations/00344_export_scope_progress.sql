-- +goose Up
-- Additive (EXP-03, EXP-09). A Space-scope export has no Project, so project_id becomes nullable and a scope says which
-- target is set. Existing rows keep scope 'project' and their project_id. Progress counts real items: total_items is the
-- number of items in the export, processed_items how many the worker has written so far.
ALTER TABLE project_excel_exports ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE project_excel_exports ADD COLUMN scope text NOT NULL DEFAULT 'project' CHECK (scope IN ('project', 'space'));
ALTER TABLE project_excel_exports ADD CONSTRAINT project_excel_exports_scope_target
    CHECK ((scope = 'project' AND project_id IS NOT NULL) OR (scope = 'space' AND project_id IS NULL));
ALTER TABLE project_excel_exports ADD COLUMN total_items integer NOT NULL DEFAULT 0 CHECK (total_items >= 0);
ALTER TABLE project_excel_exports ADD COLUMN processed_items integer NOT NULL DEFAULT 0 CHECK (processed_items >= 0);

-- +goose Down
-- Space-scope exports cannot be represented without project_id, so they are removed first (they expire within an hour anyway).
DELETE FROM project_excel_exports WHERE scope = 'space';
ALTER TABLE project_excel_exports DROP CONSTRAINT project_excel_exports_scope_target;
ALTER TABLE project_excel_exports DROP COLUMN processed_items;
ALTER TABLE project_excel_exports DROP COLUMN total_items;
ALTER TABLE project_excel_exports DROP COLUMN scope;
ALTER TABLE project_excel_exports ALTER COLUMN project_id SET NOT NULL;
