// Package events is the transactional outbox for domain events (AUT-01). Emit writes inside the caller's transaction;
// Dispatch hands pending events to a handler at least once, and handlers are responsible for being idempotent per event id.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Event types. The set only grows; payloads only gain keys (ADR-AUT).
const (
	ItemCreated       = "item.created"
	ItemUpdated       = "item.updated"
	ItemStatusChanged = "item.status_changed"
	ItemAssigned      = "item.assigned"
	ItemDueChanged    = "item.due_changed"
	ItemDueReached    = "item.due_reached"
	ItemInactive      = "item.inactive"
	FormSubmitted     = "form.submitted"
	ApprovalDecided   = "approval.decided"
	CommentCreated    = "comment.created"
)

// MaxDepth is how many automation hops one chain may take before it is stopped.
const MaxDepth = 5

type Event struct {
	ID      uuid.UUID
	Type    string
	ItemID  string
	ActorID string
	// OriginSpaceID is the Space the change happened in, when known. Empty for events without it (older events, and emitters not yet
	// migrated). Automation prefers it when it is one of an item's target Spaces.
	OriginSpaceID string
	Payload       map[string]any
	// Set by the dispatcher when it reads the row.
	Seq        int64
	CausedBy   *uuid.UUID
	Depth      int
	Chain      []uuid.UUID
	CreatedAt  time.Time
	Dispatched *time.Time
}

type causeKey struct{}

// Cause says an emission happens because an automation run acted: events emitted under it extend the chain.
type Cause struct {
	Run   uuid.UUID
	Rule  uuid.UUID
	Depth int
	Chain []uuid.UUID
}

func WithCause(ctx context.Context, c Cause) context.Context {
	return context.WithValue(ctx, causeKey{}, c)
}

type originKey struct{}

// WithOrigin marks the Space a block of work runs in: events emitted under ctx carry it as their origin unless they set their own.
func WithOrigin(ctx context.Context, spaceID string) context.Context {
	return context.WithValue(ctx, originKey{}, spaceID)
}

// OriginBulkImport marks events produced by a bulk import. Their internal audit (activity) is still written, but automation,
// webhooks and per-item notifications do not act on them unless a later policy opts in.
const OriginBulkImport = "bulk_import"

type bulkKey struct{}

// WithBulkImport marks ctx as a bulk import; Emit stamps the payload with origin=bulk_import.
func WithBulkImport(ctx context.Context) context.Context {
	return context.WithValue(ctx, bulkKey{}, true)
}

// IsBulkImport reports whether a dispatched event came from a bulk import.
func IsBulkImport(ctx context.Context) bool {
	v, _ := ctx.Value(bulkKey{}).(bool)
	return v
}

// BulkImportEvent reports whether an event was produced by a bulk import.
func BulkImportEvent(e Event) bool {
	origin, _ := e.Payload["origin"].(string)
	return origin == OriginBulkImport
}

// Emit records an event in tx. Call it from the code that changes the data, so both commit or roll back together.
func Emit(ctx context.Context, tx pgx.Tx, e Event) error {
	if e.Type == "" {
		return errors.New("event type is required")
	}
	if e.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		e.ID = id
	}
	payload := e.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	if IsBulkImport(ctx) {
		payload["origin"] = OriginBulkImport
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var item, actor, origin *string
	if e.ItemID != "" {
		item = &e.ItemID
	}
	if e.ActorID != "" {
		actor = &e.ActorID
	}
	if e.OriginSpaceID == "" {
		e.OriginSpaceID, _ = ctx.Value(originKey{}).(string)
	}
	if e.OriginSpaceID != "" {
		origin = &e.OriginSpaceID
	}
	var cause *uuid.UUID
	depth, chain := 0, []uuid.UUID{}
	if c, ok := ctx.Value(causeKey{}).(Cause); ok {
		cause, depth = &c.Run, c.Depth+1
		chain = append(append(chain, c.Chain...), c.Rule)
	}
	_, err = tx.Exec(ctx, `INSERT INTO domain_events(event_id,type,item_id,actor_id,payload,caused_by_run,depth,chain,origin_space_id)
VALUES ($1,$2,$3::uuid,$4::uuid,$5,$6,$7,$8,$9::uuid) ON CONFLICT (event_id) DO NOTHING`, e.ID, e.Type, item, actor, raw, cause, depth, chain, origin)
	return err
}

// Handler processes one event. Returning an error leaves the event pending (it is retried); handlers must tolerate a second delivery.
type Handler func(ctx context.Context, e Event) error

const maxAttempts = 8

// Dispatch claims up to limit pending events, runs the handler for each in order and marks the ones that succeeded.
// A failing event is counted and retried later; after maxAttempts it is parked with its error (dispatched_at set, last_error kept).
// Several dispatchers may run at once: rows are claimed with SKIP LOCKED inside the transaction that marks them.
func Dispatch(ctx context.Context, pool *pgxpool.Pool, handle Handler, limit int) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id,event_id,type,COALESCE(item_id::text,''),COALESCE(actor_id::text,''),payload,caused_by_run,depth,chain,created_at,attempts,COALESCE(origin_space_id::text,'')
FROM domain_events WHERE dispatched_at IS NULL ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return 0, err
	}
	type claimed struct {
		Event
		attempts int
	}
	var batch []claimed
	for rows.Next() {
		var c claimed
		var payload []byte
		if err := rows.Scan(&c.Seq, &c.ID, &c.Type, &c.ItemID, &c.ActorID, &payload, &c.CausedBy, &c.Depth, &c.Chain, &c.CreatedAt, &c.attempts, &c.OriginSpaceID); err != nil {
			rows.Close()
			return 0, err
		}
		if err := json.Unmarshal(payload, &c.Payload); err != nil {
			rows.Close()
			return 0, err
		}
		batch = append(batch, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	done := 0
	for _, c := range batch {
		herr := safely(func() error { return handle(ctx, c.Event) })
		switch {
		case herr == nil:
			_, err = tx.Exec(ctx, "UPDATE domain_events SET dispatched_at=now(), attempts=attempts+1, last_error=NULL WHERE id=$1", c.Seq)
			done++
		case c.attempts+1 >= maxAttempts:
			slog.Error("domain event parked after repeated failures", "event", c.ID, "type", c.Type, "error", herr)
			_, err = tx.Exec(ctx, "UPDATE domain_events SET dispatched_at=now(), attempts=attempts+1, last_error=$2 WHERE id=$1", c.Seq, fmt.Sprint(herr))
		default:
			_, err = tx.Exec(ctx, "UPDATE domain_events SET attempts=attempts+1, last_error=$2 WHERE id=$1", c.Seq, fmt.Sprint(herr))
		}
		if err != nil {
			return done, err
		}
	}
	return done, tx.Commit(ctx)
}

// safely turns a handler panic into an error so one bad event cannot stop the dispatcher.
func safely(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panicked: %v", r)
		}
	}()
	return f()
}
