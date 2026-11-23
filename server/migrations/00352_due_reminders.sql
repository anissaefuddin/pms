-- +goose Up
-- Existing users opt in; users created after launch start with the recommended digest schedule.
ALTER TABLE users ADD COLUMN due_reminders jsonb NOT NULL DEFAULT '{"enabled":true,"days_before":[3,1,0],"overdue":true,"overdue_every_days":3,"delivery":"digest","hour":8}'::jsonb;
UPDATE users SET due_reminders=jsonb_set(due_reminders,'{enabled}','false');

CREATE TABLE due_reminder_deliveries (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  due_date date NOT NULL,
  kind text NOT NULL CHECK (kind IN ('task_due_soon','task_due_today','task_overdue')),
  offset_days integer NOT NULL,
  sent_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id,item_id,due_date,kind,offset_days)
);
CREATE INDEX due_reminder_deliveries_sent_idx ON due_reminder_deliveries(sent_at);

-- +goose Down
DROP TABLE due_reminder_deliveries;
ALTER TABLE users DROP COLUMN due_reminders;
