-- +goose Up
-- RF-08 decision: a requester who cannot open the Item gets its status change as a notification with no Item or Space link.
-- Such a row belongs to its recipient only; the inbox and mail check the recipient, not an Item.
ALTER TABLE notifications DROP CONSTRAINT notifications_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_target_check CHECK (num_nonnulls(item_id,space_id)<=1);

-- +goose Down
-- Rows without a target are deleted, because the old rule requires exactly one target.
DELETE FROM notifications WHERE item_id IS NULL AND space_id IS NULL;
ALTER TABLE notifications DROP CONSTRAINT notifications_target_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_check CHECK (num_nonnulls(item_id,space_id)=1);
