package items

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrAssigneeIneligible means a client-owned item would gain an assignee who is not an active member of its Customer (B3-06).
// The message names no person and no Customer.
var ErrAssigneeIneligible = errors.New("an assignee must be an active member of this item's customer")

// checkNewAssignees refuses a client-owned item that gains an assignee without an active membership of its Customer. Users who
// are already assigned are not checked again, so a stale or revoked assignee can still be removed (AS-11). The membership row is
// read FOR SHARE, so a revoke (FOR UPDATE on that row) waits for this transaction. Internal items keep the existing rules.
func checkNewAssignees(ctx context.Context, tx pgx.Tx, itemID string, own ownership, assignees []uuid.UUID) error {
	if own.scope != "client" {
		return nil
	}
	for _, user := range assignees {
		if itemID != "" {
			var already bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM item_assignees WHERE item_id=$1::uuid AND user_id=$2::uuid)", itemID, user).Scan(&already); err != nil {
				return err
			}
			if already {
				continue
			}
		}
		var one int
		err := tx.QueryRow(ctx, `SELECT 1 FROM client_memberships m JOIN users u ON u.id = m.user_id
WHERE m.client_id=$1::uuid AND m.user_id=$2::uuid AND m.status='active' AND u.status='active' AND u.user_type IN ('admin','regular')
FOR SHARE OF m`, own.client, user).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAssigneeIneligible
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// patchAssignees checks the assignees of a PATCH against the item's stored stable ownership. It runs inside the PATCH transaction.
func patchAssignees(ctx context.Context, tx pgx.Tx, itemID string, assignees []uuid.UUID) error {
	var scope string
	var client *string
	if err := tx.QueryRow(ctx, "SELECT scope_state, client_id::text FROM items WHERE id=$1::uuid", itemID).Scan(&scope, &client); err != nil {
		return err
	}
	own := ownership{scope: scope}
	if client != nil {
		own.client = *client
	}
	return checkNewAssignees(ctx, tx, itemID, own, assignees)
}
