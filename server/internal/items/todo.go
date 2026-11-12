package items

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MyTodo returns only live, assigned tasks that the actor may currently view.
func MyTodo(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, mode string, filter ListFilter) (Page, error) {
	if mode == "" {
		mode = "active"
	}
	switch mode {
	case "all", "active", "overdue":
	default:
		return Page{}, Invalid("unknown to-do filter")
	}
	direction, comparison := "ASC", ">"
	switch filter.Sort {
	case "", "date":
		filter.Sort = "due_date"
	case "priority", "importance":
	case "date_modified":
		filter.Sort, direction, comparison = "updated_at", "DESC", "<"
	case "date_created":
		filter.Sort, direction, comparison = "created_at", "DESC", "<"
	case "status", "title":
	default:
		return Page{}, Invalid("unknown to-do sort")
	}
	key, cursor, scope, err := validateFilter(actor, "todo:"+mode, false, &filter)
	if err != nil {
		return Page{}, err
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback(ctx)
	visible := make([]listCursor, 0, filter.Limit+1)
	batchSize := max(128, filter.Limit+1)
	for {
		rows, err := tx.Query(ctx, `
WITH candidates AS (
 SELECT i.id,`+key+` AS key
 FROM item_assignees a JOIN items i ON i.id=a.item_id
 JOIN users u ON u.id=a.user_id
 LEFT JOIN statuses st ON st.id=i.status_id
 WHERE a.user_id=$1::uuid AND i.type='task' AND i.deleted_at IS NULL
 AND CASE $2
   WHEN 'active' THEN st.grp='active'
   WHEN 'overdue' THEN st.grp IS DISTINCT FROM 'completed' AND i.due_date < (
     now() AT TIME ZONE COALESCE((SELECT name FROM pg_timezone_names WHERE name=u.timezone),'UTC')
   )::date
   ELSE true
 END
)
SELECT id::text,key FROM candidates
WHERE $3::uuid IS NULL OR ($4::text IS NOT NULL AND key IS NULL) OR (
  (key IS NULL)=($4::text IS NULL) AND
  (COALESCE(key,'') COLLATE "C",id) `+comparison+` (COALESCE($4::text,'') COLLATE "C", $3::uuid)
)
ORDER BY key COLLATE "C" `+direction+` NULLS LAST,id `+direction+` LIMIT $5`,
			actor.ID, mode, cursorID(cursor), cursorKey(cursor), batchSize)
		if err != nil {
			return Page{}, err
		}
		candidates, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (listCursor, error) {
			p := listCursor{Scope: scope}
			err := row.Scan(&p.ID, &p.Key)
			return p, err
		})
		if err != nil {
			return Page{}, err
		}
		ids := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			ids = append(ids, candidate.ID)
		}
		caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
		if err != nil {
			return Page{}, err
		}
		for _, candidate := range candidates {
			if caps[candidate.ID].Has(policy.View) {
				visible = append(visible, candidate)
			}
		}
		if len(visible) > filter.Limit || len(candidates) < batchSize {
			break
		}
		cursor = &candidates[len(candidates)-1]
	}

	page := Page{Items: []json.RawMessage{}}
	if len(visible) > filter.Limit {
		visible = visible[:filter.Limit]
		data, err := json.Marshal(visible[len(visible)-1])
		if err != nil {
			return Page{}, err
		}
		encoded := base64.RawURLEncoding.EncodeToString(data)
		page.NextCursor = &encoded
	}
	ids := make([]string, 0, len(visible))
	for _, item := range visible {
		ids = append(ids, item.ID)
	}
	details, err := readDetails(ctx, tx, actor, ids)
	if err != nil {
		return Page{}, err
	}
	for _, item := range details {
		page.Items = append(page.Items, item.Body)
	}
	return page, tx.Commit(ctx)
}
