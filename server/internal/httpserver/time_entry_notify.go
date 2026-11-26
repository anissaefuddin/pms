package httpserver

import (
	"context"

	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

// timeReviewers names, for each Task, the people who may approve its time: Full access to it, however far up it was given (the
// Task, anything above it, or the Space). The author is left out by Record. One query finds the candidates and one per person
// reads their rights over all the Tasks, so the cost does not grow with the number of Tasks times the number of people.
func timeReviewers(ctx context.Context, tx pgx.Tx, tasks []string) (map[string][]notifications.Principal, error) {
	rows, err := tx.Query(ctx, `WITH RECURSIVE up(space_id, item_id) AS (
  SELECT space_id, item_id FROM item_locations WHERE item_id=ANY(ARRAY(SELECT unnest($1::text[])::uuid)) AND deleted_root_id IS NULL
  UNION
  SELECT up.space_id, l.parent_item_id FROM up JOIN item_locations l ON l.item_id=up.item_id AND l.space_id=up.space_id
  WHERE l.deleted_root_id IS NULL AND l.parent_item_id IS NOT NULL),
targets AS (SELECT sh.principal_type, sh.principal_id FROM shares sh
  WHERE (sh.target_type='space' AND sh.target_id IN (SELECT space_id FROM up)) OR (sh.target_type='item' AND sh.target_id IN (SELECT item_id FROM up))
  UNION SELECT 'user', user_id FROM space_admins WHERE space_id IN (SELECT space_id FROM up))
SELECT u.id::text, u.user_type::text FROM users u WHERE u.status='active' AND (
  u.id IN (SELECT principal_id FROM targets WHERE principal_type='user')
  OR u.id IN (SELECT user_id FROM group_members WHERE group_id IN (SELECT principal_id FROM targets WHERE principal_type='group')))
ORDER BY u.id`, tasks)
	if err != nil {
		return nil, err
	}
	candidates, err := policy.CollectActors(rows)
	if err != nil {
		return nil, err
	}
	out := map[string][]notifications.Principal{}
	for _, person := range candidates {
		caps, err := policy.ItemsCaps(ctx, tx, person, tasks, nil)
		if err != nil {
			return nil, err
		}
		for _, task := range tasks {
			if caps[task].Contains(policy.RoleCaps(policy.Full)) {
				out[task] = append(out[task], notifications.Principal{Kind: "user", ID: person.ID})
			}
		}
	}
	return out, nil
}

// notifyTimeEntry tells the author about a step someone else took on their entry. Record checks that they still see the Task.
func notifyTimeEntry(ctx context.Context, tx pgx.Tx, batch *notifications.Batch, actor policy.Actor, e timeEntry, kind, note string) error {
	_, err := batch.Record(ctx, tx, actor, kind, notifications.Target{Item: e.ItemID}, note, []notifications.Principal{{Kind: "user", ID: e.UserID}}, nil)
	return err
}
