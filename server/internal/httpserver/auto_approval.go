package httpserver

import (
	"context"
	"log/slog"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func init() { items.OnStatusEntered = startStatusApproval }

// startStatusApproval creates the approval a status asks for (WF-03B). Approvers who cannot see the item are left out;
// with none left the change goes through without an approval and the problem is logged, so a stale setting never blocks work.
func startStatusApproval(ctx context.Context, tx pgx.Tx, actor policy.Actor, itemID, statusID string, batch *notifications.Batch) error {
	var approvers []uuid.UUID
	var onApproved, onRejected *uuid.UUID
	var name string
	if err := tx.QueryRow(ctx, "SELECT approval_approver_ids, approved_status_id, rejected_status_id, name FROM statuses WHERE id=$1::uuid", statusID).Scan(&approvers, &onApproved, &onRejected, &name); err != nil {
		return err
	}
	if len(approvers) == 0 {
		return nil
	}
	item := uuid.MustParse(itemID)
	allowed := make([]uuid.UUID, 0, len(approvers))
	for _, id := range approvers {
		if _, err := checkApprovers(ctx, tx, []uuid.UUID{id}, item); err == nil {
			allowed = append(allowed, id)
		}
	}
	if len(allowed) == 0 {
		slog.Warn("status approval skipped: no configured approver can access the item", "item", itemID, "status", statusID)
		return nil
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	text := "Approval required to enter " + name
	if _, err := tx.Exec(ctx, `INSERT INTO approvals(id,item_id,description,created_by,trigger_status_id,on_approved_status_id,on_rejected_status_id)
VALUES($1,$2,$3,$4::uuid,$5::uuid,$6,$7)`, id, item, text, actor.ID, statusID, onApproved, onRejected); err != nil {
		return err
	}
	return addApprovers(ctx, tx, actor, id, item, allowed, text, batch)
}

// applyApprovalOutcome moves the item to the status chosen for the result, if the approval has one and the item is still
// in the status that started it. It is a system move: the people who decided are not asked to satisfy the status guards.
func applyApprovalOutcome(ctx context.Context, tx pgx.Tx, actor policy.Actor, approvalID, itemID uuid.UUID) error {
	var result string
	var target *uuid.UUID
	var trigger *uuid.UUID
	err := tx.QueryRow(ctx, `SELECT CASE WHEN EXISTS(SELECT 1 FROM approval_approvers x WHERE x.approval_id=a.id AND x.decision='rejected') THEN 'rejected'
 WHEN NOT EXISTS(SELECT 1 FROM approval_approvers x WHERE x.approval_id=a.id AND x.decision IS DISTINCT FROM 'approved') THEN 'approved' ELSE 'pending' END,
 CASE WHEN EXISTS(SELECT 1 FROM approval_approvers x WHERE x.approval_id=a.id AND x.decision='rejected') THEN a.on_rejected_status_id ELSE a.on_approved_status_id END, a.trigger_status_id
FROM approvals a WHERE a.id=$1`, approvalID).Scan(&result, &target, &trigger)
	if err != nil || result == "pending" || target == nil || trigger == nil {
		return err
	}
	var from string
	err = tx.QueryRow(ctx, `UPDATE items i SET status_id=$2, version=version+1, status_changed_at=now(),
 completed_at=CASE WHEN (SELECT grp='completed' FROM statuses WHERE id=$2) THEN COALESCE(completed_at,now()) ELSE NULL END
WHERE i.id=$1 AND i.status_id=$3 AND i.deleted_at IS NULL RETURNING $3::text`, itemID, *target, *trigger).Scan(&from)
	if err == pgx.ErrNoRows {
		return nil // the item moved on by hand; the decision stays on record without moving it again
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO activity(item_id,actor_id,action,changes) VALUES ($1,$2::uuid,'update',jsonb_build_object('status_id',jsonb_build_object('from',$3::text,'to',$4::text),'reason','approval_'||$5::text))`,
		itemID, actor.ID, from, target.String(), result)
	return err
}
