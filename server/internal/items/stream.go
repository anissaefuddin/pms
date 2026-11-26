package items

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type streamCursor struct {
	At   time.Time `json:"at"`
	Kind string    `json:"kind"`
	ID   string    `json:"id"`
}

type StreamEntry struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Action    string          `json:"action"`
	Changes   json.RawMessage `json:"changes"`
	Snippet   *string         `json:"snippet"`
	Auto      bool            `json:"auto"`
	CreatedAt time.Time       `json:"created_at"`
	Actor     struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"actor"`
	Item struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Type  string `json:"type"`
	} `json:"item"`
}

// Stream lists activity and comments on items the actor can view, newest first (NAV-04).
// mode narrows it to followed items, assigned items or the actor's own conversations.
func Stream(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, mode string, hideAuto bool, limit int, cursor string) ([]StreamEntry, *string, error) {
	switch mode {
	case "", "everything", "followed", "assigned", "conversations":
	default:
		return nil, nil, Invalid("unknown stream filter")
	}
	if limit < 1 || limit > 100 {
		return nil, nil, Invalid("limit must be between 1 and 100")
	}
	var after *streamCursor
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		after = &streamCursor{}
		if err != nil || json.Unmarshal(raw, after) != nil || after.Kind == "" || after.ID == "" {
			return nil, nil, Invalid("invalid cursor")
		}
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	out := []StreamEntry{}
	batch := max(128, limit*2)
	for len(out) <= limit {
		var at *time.Time
		var kind, id *string
		if after != nil {
			at, kind, id = &after.At, &after.Kind, &after.ID
		}
		rows, err := tx.Query(ctx, `
WITH entries AS (
  SELECT a.created_at AS at, 'activity' AS kind, a.id::text AS id, a.item_id, a.actor_id, a.action,
         a.changes, NULL::text AS snippet, a.auto
  FROM activity a
  UNION ALL
  SELECT c.created_at, 'comment', c.id::text, c.item_id, c.author_id, 'comment', '{}'::jsonb,
         left(c.body_text, 200), false
  FROM comments c WHERE c.deleted_at IS NULL
)
SELECT e.at, e.kind, e.id, e.action, e.changes, e.snippet, e.auto,
       u.id::text, concat_ws(' ', u.first_name, u.last_name), i.id::text, i.title, i.type::text
FROM entries e JOIN items i ON i.id=e.item_id AND i.deleted_at IS NULL
JOIN users u ON u.id=e.actor_id
WHERE ($1::timestamptz IS NULL OR (e.at, e.kind, e.id) < ($1, $2::text, $3::text))
  AND NOT ($4 AND e.auto)
  AND CASE $5
    WHEN 'followed' THEN EXISTS (SELECT 1 FROM item_followers f WHERE f.item_id=e.item_id AND f.user_id=$6::uuid)
    WHEN 'assigned' THEN EXISTS (SELECT 1 FROM item_assignees s WHERE s.item_id=e.item_id AND s.user_id=$6::uuid)
    WHEN 'conversations' THEN e.kind='comment' AND EXISTS (
      SELECT 1 FROM comments m WHERE m.item_id=e.item_id AND m.deleted_at IS NULL AND (m.author_id=$6::uuid
        OR EXISTS (SELECT 1 FROM comment_mentions cm WHERE cm.comment_id=m.id AND cm.mention_type='user' AND cm.target_id=$6::uuid)))
    ELSE true END
ORDER BY e.at DESC, e.kind DESC, e.id DESC LIMIT $7`,
			at, kind, id, hideAuto, mode, actor.ID, batch)
		if err != nil {
			return nil, nil, err
		}
		candidates := []StreamEntry{}
		for rows.Next() {
			var e StreamEntry
			var changes []byte
			if err := rows.Scan(&e.CreatedAt, &e.Kind, &e.ID, &e.Action, &changes, &e.Snippet, &e.Auto,
				&e.Actor.ID, &e.Actor.Name, &e.Item.ID, &e.Item.Title, &e.Item.Type); err != nil {
				rows.Close()
				return nil, nil, err
			}
			e.Changes = changes
			candidates = append(candidates, e)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, nil, err
		}
		ids := make([]string, 0, len(candidates))
		for _, e := range candidates {
			ids = append(ids, e.Item.ID)
		}
		caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
		if err != nil {
			return nil, nil, err
		}
		for _, e := range candidates {
			if caps[e.Item.ID].Has(policy.View) {
				out = append(out, e)
			}
		}
		if len(candidates) < batch {
			break
		}
		last := candidates[len(candidates)-1]
		after = &streamCursor{At: last.CreatedAt, Kind: last.Kind, ID: last.ID}
	}
	var next *string
	if len(out) > limit {
		out = out[:limit]
		last := out[limit-1]
		data, _ := json.Marshal(streamCursor{At: last.CreatedAt, Kind: last.Kind, ID: last.ID})
		encoded := base64.RawURLEncoding.EncodeToString(data)
		next = &encoded
	}
	ids, changes := make([]string, len(out)), make([]json.RawMessage, len(out))
	for i, e := range out {
		ids[i], changes[i] = e.Item.ID, e.Changes
	}
	if err := RedactHistory(ctx, tx, actor, ids, changes); err != nil {
		return nil, nil, err
	}
	for i := range out {
		out[i].Changes = changes[i]
	}
	return out, next, tx.Commit(ctx)
}
