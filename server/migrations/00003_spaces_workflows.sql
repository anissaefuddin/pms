-- +goose Up
CREATE TYPE space_kind AS ENUM ('shared', 'personal');
CREATE TYPE status_group AS ENUM ('active', 'completed', 'deferred', 'cancelled');

CREATE TABLE spaces (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  kind space_kind NOT NULL DEFAULT 'shared',
  owner_id uuid REFERENCES users(id),
  version integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  -- a Personal space always has an owner
  CONSTRAINT spaces_personal_has_owner CHECK (kind <> 'personal' OR owner_id IS NOT NULL)
);
CREATE UNIQUE INDEX spaces_one_personal_per_owner ON spaces (owner_id) WHERE kind = 'personal';
CREATE TRIGGER spaces_set_updated_at BEFORE UPDATE ON spaces FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE workflows (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  is_default boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX workflows_one_default ON workflows ((true)) WHERE is_default;

CREATE TABLE statuses (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id uuid NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
  name text NOT NULL,
  grp status_group NOT NULL,
  color text NOT NULL, -- design-token palette name, not hex
  position integer NOT NULL,
  UNIQUE (workflow_id, name),
  UNIQUE (workflow_id, position) DEFERRABLE INITIALLY DEFERRED -- lets a reorder swap positions inside one transaction
);
