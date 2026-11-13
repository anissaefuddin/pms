package httpserver

import (
	"context"
	"time"

	"github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type approvalDeadlineArgs struct{}

func (approvalDeadlineArgs) Kind() string { return "approval_deadlines" }

type approvalDeadlineWorker struct {
	river.WorkerDefaults[approvalDeadlineArgs]
	pool *pgxpool.Pool
}

func ApprovalDeadlines(pool *pgxpool.Pool) mail.Option {
	return func(workers *river.Workers, periodic *[]*river.PeriodicJob) {
		river.AddWorker(workers, &approvalDeadlineWorker{pool: pool})
		*periodic = append(*periodic, river.NewPeriodicJob(river.PeriodicInterval(15*time.Minute),
			func() (river.JobArgs, *river.InsertOpts) { return approvalDeadlineArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true}))
	}
}

func (w *approvalDeadlineWorker) Work(ctx context.Context, _ *river.Job[approvalDeadlineArgs]) error {
	client := river.ClientFromContext[pgx.Tx](ctx)
	return DeliverApprovalDeadlines(ctx, w.pool, time.Now(), func(ctx context.Context, tx pgx.Tx, id string) error {
		_, err := client.InsertTx(ctx, tx, mail.NotificationArgs{ID: id}, &river.InsertOpts{MaxAttempts: 8})
		return err
	})
}

// DeliverApprovalDeadlines advances one bounded batch and queues delivery in the same transaction.
func DeliverApprovalDeadlines(ctx context.Context, pool *pgxpool.Pool, now time.Time, enqueue func(context.Context, pgx.Tx, string) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	today := now.UTC().Format(time.DateOnly)
	rows, err := tx.Query(ctx, `
SELECT a.id::text,a.item_id::text,a.created_by::text,a.deadline_stage
FROM approvals a JOIN items i ON i.id=a.item_id AND i.deleted_at IS NULL
WHERE a.due_date <= $1::date
AND (a.deadline_stage=0 OR (a.deadline_stage=1 AND a.due_date < $1::date))
AND (`+approvalOutcome+`)='pending'
ORDER BY a.due_date,a.id LIMIT 50 FOR UPDATE OF a SKIP LOCKED`, today)
	if err != nil {
		return err
	}
	type due struct {
		ID, Item, Creator string
		Stage             int
	}
	batch, err := pgx.CollectRows(rows, pgx.RowToStructByPos[due])
	if err != nil {
		return err
	}
	for _, approval := range batch {
		kind := "approval_reminder"
		recipientSQL := `SELECT u.id::text,u.user_type::text FROM users u
JOIN approval_approvers x ON x.user_id=u.id WHERE x.approval_id=$1 AND x.decision IS NULL AND u.status='active'`
		recipientID := approval.ID
		if approval.Stage == 1 {
			kind = "approval_escalated"
			recipientSQL = "SELECT id::text,user_type::text FROM users WHERE id=$1 AND status='active'"
			recipientID = approval.Creator
		}
		rows, err := tx.Query(ctx, recipientSQL, recipientID)
		if err != nil {
			return err
		}
		recipients, err := pgx.CollectRows(rows, pgx.RowToStructByPos[policy.Actor])
		if err != nil {
			return err
		}
		for _, recipient := range recipients {
			visible, err := notifications.CanView(ctx, tx, recipient, notifications.Target{Item: approval.Item})
			if err != nil {
				return err
			}
			if !visible {
				continue
			}
			id := uuid.NewString()
			if _, err := tx.Exec(ctx, "INSERT INTO notifications(id,recipient_id,kind,item_id) VALUES ($1,$2,$3,$4)", id, recipient.ID, kind, approval.Item); err != nil {
				return err
			}
			if err := enqueue(ctx, tx, id); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, "UPDATE approvals SET deadline_stage=deadline_stage+1 WHERE id=$1", approval.ID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
