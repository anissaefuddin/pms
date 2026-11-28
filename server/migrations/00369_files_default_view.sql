-- +goose Up
-- +goose StatementBegin
-- Spaces, projects and folders open on Files. A project or folder created with a chosen default_view_type still opens on that view.
CREATE OR REPLACE FUNCTION seed_container_views() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE wanted text;
BEGIN
  IF TG_TABLE_NAME='spaces' THEN
    INSERT INTO views(scope_space_id,type,name,position,is_default)
    VALUES (NEW.id,'table','Table',0,false),(NEW.id,'board','Board',1,false),
           (NEW.id,'gantt','Gantt',2,false),(NEW.id,'calendar','Calendar',3,false),(NEW.id,'files','Files',4,true);
  ELSIF NEW.type IN ('project','folder') THEN
    wanted := COALESCE(NEW.default_view_type,'files');
    INSERT INTO views(scope_item_id,type,name,position,is_default)
    VALUES (NEW.id,'table','Table',0,wanted='table'),(NEW.id,'board','Board',1,wanted='board'),(NEW.id,'files','Files',2,wanted='files');
    IF wanted IN ('gantt','calendar','chart') THEN
      INSERT INTO views(scope_item_id,type,name,position,is_default)
      VALUES (NEW.id,wanted::view_type,CASE wanted WHEN 'gantt' THEN 'Gantt' WHEN 'calendar' THEN 'Calendar' ELSE 'Chart' END,3,true);
    END IF;
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- Existing Spaces, projects and folders move to Files only while they still open on Table (or on nothing). A scope whose default
-- is another view was changed on purpose and is left alone. No view is removed.
CREATE TEMPORARY TABLE files_default_scopes ON COMMIT DROP AS
SELECT s.id AS space_id, NULL::uuid AS item_id FROM spaces s
WHERE NOT EXISTS (SELECT 1 FROM views v WHERE v.scope_space_id=s.id AND v.is_default AND v.type<>'table')
UNION ALL
SELECT NULL::uuid, i.id FROM items i
WHERE i.type IN ('project','folder') AND NOT EXISTS (SELECT 1 FROM views v WHERE v.scope_item_id=i.id AND v.is_default AND v.type<>'table');

INSERT INTO views(scope_space_id,scope_item_id,type,name,position,is_default)
SELECT t.space_id, t.item_id, 'files', 'Files',
       COALESCE((SELECT max(v.position)+1 FROM views v WHERE v.scope_space_id=t.space_id OR v.scope_item_id=t.item_id),0), false
FROM files_default_scopes t
WHERE NOT EXISTS (SELECT 1 FROM views v WHERE (v.scope_space_id=t.space_id OR v.scope_item_id=t.item_id) AND v.type='files' AND v.visibility='public');

UPDATE views v SET is_default=false FROM files_default_scopes t
WHERE v.is_default AND (v.scope_space_id=t.space_id OR v.scope_item_id=t.item_id);

UPDATE views SET is_default=true WHERE id IN (
  SELECT DISTINCT ON (t.space_id, t.item_id) v.id FROM files_default_scopes t
  JOIN views v ON (v.scope_space_id=t.space_id OR v.scope_item_id=t.item_id) AND v.type='files' AND v.visibility='public'
  ORDER BY t.space_id, t.item_id, v.position, v.id);

-- +goose Down
-- Restores the previous seeding for new containers. Defaults and views already changed stay: later choices cannot be told apart.
-- +goose StatementBegin
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
