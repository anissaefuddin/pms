package items

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// LockGrants holds the rows that give the actor their rights until this transaction ends, so an assignment cannot commit after the
// right it was decided on has been taken away (DEF-049): a revoke waits, or this serializable transaction fails and is retried.
// (exported for the share endpoints)
func LockGrants(ctx context.Context, tx pgx.Tx, actorID string) error {
	return lockGrants(ctx, tx, actorID)
}

func lockGrants(ctx context.Context, tx pgx.Tx, actorID string) error {
	for _, q := range []string{
		"SELECT 1 FROM group_members WHERE user_id=$1::uuid FOR SHARE",
		`SELECT 1 FROM shares WHERE (principal_type='user' AND principal_id=$1::uuid)
 OR (principal_type='group' AND principal_id IN (SELECT group_id FROM group_members WHERE user_id=$1::uuid)) FOR SHARE`,
		"SELECT 1 FROM space_admins WHERE user_id=$1::uuid FOR SHARE",
	} {
		if _, err := tx.Exec(ctx, q, actorID); err != nil {
			return err
		}
	}
	return nil
}
