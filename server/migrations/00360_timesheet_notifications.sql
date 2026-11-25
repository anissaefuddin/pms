-- +goose Up
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (kind IN ('mention','assigned','status_changed','comment','shared','approval_requested','approval_decided','automation','approval_reminder','approval_escalated','import_summary','timesheet_submitted','timesheet_approved','timesheet_rejected','timesheet_locked'));

-- +goose Down
DELETE FROM notifications WHERE kind LIKE 'timesheet\_%';
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (kind IN ('mention','assigned','status_changed','comment','shared','approval_requested','approval_decided','automation','approval_reminder','approval_escalated','import_summary'));
