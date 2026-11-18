-- +goose Up
-- Import report (decision 14 and blocker 3): kept after the one-hour receipt expires, so the importer can read what happened.
CREATE TABLE excel_import_reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    report jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX excel_import_reports_user_idx ON excel_import_reports (user_id, space_id, created_at);

-- +goose Down
DROP TABLE excel_import_reports;
