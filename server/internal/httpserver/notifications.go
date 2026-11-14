package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/realtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Server) notificationBatch() *notifications.Batch { return &notifications.Batch{Queue: s.mail} }
func (s *Server) publishNotifications(batch *notifications.Batch) {
	for _, n := range batch.Deliveries {
		s.events.Publish(realtime.Event{Type: "notification.created", EntityID: n.ID, RecipientID: n.Recipient})
	}
}

const notificationJSON = `jsonb_build_object('id',n.id,'kind',n.kind,'created_at',n.created_at,
'read_at',n.read_at,'archived_at',n.archived_at,'snippet',n.snippet,'item_id',n.item_id,
'item_title',i.title,'space_id',n.space_id,'space_title',s.name,
'actor',CASE WHEN u.id IS NOT NULL THEN jsonb_build_object('id',u.id,'name',concat_ws(' ',u.first_name,u.last_name)) END,
'recipient',CASE WHEN ru.id IS NOT NULL THEN jsonb_build_object('id',ru.id,'name',concat_ws(' ',ru.first_name,ru.last_name)) END)`

const notificationJoins = ` FROM notifications n LEFT JOIN items i ON i.id=n.item_id
LEFT JOIN spaces s ON s.id=n.space_id LEFT JOIN users u ON u.id=n.actor_id LEFT JOIN users ru ON ru.id=n.recipient_id`

