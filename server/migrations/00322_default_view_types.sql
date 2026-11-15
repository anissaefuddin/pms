-- +goose Up
-- +goose StatementBegin
-- Project and folder seeding now also creates the non-table/board view chosen as default_view_type, marked default.
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

-- +goose Down
-- Restores the table/board-only seeding. Extra views already created stay in place; they are not deleted
-- because user content may reference them.
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
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
