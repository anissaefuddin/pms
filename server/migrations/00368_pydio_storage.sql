-- +goose Up
-- Pydio Cells as file storage. No row or enabled = false: new files go to PMS_STORAGE. The row is kept when storage is turned off,
-- because files already in Pydio are still read with it. token_enc is encrypted with PMS_CONNECTOR_KEY.
CREATE TABLE pydio_storage (
    id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled boolean NOT NULL,
    base_url text NOT NULL CHECK (base_url LIKE 'https://%' AND char_length(base_url) <= 500),
    root text NOT NULL,
    token_enc text NOT NULL,
    -- {"<customer code>": "<workspace>/<path>"}
    customer_roots jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(customer_roots) = 'object' AND pg_column_size(customer_roots) <= 262144),
    -- Text field whose value names a project's folder; without it the folder is named by the project number.
    code_field_id uuid REFERENCES custom_fields(id) ON DELETE SET NULL,
    version integer NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid REFERENCES users(id) ON DELETE SET NULL
);
-- An administrator's folder for one project, instead of the generated one.
CREATE TABLE pydio_project_folders (
    item_id uuid PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
    folder text NOT NULL UNIQUE,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid REFERENCES users(id) ON DELETE SET NULL
);

-- +goose Down
DROP TABLE pydio_project_folders;
DROP TABLE pydio_storage;
