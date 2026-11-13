-- +goose Up
-- Approval on entering a status (WF-03B): a status can name approvers and the statuses an Approved or Rejected result moves the item to.
ALTER TABLE statuses
    ADD COLUMN approval_approver_ids uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN approved_status_id uuid REFERENCES statuses(id) ON DELETE SET NULL,
    ADD COLUMN rejected_status_id uuid REFERENCES statuses(id) ON DELETE SET NULL;
-- The approval remembers which status started it and where to go, so a later change of the status settings cannot redirect it.
ALTER TABLE approvals
    ADD COLUMN trigger_status_id uuid REFERENCES statuses(id) ON DELETE SET NULL,
    ADD COLUMN on_approved_status_id uuid REFERENCES statuses(id) ON DELETE SET NULL,
    ADD COLUMN on_rejected_status_id uuid REFERENCES statuses(id) ON DELETE SET NULL;
