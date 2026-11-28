-- +goose Up
-- Pydio folder links configured by an administrator. At most one row. No row = the PMS_PYDIO_* environment applies; a row replaces
-- it as a whole, also when enabled is false. No credential is stored here.
CREATE TABLE pydio_settings (
    id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled boolean NOT NULL,
    base_url text NOT NULL CHECK (base_url LIKE 'https://%' AND char_length(base_url) <= 500),
    roots text[] NOT NULL CHECK (cardinality(roots) BETWEEN 1 AND 50),
    -- {"<customer code>": ["<workspace>/<path>", ...]}
    customer_roots jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(customer_roots) = 'object' AND pg_column_size(customer_roots) <= 262144),
    version integer NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid REFERENCES users(id) ON DELETE SET NULL
);

-- +goose Down
DROP TABLE pydio_settings;
