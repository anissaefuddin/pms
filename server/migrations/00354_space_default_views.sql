-- +goose Up
-- +goose StatementBegin
-- A Space now starts with all four planning views (Table, Board, Gantt, Calendar). Table stays the default
-- until someone marks another public view as default.
CREATE OR REPLACE FUNCTION seed_container_views() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_TABLE_NAME='spaces' THEN
    INSERT INTO views(scope_space_id,type,name,position,is_default)
    VALUES (NEW.id,'table','Table',0,true),(NEW.id,'board','Board',1,false),
           (NEW.id,'gantt','Gantt',2,false),(NEW.id,'calendar','Calendar',3,false);
  ELSIF NEW.type IN ('project','folder') THEN
    INSERT INTO views(scope_item_id,type,name,position,is_default)
    VALUES (NEW.id,'table','Table',0,COALESCE(NEW.default_view_type,'table')='table'),
           (NEW.id,'board','Board',1,COALESCE(NEW.default_view_type,'table')='board');
    IF NEW.default_view_type IN ('gantt','calendar','files','chart') THEN
      INSERT INTO views(scope_item_id,type,name,position,is_default)
      VALUES (NEW.id,NEW.default_view_type::view_type,
              CASE NEW.default_view_type WHEN 'gantt' THEN 'Gantt' WHEN 'calendar' THEN 'Calendar' WHEN 'files' THEN 'Files' ELSE 'Chart' END,
              2,true);
    END IF;
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- Spaces created before this migration get the Gantt and Calendar views they are missing. Nothing existing is changed or removed.
INSERT INTO views(scope_space_id,type,name,position,is_default)
SELECT s.id, v.type::view_type, v.name, v.pos, false
FROM spaces s
CROSS JOIN (VALUES ('gantt','Gantt',2),('calendar','Calendar',3)) AS v(type,name,pos)
WHERE NOT EXISTS (SELECT 1 FROM views x WHERE x.scope_space_id = s.id AND x.type = v.type::view_type);

-- +goose Down
-- Restores the table/board-only seeding for new Spaces. Views already created stay in place; user content may reference them.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION seed_container_views() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_TABLE_NAME='spaces' THEN
    INSERT INTO views(scope_space_id,type,name,position,is_default)
    VALUES (NEW.id,'table','Table',0,true),(NEW.id,'board','Board',1,false);
  ELSIF NEW.type IN ('project','folder') THEN
    INSERT INTO views(scope_item_id,type,name,position,is_default)
    VALUES (NEW.id,'table','Table',0,COALESCE(NEW.default_view_type,'table')='table'),
           (NEW.id,'board','Board',1,COALESCE(NEW.default_view_type,'table')='board');
    IF NEW.default_view_type IN ('gantt','calendar','files','chart') THEN
      INSERT INTO views(scope_item_id,type,name,position,is_default)
      VALUES (NEW.id,NEW.default_view_type::view_type,
              CASE NEW.default_view_type WHEN 'gantt' THEN 'Gantt' WHEN 'calendar' THEN 'Calendar' WHEN 'files' THEN 'Files' ELSE 'Chart' END,
              2,true);
    END IF;
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
