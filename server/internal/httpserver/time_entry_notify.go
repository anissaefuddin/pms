package httpserver

import (
	"context"

	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

// timeReviewers are the people who may approve time on a Task: Full access to it. The author is left out by Record.
func timeReviewers(ctx context.Context, tx pgx.Tx, item string) ([]notifications.Principal, error) {
	rows, err := tx.Query(ctx, `WITH places AS (SELECT space_id FROM item_locations WHERE item_id=$1::uuid AND deleted_root_id IS NULL),
targets AS (SELECT sh.principal_type, sh.principal_id FROM shares sh
  WHERE (sh.target_type='space' AND sh.target_id IN (SELECT space_id FROM places)) OR (sh.target_type='item' AND sh.target_id=$1::uuid)
    OR (sh.target_type='item' AND sh.target_id IN (SELECT parent_item_id FROM item_locations WHERE item_id=$1::uuid AND parent_item_id IS NOT NULL))
  UNION SELECT 'user', user_id FROM space_admins WHERE space_id IN (SELECT space_id FROM places))
SELECT u.id::text, u.user_type::text FROM users u WHERE u.status='active' AND (
  u.id IN (SELECT principal_id FROM targets WHERE principal_type='user')
  OR u.id IN (SELECT user_id FROM group_members WHERE group_id IN (SELECT principal_id FROM targets WHERE principal_type='group')))`, item)
	if err != nil {
		return nil, err
	}
	candidates, err := policy.CollectActors(rows)
	if err != nil {
		return nil, err
	}
	var out []notifications.Principal
	for _, person := range candidates {
		caps, err := policy.ItemCaps(ctx, tx, person, item)
		if err != nil {
			return nil, err
		}
		if caps.Contains(policy.RoleCaps(policy.Full)) {
			out = append(out, notifications.Principal{Kind: "user", ID: person.ID})
		}
	}
	return out, nil
}

// notifyTimeEntry tells the author about a step someone else took on their entry. Record checks that they still see the Task.
func notifyTimeEntry(ctx context.Context, tx pgx.Tx, batch *notifications.Batch, actor policy.Actor, e timeEntry, kind, note string) error {
	_, err := batch.Record(ctx, tx, actor, kind, notifications.Target{Item: e.ItemID}, note, []notifications.Principal{{Kind: "user", ID: e.UserID}}, nil)
	return err
}
