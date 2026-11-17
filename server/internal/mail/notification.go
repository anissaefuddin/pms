package mail

import (
	"context"
	"errors"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type NotificationArgs struct {
	ID string `json:"notification_id"`
}

func (NotificationArgs) Kind() string { return "send_notification_email" }

type NotificationWorker struct {
	river.WorkerDefaults[NotificationArgs]
	pool *pgxpool.Pool
	mail *Worker
}

func (q *Queue) NotifyTx(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := q.client.InsertTx(ctx, tx, NotificationArgs{ID: id}, &river.InsertOpts{MaxAttempts: 8})
	return err
}

func (w *NotificationWorker) Work(ctx context.Context, job *river.Job[NotificationArgs]) error {
	var actor policy.Actor
	var itemID, spaceID string
	args := Args{Template: "notification", URL: w.mail.cfg.PublicURL + "/inbox"}
	tx, err := w.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT u.id::text,u.user_type::text,u.email::text,u.locale,
COALESCE(n.item_id::text,''),COALESCE(n.space_id::text,'') FROM notifications n
JOIN users u ON u.id=n.recipient_id AND u.status='active' WHERE n.id=$1::uuid AND n.kind<>'comment'
AND NOT (CASE n.kind WHEN 'mention' THEN 'mention' WHEN 'assigned' THEN 'assigned' WHEN 'shared' THEN 'shared' ELSE 'task_changes' END = ANY(u.email_muted))`,
		job.Args.ID).Scan(&actor.ID, &actor.Type, &args.To, &args.Locale, &itemID, &spaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// A row with no Item or Space is the recipient's own request status (RF-08): the recipient owns it, so no access check.
	if itemID != "" || spaceID != "" {
		var caps policy.Caps
		if itemID != "" {
			caps, err = policy.ItemCaps(ctx, tx, actor, itemID)
		} else {
			space, _, lookupErr := policy.SpaceAccessFor(ctx, tx, actor, spaceID)
			caps, err = space.Caps, lookupErr
		}
		if err != nil || !caps.Has(policy.View) {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return w.mail.deliver(ctx, args)
}
