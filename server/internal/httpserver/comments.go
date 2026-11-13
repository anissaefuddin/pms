package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/anissaefuddin/pms/server/internal/events"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"net/http"
	"strconv"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const commentJSON = `jsonb_build_object('id',c.id,'item_id',c.item_id,'body',c.body,
'author',jsonb_build_object('id',u.id,'name',concat_ws(' ',u.first_name,u.last_name)),
'version',c.version,'created_at',c.created_at,'edited_at',c.edited_at)`

func (s *Server) commentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/items/{itemId}/comments", s.requireUser(s.listComments))
	mux.HandleFunc("POST /api/v1/items/{itemId}/comments", s.requireUser(s.writeComment))
	mux.HandleFunc("PATCH /api/v1/comments/{commentId}", s.requireUser(s.writeComment))
	mux.HandleFunc("DELETE /api/v1/comments/{commentId}", s.requireUser(s.writeComment))
}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 200 {
			writeItemError(w, items.Invalid("limit must be between 1 and 200"))
			return
		}
	}
	var cursor *uuid.UUID
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			writeItemError(w, items.Invalid("invalid comment cursor"))
			return
		}
		cursor = &parsed
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	caps, err := policy.ItemCaps(ctx, tx, actorOf(r), id.String())
	if err != nil {
		writeItemError(w, err)
		return
	}
	if !caps.Has(policy.View) {
		writeItemError(w, items.ErrNotFound)
		return
	}
	rows, err := tx.Query(ctx, `
SELECT c.id,`+commentJSON+` FROM comments c JOIN users u ON u.id=c.author_id
WHERE c.item_id=$1 AND c.deleted_at IS NULL AND ($2::uuid IS NULL OR
 (c.created_at,c.id)<(SELECT created_at,id FROM comments WHERE id=$2 AND item_id=$1))
ORDER BY c.created_at DESC,c.id DESC LIMIT $3`, id, cursor, limit+1)
	if err != nil {
		writeItemError(w, err)
		return
	}
	type row struct {
		id   uuid.UUID
		body json.RawMessage
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var out row
		err := r.Scan(&out.id, &out.body)
		return out, err
	})
	if err != nil {
		writeItemError(w, err)
		return
	}
	var next *string
	if len(list) > limit {
		value := list[limit-1].id.String()
		next = &value
		list = list[:limit]
	}
	result := make([]json.RawMessage, 0, len(list))
	for _, entry := range list {
		result = append(result, entry.body)
	}
	if err := tx.Commit(ctx); err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": result, "next_cursor": next})
}

func (s *Server) writeComment(w http.ResponseWriter, r *http.Request) {
	creating, deleting := r.Method == http.MethodPost, r.Method == http.MethodDelete
	rawID := r.PathValue("commentId")
	if creating {
		rawID = r.PathValue("itemId")
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	version := 0
	if !creating && !deleting {
		var ok bool
		version, ok = ifMatchVersion(w, r)
		if !ok {
			return
		}
	}
	var input struct {
		Body json.RawMessage `json:"body"`
	}
	var plain string
	var mentions []items.Mention
	if !deleting {
		if !decode(w, r, &input) {
			return
		}
		plain, mentions, err = items.CommentText(input.Body)
		if err != nil {
			writeItemError(w, err)
			return
		}
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	itemID := id.String()
	if !creating {
		err = tx.QueryRow(ctx, "SELECT item_id::text FROM comments WHERE id=$1", id).Scan(&itemID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = items.ErrNotFound
		}
		if err != nil {
			writeItemError(w, err)
			return
		}
	}
	batch := s.notificationBatch()
	result, err := mutateComment(ctx, tx, actorOf(r), id, creating, deleting, version, input.Body, plain, mentions, batch)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40001" {
			err = items.ErrConflict
		}
		writeItemError(w, err)
		return
	}
	s.publishNotifications(batch)
	if deleting {
		s.publishItem(r, "item.updated", itemID, itemID, 0)
		w.WriteHeader(204)
		return
	}
	status := 200
	if creating {
		status = 201
		var created struct{ ID string }
		_ = json.Unmarshal(result.Body, &created)
		s.publishItem(r, "comment.created", itemID, created.ID, result.Version)
	} else {
		s.publishItem(r, "item.updated", itemID, itemID, 0)
	}
	writeItem(w, status, result, nil)
}

