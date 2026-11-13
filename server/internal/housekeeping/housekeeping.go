// Package housekeeping deletes rows nobody can use any more: expired sessions, spent or expired tokens and orphaned shares.
// Invited users are kept on purpose: they hold a seat until an administrator cancels the invitation.
package housekeeping

import (
	"context"
	"log/slog"
	"time"

	"github.com/anissaefuddin/pms/server/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type Args struct{}

func (Args) Kind() string { return "housekeeping" }

type Worker struct {
	river.WorkerDefaults[Args]
	Pool     *pgxpool.Pool
	FilesDir string // empty: skip the file sweep
}

func (w *Worker) Work(ctx context.Context, _ *river.Job[Args]) error {
	if err := Run(ctx, w.Pool); err != nil {
		return err
	}
	if w.FilesDir == "" {
		return nil
	}
	removed, err := SweepFiles(ctx, w.Pool, storage.LocalStorage{Dir: w.FilesDir}, fileGrace)
	slog.Info("housekeeping files", "orphans_removed", removed)
	return err
}

// An upload writes the file before its row, so a young file without a row may still be in flight.
const fileGrace = 24 * time.Hour

// SweepFiles removes stored files no attachment row points to (a crash between write and insert, or a purged item).
// Rows that are only soft-deleted keep their file: the Recycle bin can restore them.
func SweepFiles(ctx context.Context, pool *pgxpool.Pool, store storage.LocalStorage, grace time.Duration) (int, error) {
	keys, err := store.Keys(time.Now().Add(-grace))
	if err != nil {
		return 0, err
	}
	removed := 0
	for len(keys) > 0 {
		batch := keys[:min(500, len(keys))]
		keys = keys[len(batch):]
		rows, err := pool.Query(ctx, "SELECT storage_key FROM attachment_versions WHERE storage_key=ANY($1) UNION SELECT storage_key FROM attachments WHERE storage_key=ANY($1) UNION SELECT p->>'key' FROM attachment_previews, jsonb_array_elements(pages) p WHERE p->>'key'=ANY($1)", batch)
		if err != nil {
			return removed, err
		}
		used := map[string]bool{}
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				rows.Close()
				return removed, err
			}
			used[key] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return removed, err
		}
		for _, key := range batch {
			if used[key] {
				continue
			}
			if err := store.Remove(key); err != nil {
				return removed, err
			}
			removed++
		}
	}
	return removed, nil
}

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
	// Shares point at users, groups and Spaces without a foreign key (they are polymorphic), so clean up the orphans here.
	// Soft-deleted Spaces keep their shares: the Recycle bin restores them.
	shares, err := pool.Exec(ctx, `DELETE FROM shares sh WHERE
		(sh.principal_type = 'group' AND NOT EXISTS (SELECT 1 FROM groups g WHERE g.id = sh.principal_id))
		OR (sh.principal_type = 'user' AND NOT EXISTS (SELECT 1 FROM users u WHERE u.id = sh.principal_id))
		OR (sh.target_type = 'space' AND NOT EXISTS (SELECT 1 FROM spaces s WHERE s.id = sh.target_id))
		OR (sh.target_type = 'item' AND NOT EXISTS (SELECT 1 FROM items i WHERE i.id = sh.target_id))`)
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "DELETE FROM project_excel_exports WHERE expires_at<=now()"); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "DELETE FROM project_excel_imports WHERE expires_at<=now()"); err != nil {
		return err
	}
	slog.Info("housekeeping", "sessions", sessions.RowsAffected(), "tokens", tokens.RowsAffected(), "shares", shares.RowsAffected())
	return nil
}
