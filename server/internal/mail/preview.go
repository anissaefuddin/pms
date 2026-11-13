package mail

import (
	"context"

	"github.com/anissaefuddin/pms/server/internal/previews"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

// PreviewTx queues the preview render of one file version in the caller's transaction (FIL-02).
func (q *Queue) PreviewTx(ctx context.Context, tx pgx.Tx, args previews.Args) error {
	_, err := q.client.InsertTx(ctx, tx, args, &river.InsertOpts{MaxAttempts: 3})
	return err
}
