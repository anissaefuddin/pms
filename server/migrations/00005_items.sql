-- +goose Up
CREATE TYPE item_type AS ENUM ('project', 'folder', 'task');

CREATE TABLE items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  seq bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
  type item_type NOT NULL,
  title text NOT NULL CHECK (length(btrim(title)) > 0),
  description jsonb,
  description_text text NOT NULL DEFAULT '',
  status_id uuid REFERENCES statuses(id),
  owner_id uuid REFERENCES users(id),
  default_workflow_id uuid REFERENCES workflows(id),
  default_view_type text,
  start_date date,
  due_date date,
  duration_days integer CHECK (duration_days >= 0),
  is_milestone boolean NOT NULL DEFAULT false,
  completed_at timestamptz,
  version integer NOT NULL DEFAULT 1 CHECK (version > 0),
  fields jsonb NOT NULL DEFAULT '{}',
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid REFERENCES users(id),
  deleted_root_id uuid REFERENCES items(id),
  search tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('simple', title), 'A') ||
    setweight(to_tsvector('simple', description_text), 'B')
  ) STORED,
  CHECK (due_date >= start_date),
  CHECK (NOT is_milestone OR duration_days IS NOT DISTINCT FROM 0)
);
CREATE INDEX items_search_idx ON items USING gin (search);
CREATE INDEX items_live_status_idx ON items (status_id) WHERE deleted_at IS NULL;
CREATE INDEX items_live_due_idx ON items (due_date) WHERE deleted_at IS NULL;
CREATE TRIGGER items_set_updated_at BEFORE UPDATE ON items FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE item_locations (
  item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  space_id uuid NOT NULL REFERENCES spaces(id),
  parent_item_id uuid REFERENCES items(id),
  rank text NOT NULL,
  UNIQUE NULLS NOT DISTINCT (item_id, space_id, parent_item_id)
);
CREATE INDEX item_locations_parent_rank_idx ON item_locations (parent_item_id, rank);
CREATE INDEX item_locations_root_rank_idx ON item_locations (space_id, rank) WHERE parent_item_id IS NULL;

-- +goose StatementBegin
CREATE FUNCTION prevent_item_location_cycle() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF EXISTS (
    WITH RECURSIVE ancestors(id) AS (
      SELECT NEW.parent_item_id
      UNION
      SELECT l.parent_item_id FROM item_locations l JOIN ancestors a ON l.item_id = a.id
    )
    SELECT 1 FROM ancestors WHERE id = NEW.item_id
  ) THEN
    RAISE EXCEPTION 'item location cycle' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;
CREATE FUNCTION lock_item_location_edges() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  PERFORM pg_advisory_xact_lock(5102005);
  RETURN NULL;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER item_locations_lock_edges
BEFORE INSERT OR UPDATE OF item_id, parent_item_id ON item_locations
FOR EACH STATEMENT EXECUTE FUNCTION lock_item_location_edges();
CREATE TRIGGER item_locations_prevent_cycle
AFTER INSERT OR UPDATE OF item_id, parent_item_id ON item_locations
FOR EACH ROW EXECUTE FUNCTION prevent_item_location_cycle();

CREATE TABLE item_assignees (
  item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id),
  PRIMARY KEY (item_id, user_id)
);
CREATE TABLE item_followers (
  item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id),
  PRIMARY KEY (item_id, user_id)
);
