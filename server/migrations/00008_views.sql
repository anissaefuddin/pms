-- +goose Up
CREATE TYPE view_type AS ENUM ('table','board');
CREATE TYPE view_visibility AS ENUM ('public','personal');

CREATE TABLE views (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scope_space_id uuid REFERENCES spaces(id) ON DELETE CASCADE,
  scope_item_id uuid REFERENCES items(id) ON DELETE CASCADE,
  type view_type NOT NULL,
  name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 80),
  config jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(config)='object'),
  visibility view_visibility NOT NULL DEFAULT 'public',
  owner_id uuid REFERENCES users(id),
  position integer NOT NULL CHECK (position >= 0),
  is_default boolean NOT NULL DEFAULT false,
  CHECK (num_nonnulls(scope_space_id,scope_item_id)=1),
  CHECK (visibility='public' OR (owner_id IS NOT NULL AND NOT is_default))
);
CREATE UNIQUE INDEX views_default_space_idx ON views(scope_space_id) WHERE is_default AND scope_space_id IS NOT NULL;
CREATE UNIQUE INDEX views_default_item_idx ON views(scope_item_id) WHERE is_default AND scope_item_id IS NOT NULL;
CREATE INDEX views_space_order_idx ON views(scope_space_id,position,id);
CREATE INDEX views_item_order_idx ON views(scope_item_id,position,id);

-- +goose StatementBegin
CREATE FUNCTION seed_container_views() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_TABLE_NAME='spaces' THEN
    INSERT INTO views(scope_space_id,type,name,position,is_default)
    VALUES (NEW.id,'table','Table',0,true),(NEW.id,'board','Board',1,false);
  ELSIF NEW.type IN ('project','folder') THEN
    INSERT INTO views(scope_item_id,type,name,position,is_default)
    VALUES (NEW.id,'table','Table',0,COALESCE(NEW.default_view_type,'table')='table'),
           (NEW.id,'board','Board',1,COALESCE(NEW.default_view_type,'table')='board');
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER spaces_seed_views AFTER INSERT ON spaces FOR EACH ROW EXECUTE FUNCTION seed_container_views();
CREATE TRIGGER items_seed_views AFTER INSERT ON items FOR EACH ROW EXECUTE FUNCTION seed_container_views();

INSERT INTO views(scope_space_id,type,name,position,is_default)
SELECT s.id,v.type::view_type,v.name,v.position,v.type='table'
FROM spaces s CROSS JOIN (VALUES ('table','Table',0),('board','Board',1)) v(type,name,position);
INSERT INTO views(scope_item_id,type,name,position,is_default)
SELECT i.id,v.type::view_type,v.name,v.position,v.type=COALESCE(i.default_view_type,'table')
FROM items i CROSS JOIN (VALUES ('table','Table',0),('board','Board',1)) v(type,name,position)
WHERE i.type IN ('project','folder');
