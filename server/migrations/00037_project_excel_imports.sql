-- +goose Up
CREATE TABLE project_excel_imports (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    digest text NOT NULL CHECK (length(digest)=64),
    item_ids uuid[] NOT NULL,
    expires_at timestamptz NOT NULL DEFAULT now() + interval '1 hour',
    PRIMARY KEY(user_id,space_id,digest)
);
CREATE INDEX project_excel_imports_expiry_idx ON project_excel_imports(expires_at);