type inboxRow struct {
	ID     string
	Item   string
	Space  string
	Unread bool
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 50
	var err error
	if q.Has("limit") {
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil || limit < 1 || limit > 200 {
			writeItemError(w, items.Invalid("limit must be between 1 and 200"))
			return
		}
	}
	cursor := q.Get("cursor")
	if cursor != "" {
		id, err := uuid.Parse(cursor)
		if err != nil {
			writeItemError(w, items.Invalid("invalid notification cursor"))
			return
		}
		cursor = id.String()
	}
	unreadOnly := false
	if q.Has("unread_only") {
		unreadOnly, err = strconv.ParseBool(q.Get("unread_only"))
		if err != nil {
			writeItemError(w, items.Invalid("unread_only must be a boolean"))
			return
		}
	}
	// box: incoming (default), archive (own archived) or sent (notifications the caller triggered for others).
	filter := "n.recipient_id=$1::uuid AND n.archived_at IS NULL"
	incoming := false
	switch q.Get("box") {
	case "", "incoming":
		incoming = true
	case "archive":
		filter = "n.recipient_id=$1::uuid AND n.archived_at IS NOT NULL"
	case "sent":
		filter = "n.actor_id=$1::uuid AND n.recipient_id<>$1::uuid"
	default:
		writeItemError(w, items.Invalid("box must be incoming, archive or sent"))
		return
	}
	ctx := r.Context()
	actor := actorOf(r)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	// Visibility is always checked for the caller, so Sent never reveals items the sender can no longer see.
	// shortcut: scan active Inbox metadata to count only accessible entries. Move this into a policy-filtered query when Inbox volume grows.
	rows, err := tx.Query(ctx, `SELECT n.id::text,COALESCE(n.item_id::text,''),COALESCE(n.space_id::text,''),n.read_at IS NULL FROM notifications n WHERE `+filter+` ORDER BY n.id DESC`, actor.ID)
	if err != nil {
		writeItemError(w, err)
		return
	}
	inbox, err := pgx.CollectRows(rows, pgx.RowToStructByPos[inboxRow])
	if err != nil {
		writeItemError(w, err)
		return
	}
	ids := make([]string, 0, len(inbox))
	seen := map[string]bool{}
	for _, n := range inbox {
		if n.Item != "" && !seen[n.Item] {
			ids = append(ids, n.Item)
			seen[n.Item] = true
		}
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		writeItemError(w, err)
		return
	}
	spaceVisible := map[string]bool{}
	unread := 0
	pageIDs := []string{}
	var next *string
	var last string
	for _, n := range inbox {
		visible := caps[n.Item].Has(policy.View)
		if n.Space != "" {
			if _, known := spaceVisible[n.Space]; !known {
				canView, err := notifications.CanView(ctx, tx, actor, notifications.Target{Space: n.Space})
				if err != nil {
					writeItemError(w, err)
					return
				}
				spaceVisible[n.Space] = canView
			}
			visible = spaceVisible[n.Space]
		}
		if !visible {
			continue
		}
		if n.Unread && incoming {
			unread++
		}
		if (unreadOnly && !n.Unread) || (cursor != "" && n.ID >= cursor) {
			continue
		}
		if len(pageIDs) == limit {
			next = &last
			continue
		}
		pageIDs = append(pageIDs, n.ID)
		last = n.ID
	}
	out := []json.RawMessage{}
	if len(pageIDs) != 0 {
		rows, err := tx.Query(ctx, "SELECT "+notificationJSON+notificationJoins+" WHERE n.id=ANY(ARRAY(SELECT unnest($1::text[])::uuid)) ORDER BY n.id DESC", pageIDs)
		if err != nil {
			writeItemError(w, err)
			return
		}
		out, err = pgx.CollectRows(rows, pgx.RowTo[json.RawMessage])
		if err != nil {
			writeItemError(w, err)
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": out, "next_cursor": next, "unread_count": unread})
}

func (s *Server) updateNotification(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("notificationId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	var input struct{ Read, Archived *bool }
	if !decode(w, r, &input) {
		return
	}
	if input.Read == nil && input.Archived == nil {
		writeItemError(w, items.Invalid("read or archived is required"))
		return
	}
	ctx := r.Context()
	actor := actorOf(r)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var target notifications.Target
	err = tx.QueryRow(ctx, `SELECT COALESCE(item_id::text,''),COALESCE(space_id::text,'')
FROM notifications WHERE id=$1 AND recipient_id=$2::uuid FOR UPDATE`, id, actor.ID).Scan(&target.Item, &target.Space)
	if errors.Is(err, pgx.ErrNoRows) {
		err = items.ErrNotFound
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	visible, err := notifications.CanView(ctx, tx, actor, target)
	if err != nil {
		writeItemError(w, err)
		return
	}
	if !visible {
		writeItemError(w, items.ErrNotFound)
		return
	}
	_, err = tx.Exec(ctx, `UPDATE notifications SET
read_at=CASE WHEN $2::bool IS NULL THEN read_at WHEN $2 THEN COALESCE(read_at,now()) END,
archived_at=CASE WHEN $3::bool IS NULL THEN archived_at WHEN $3 THEN COALESCE(archived_at,now()) END
WHERE id=$1`, id, input.Read, input.Archived)
	if err != nil {
		writeItemError(w, err)
		return
	}
	var out json.RawMessage
	err = tx.QueryRow(ctx, "SELECT "+notificationJSON+notificationJoins+" WHERE n.id=$1", id).Scan(&out)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) bulkNotifications(w http.ResponseWriter, r *http.Request, archive bool) {
	query := "UPDATE notifications SET read_at=now() WHERE recipient_id=$1::uuid AND archived_at IS NULL AND read_at IS NULL"
	if archive {
		query = "UPDATE notifications SET archived_at=now() WHERE recipient_id=$1::uuid AND archived_at IS NULL"
	}
	if _, err := s.pool.Exec(r.Context(), query, actorOf(r).ID); err != nil {
		writeItemError(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) canStreamNotification(r *http.Request, actor policy.Actor, id string) (bool, error) {
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var target notifications.Target
	err = tx.QueryRow(ctx, `SELECT COALESCE(item_id::text,''),COALESCE(space_id::text,'')
FROM notifications WHERE id=$1::uuid AND recipient_id=$2::uuid AND archived_at IS NULL`, id, actor.ID).Scan(&target.Item, &target.Space)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	visible, err := notifications.CanView(ctx, tx, actor, target)
	if err != nil {
		return false, err
	}
	return visible, tx.Commit(ctx)
}