func mutateComment(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID, creating, deleting bool,
	version int, body json.RawMessage, plain string, mentions []items.Mention, batch *notifications.Batch) (items.Detail, error) {
	itemID := id
	var author string
	var current int
	if !creating {
		err := tx.QueryRow(ctx, "SELECT item_id,author_id::text,version FROM comments WHERE id=$1 AND deleted_at IS NULL", id).Scan(&itemID, &author, &current)
		if errors.Is(err, pgx.ErrNoRows) {
			return items.Detail{}, items.ErrNotFound
		}
		if err != nil {
			return items.Detail{}, err
		}
	}
	caps, err := policy.ItemCaps(ctx, tx, actor, itemID.String())
	if err != nil {
		return items.Detail{}, err
	}
	if !caps.Has(policy.View) {
		return items.Detail{}, items.ErrNotFound
	}
	canWrite := caps.Has(policy.Comment) && (creating || author == actor.ID)
	canModerate := !creating && caps.Contains(policy.RoleCaps(policy.Full))
	if !canWrite && !canModerate {
		return items.Detail{}, items.ErrForbidden
	}
	if deleting {
		_, err = tx.Exec(ctx, "UPDATE comments SET deleted_at=now(),version=version+1 WHERE id=$1", id)
		return items.Detail{}, err
	}
	var previous []notifications.Principal
	if !creating {
		rows, err := tx.Query(ctx, "SELECT mention_type,COALESCE(target_id::text,'') FROM comment_mentions WHERE comment_id=$1", id)
		if err != nil {
			return items.Detail{}, err
		}
		previous, err = pgx.CollectRows(rows, pgx.RowToStructByPos[notifications.Principal])
		if err != nil {
			return items.Detail{}, err
		}
	}
	if creating {
		id, err = uuid.NewV7()
		if err != nil {
			return items.Detail{}, err
		}
		_, err = tx.Exec(ctx, "INSERT INTO comments(id,item_id,author_id,body,body_text) VALUES($1,$2,$3::uuid,$4,$5)", id, itemID, actor.ID, body, plain)
		if err == nil {
			err = events.Emit(ctx, tx, events.Event{Type: events.CommentCreated, ItemID: itemID.String(), ActorID: actor.ID, Payload: map[string]any{"comment_id": id.String()}})
		}
	} else {
		if current != version {
			return items.Detail{}, items.ErrConflict
		}
		_, err = tx.Exec(ctx, "UPDATE comments SET body=$2,body_text=$3,edited_at=now(),version=version+1 WHERE id=$1", id, body, plain)
		if err == nil {
			_, err = tx.Exec(ctx, "DELETE FROM comment_mentions WHERE comment_id=$1", id)
		}
	}
	if err != nil {
		return items.Detail{}, err
	}
	for _, mention := range mentions {
		if mention.ID != nil {
			var exists bool
			query := "SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active')"
			if mention.Kind == "group" {
				query = "SELECT EXISTS(SELECT 1 FROM groups WHERE id=$1)"
			}
			if err = tx.QueryRow(ctx, query, *mention.ID).Scan(&exists); err != nil {
				return items.Detail{}, err
			}
			if !exists {
				return items.Detail{}, items.Invalid("mention target is unavailable")
			}
		}
		if _, err = tx.Exec(ctx, "INSERT INTO comment_mentions(comment_id,mention_type,target_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", id, mention.Kind, mention.ID); err != nil {
			return items.Detail{}, err
		}
	}
	target := notifications.Target{Item: itemID.String()}
	added := items.NewMentions(items.MentionPrincipals(mentions), previous)
	notified, err := batch.Record(ctx, tx, actor, "mention", target, plain, added, nil)
	if err != nil {
		return items.Detail{}, err
	}
	if creating {
		if _, err := batch.Record(ctx, tx, actor, "comment", target, plain, []notifications.Principal{{Kind: "followers"}}, notified); err != nil {
			return items.Detail{}, err
		}
	}
	var out items.Detail
	err = tx.QueryRow(ctx, "SELECT c.version,"+commentJSON+" FROM comments c JOIN users u ON u.id=c.author_id WHERE c.id=$1", id).Scan(&out.Version, &out.Body)
	return out, err
}
