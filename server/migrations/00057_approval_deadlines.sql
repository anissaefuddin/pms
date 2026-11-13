-- +goose Up
ALTER TABLE approvals ADD COLUMN deadline_stage integer NOT NULL DEFAULT 0 CHECK (deadline_stage BETWEEN 0 AND 2);
CREATE INDEX approvals_deadline_idx ON approvals (due_date, id) WHERE due_date IS NOT NULL AND deadline_stage < 2;

ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (kind IN ('mention','assigned','status_changed','comment','shared','approval_requested','approval_decided','automation','approval_reminder','approval_escalated'));

-- +goose Down
DELETE FROM notifications WHERE kind IN ('approval_reminder','approval_escalated');
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (kind IN ('mention','assigned','status_changed','comment','shared','approval_requested','approval_decided','automation'));
DROP INDEX approvals_deadline_idx;
ALTER TABLE approvals DROP COLUMN deadline_stage;
