package mail

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/anissaefuddin/pms/server/internal/bootstrap"
	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type InvitationArgs struct {
	UserID     string `json:"user_id"`
	Generation int64  `json:"generation"`
}

func (InvitationArgs) Kind() string { return "send_invitation_email" }

func (q *Queue) InviteTx(ctx context.Context, tx pgx.Tx, id string) error {
	var generation int64
	if err := tx.QueryRow(ctx, "UPDATE users SET invitation_generation=invitation_generation+1 WHERE id=$1 RETURNING invitation_generation", id).Scan(&generation); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM user_tokens WHERE user_id=$1 AND purpose='invite' AND used_at IS NULL", id); err != nil {
		return err
	}
	_, err := q.client.InsertTx(ctx, tx, InvitationArgs{UserID: id, Generation: generation}, &river.InsertOpts{MaxAttempts: 8})
	return err
}

type InvitationWorker struct {
	river.WorkerDefaults[InvitationArgs]
	pool *pgxpool.Pool
	mail *Worker
}

func (w *InvitationWorker) Work(ctx context.Context, job *river.Job[InvitationArgs]) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Use the same lock order as invitation acceptance and administration.
	if _, err := tx.Exec(ctx, "SELECT 1 FROM account WHERE id=1 FOR UPDATE"); err != nil {
		return err
	}
	args := Args{Template: "set_password"}
	var inviter *string
	err = tx.QueryRow(ctx, `SELECT email::text,locale,invitation_message,invited_by::text
FROM users WHERE id=$1 AND status='invited' AND invitation_generation=$2 FOR UPDATE`,
		job.Args.UserID, job.Args.Generation).Scan(&args.To, &args.Locale, &args.Message, &inviter)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	plain, hash, err := security.Token()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM user_tokens WHERE user_id=$1 AND purpose='invite' AND used_at IS NULL", job.Args.UserID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at,created_by) VALUES($1,$2,'invite',$3,$4)",
		hash, job.Args.UserID, time.Now().Add(bootstrap.InviteValidity), inviter); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	args.URL = fmt.Sprintf("%s/invitations/%s", w.mail.cfg.PublicURL, plain)
	return w.mail.deliver(ctx, args)
}
