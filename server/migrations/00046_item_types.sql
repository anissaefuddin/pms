-- +goose Up
-- Custom item types (CUST-01..03, ADR-CIT). Behaviour stays one of three classes (`base`); a custom type only adds a name,
-- an icon, allowed children, default values and a default workflow. items.type keeps the class so old code stays correct.
CREATE TABLE item_type_defs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key text NOT NULL UNIQUE CHECK (key ~ '^[a-z][a-z0-9_]{1,39}$'),
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 80),
    icon text NOT NULL DEFAULT 'circle' CHECK (icon ~ '^[a-z0-9-]{1,40}$'),
    base item_type NOT NULL,
    is_builtin boolean NOT NULL DEFAULT false,
    active boolean NOT NULL DEFAULT true,
    allowed_children uuid[] NOT NULL DEFAULT '{}',
    default_workflow_id uuid REFERENCES workflows(id),
    defaults jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(defaults) = 'object'),
    layout jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(layout) = 'object'),
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (NOT is_builtin OR key = base::text)
);
CREATE UNIQUE INDEX item_type_defs_name_idx ON item_type_defs (lower(name));
INSERT INTO item_type_defs(key,name,icon,base,is_builtin) VALUES
    ('task','Task','check-square','task',true),('project','Project','folder-kanban','project',true),('folder','Folder','folder','folder',true);
ALTER TABLE items ADD COLUMN type_id uuid REFERENCES item_type_defs(id);
UPDATE items i SET type_id = t.id FROM item_type_defs t WHERE t.is_builtin AND t.key = i.type::text;
-- Rows inserted without a type_id (older code paths, fixtures) get the built-in type of their class.
-- +goose StatementBegin
CREATE FUNCTION items_default_type() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.type_id IS NULL THEN
    SELECT id INTO NEW.type_id FROM item_type_defs WHERE is_builtin AND key = NEW.type::text;
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER items_default_type BEFORE INSERT ON items FOR EACH ROW EXECUTE FUNCTION items_default_type();
ALTER TABLE items ALTER COLUMN type_id SET NOT NULL;
CREATE INDEX items_type_id_idx ON items (type_id) WHERE deleted_at IS NULL;
