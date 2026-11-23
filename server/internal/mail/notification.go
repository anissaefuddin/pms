package mail

import (
	"context"
	"errors"
	"log/slog"
	"strings"

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
	var kind, snippet string
	err = tx.QueryRow(ctx, `SELECT u.id::text,u.user_type::text,u.email::text,u.locale,
COALESCE(n.item_id::text,''),COALESCE(n.space_id::text,''),n.kind,COALESCE(n.snippet,''),
to_char(n.created_at AT TIME ZONE u.timezone,'YYYY-MM-DD HH24:MI')||' '||u.timezone FROM notifications n
JOIN users u ON u.id=n.recipient_id AND u.status='active' WHERE n.id=$1::uuid AND n.kind<>'comment'
AND NOT (CASE n.kind WHEN 'mention' THEN 'mention' WHEN 'assigned' THEN 'assigned' WHEN 'shared' THEN 'shared' ELSE 'task_changes' END = ANY(u.email_muted))`,
		job.Args.ID).Scan(&actor.ID, &actor.Type, &args.To, &args.Locale, &itemID, &spaceID, &kind, &snippet, &args.Data.OccurredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// A row with no Item or Space is the recipient's own request status (RF-08): the recipient owns it, so no access check.
	var caps policy.Caps
	if itemID != "" || spaceID != "" {
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
	if kind == "shared" {
		switch {
		case caps.Has(policy.Share):
			args.Data.Access = "Full"
		case caps.Has(policy.Edit):
			args.Data.Access = "Editor"
		case caps.Has(policy.Comment):
			args.Data.Access = "Limited"
		default:
			args.Data.Access = "Read only"
		}
	}
	args.Data.Kind = kind
	args.Message = snippet
	if itemID != "" {
		args.URL = w.mail.cfg.PublicURL + "/items/" + itemID
		err = tx.QueryRow(ctx, `SELECT COALESCE(NULLIF(concat_ws(' ',a.first_name,a.last_name),''),'LENS'),
i.title,i.type::text,COALESCE(st.name,''),COALESCE(i.due_date::text,''),
COALESCE((SELECT s.name FROM item_locations il JOIN spaces s ON s.id=il.space_id WHERE il.item_id=i.id ORDER BY il.rank LIMIT 1),'')
FROM notifications n LEFT JOIN users a ON a.id=n.actor_id JOIN items i ON i.id=n.item_id
LEFT JOIN statuses st ON st.id=i.status_id WHERE n.id=$1::uuid`, job.Args.ID).
			Scan(&args.Data.Actor, &args.Data.Item, &args.Data.ItemType, &args.Data.Status, &args.Data.Due, &args.Data.Space)
		if err != nil {
			return err
		}
	} else if spaceID != "" {
		args.URL = w.mail.cfg.PublicURL + "/spaces/" + spaceID
		err = tx.QueryRow(ctx, `SELECT COALESCE(NULLIF(concat_ws(' ',a.first_name,a.last_name),''),'LENS'),s.name
FROM notifications n LEFT JOIN users a ON a.id=n.actor_id JOIN spaces s ON s.id=n.space_id WHERE n.id=$1::uuid`, job.Args.ID).
			Scan(&args.Data.Actor, &args.Data.Item)
		if err != nil {
			return err
		}
	}
	if kind == "status_changed" && strings.Contains(snippet, "→") {
		parts := strings.SplitN(snippet, "→", 2)
		args.Data.Before = strings.TrimSpace(parts[0])
		args.Data.After = strings.TrimSpace(parts[1])
	}
	if strings.HasPrefix(kind, "approval_") {
		args.Data.Status = snippet
	}
	known := map[string]bool{"mention": true, "assigned": true, "shared": true, "status_changed": true, "approval_requested": true, "approval_decided": true, "approval_reminder": true, "approval_escalated": true, "automation": true, "import_summary": true}
	if !known[kind] {
		slog.Warn("unsupported notification email kind; using safe fallback", "kind", kind, "notification", job.Args.ID)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return w.mail.deliver(ctx, args)
}
