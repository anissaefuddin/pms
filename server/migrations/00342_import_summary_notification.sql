-- +goose Up
-- One summary notification per bulk import (decision 14). Adds the kind to the CHECK constraint; existing rows are unaffected.
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (kind IN ('mention','assigned','status_changed','comment','shared','approval_requested','approval_decided','automation','approval_reminder','approval_escalated','import_summary'));

-- +goose Down
DELETE FROM notifications WHERE kind = 'import_summary';
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (kind IN ('mention','assigned','status_changed','comment','shared','approval_requested','approval_decided','automation','approval_reminder','approval_escalated'));
