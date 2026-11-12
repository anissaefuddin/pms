// Package housekeeping deletes rows nobody can use any more: expired sessions and spent or expired tokens.
// Invited users are kept on purpose: they hold a seat until an administrator cancels the invitation.
package housekeeping

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type Args struct{}

func (Args) Kind() string { return "housekeeping" }

type Worker struct {
	river.WorkerDefaults[Args]
	Pool *pgxpool.Pool
}

func (w *Worker) Work(ctx context.Context, _ *river.Job[Args]) error { return Run(ctx, w.Pool) }

// Run deletes expired rows and logs how many were removed.
func Run(ctx context.Context, pool *pgxpool.Pool) error {
	// expires_at <= absolute_expires_at, so this one predicate also covers the absolute limit.
	sessions, err := pool.Exec(ctx, "DELETE FROM sessions WHERE expires_at<now()")
	if err != nil {
		return err
	}
	tokens, err := pool.Exec(ctx, "DELETE FROM user_tokens WHERE expires_at<now() OR used_at IS NOT NULL")
	if err != nil {
		return err
	}
	slog.Info("housekeeping", "sessions", sessions.RowsAffected(), "tokens", tokens.RowsAffected())
	return nil
}
