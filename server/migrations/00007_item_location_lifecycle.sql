-- +goose Up
ALTER TABLE item_locations ADD COLUMN deleted_root_id uuid REFERENCES items(id) ON DELETE SET NULL;
CREATE INDEX item_locations_live_order_idx
  ON item_locations (space_id, parent_item_id, rank COLLATE "C", item_id)
  WHERE deleted_root_id IS NULL;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION shares_check_refs() RETURNS trigger AS $$
BEGIN
  IF NEW.target_type = 'space' THEN
    IF NOT EXISTS (SELECT 1 FROM spaces WHERE id = NEW.target_id AND kind = 'shared' AND deleted_at IS NULL FOR KEY SHARE) THEN
      RAISE EXCEPTION 'share target is not a live shared Space' USING ERRCODE = '23503';
    END IF;
  ELSIF NOT EXISTS (SELECT 1 FROM items WHERE id = NEW.target_id AND deleted_at IS NULL FOR KEY SHARE)
    OR NOT EXISTS (
      SELECT 1 FROM item_locations l JOIN spaces s ON s.id=l.space_id
      WHERE l.item_id=NEW.target_id AND l.deleted_root_id IS NULL AND s.kind='shared' AND s.deleted_at IS NULL
    ) OR EXISTS (
      SELECT 1 FROM item_locations l JOIN spaces s ON s.id=l.space_id
      WHERE l.item_id=NEW.target_id AND l.deleted_root_id IS NULL AND s.kind='personal'
    ) THEN
    RAISE EXCEPTION 'share target must be a live Item in shared Spaces' USING ERRCODE = '23503';
  END IF;
  IF NEW.principal_type = 'user' THEN
    IF NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.principal_id AND status <> 'deactivated' FOR KEY SHARE) THEN
      RAISE EXCEPTION 'share principal user does not exist or is deactivated' USING ERRCODE = '23503';
    END IF;
  ELSIF NOT EXISTS (SELECT 1 FROM groups WHERE id = NEW.principal_id FOR KEY SHARE) THEN
    RAISE EXCEPTION 'share principal group does not exist' USING ERRCODE = '23503';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
