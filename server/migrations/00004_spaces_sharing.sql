-- +goose Up
ALTER TABLE spaces
  ADD COLUMN deleted_at timestamptz,
  ADD COLUMN deleted_by uuid REFERENCES users(id);
CREATE INDEX spaces_live_idx ON spaces (kind, name) WHERE deleted_at IS NULL;

CREATE TYPE share_target AS ENUM ('space', 'item');
CREATE TYPE principal_type AS ENUM ('user', 'group');
CREATE TYPE share_role AS ENUM ('read', 'limited', 'editor', 'full');

CREATE TABLE groups (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL CHECK (btrim(name) <> ''),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX groups_name_key ON groups (lower(name));
CREATE TRIGGER groups_set_updated_at BEFORE UPDATE ON groups FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE group_members (
  group_id uuid NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (group_id, user_id)
);
CREATE INDEX group_members_user_idx ON group_members (user_id);

-- One table for every shareable thing. target_id and principal_id are polymorphic, so a trigger (below) checks that they
-- exist when a row is written, and housekeeping removes shares whose target or principal is gone later.
CREATE TABLE shares (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  target_type share_target NOT NULL,
  target_id uuid NOT NULL,
  principal_type principal_type NOT NULL,
  principal_id uuid NOT NULL,
  role share_role NOT NULL,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (target_type, target_id, principal_type, principal_id)
);
CREATE INDEX shares_principal_idx ON shares (principal_type, principal_id);

-- +goose StatementBegin
CREATE FUNCTION shares_check_refs() RETURNS trigger AS $$
BEGIN
  IF NEW.target_type = 'space' THEN
    IF NOT EXISTS (SELECT 1 FROM spaces WHERE id = NEW.target_id AND kind = 'shared' AND deleted_at IS NULL FOR KEY SHARE) THEN
      RAISE EXCEPTION 'share target % is not a live shared space', NEW.target_id USING ERRCODE = '23503';
    END IF;
  ELSE
    RAISE EXCEPTION 'sharing % is not available yet', NEW.target_type USING ERRCODE = '23503'; -- items arrive with S1.2.1
  END IF;
  IF NEW.principal_type = 'user' THEN
    IF NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.principal_id AND status <> 'deactivated' FOR KEY SHARE) THEN
      RAISE EXCEPTION 'share principal user % does not exist or is deactivated', NEW.principal_id USING ERRCODE = '23503';
    END IF;
  ELSIF NOT EXISTS (SELECT 1 FROM groups WHERE id = NEW.principal_id FOR KEY SHARE) THEN
    RAISE EXCEPTION 'share principal group % does not exist', NEW.principal_id USING ERRCODE = '23503';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER shares_check_refs BEFORE INSERT OR UPDATE OF target_type, target_id, principal_type, principal_id
  ON shares FOR EACH ROW EXECUTE FUNCTION shares_check_refs();
