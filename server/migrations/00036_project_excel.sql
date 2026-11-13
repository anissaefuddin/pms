-- +goose Up
CREATE TABLE project_excel_exports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL DEFAULT now() + interval '1 hour',
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','ready','failed')),
    error text,
    workbook bytea CHECK (octet_length(workbook) <= 5242880),
    item_ids uuid[],
    includes_fields boolean NOT NULL DEFAULT false,
    CHECK ((status = 'ready') = (workbook IS NOT NULL AND item_ids IS NOT NULL))
);
CREATE INDEX project_excel_exports_expiry_idx ON project_excel_exports(expires_at);
CREATE INDEX project_excel_exports_user_idx ON project_excel_exports(user_id,expires_at);

