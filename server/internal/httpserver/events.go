package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/realtime"
	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Server) publishItem(r *http.Request, kind, itemID, entityID string, version int) {
	// After commit, finish routing even if the caller disconnected.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
	defer cancel()
	event := realtime.Event{Type: kind, ItemID: itemID, EntityID: entityID, Version: version}
	rows, err := s.pool.Query(ctx, `SELECT space_id::text,COALESCE(parent_item_id::text,'')
FROM item_locations WHERE item_id=$1::uuid AND
CASE WHEN $2='item.deleted' THEN deleted_root_id=item_id ELSE deleted_root_id IS NULL END`, itemID, kind)
	if err == nil {
		event.Locations, err = pgx.CollectRows(rows, pgx.RowToStructByPos[realtime.Location])
	}
	if err != nil {
		slog.Error("route realtime event", "error", err)
	}
	s.events.Publish(event)
}

// One goroutine per open stream: cap them per user so a runaway client or script cannot exhaust the server.
const maxStreamsPerUser = 10

var userStreams = struct {
	sync.Mutex
	count map[string]int
}{count: map[string]int{}}

func takeStream(user string) bool {
	userStreams.Lock()
	defer userStreams.Unlock()
	if userStreams.count[user] >= maxStreamsPerUser {
		return false
	}
	userStreams.count[user]++
	return true
}

func releaseStream(user string) {
	userStreams.Lock()
	defer userStreams.Unlock()
	if userStreams.count[user]--; userStreams.count[user] <= 0 {
		delete(userStreams.count, user)
	}
}

func (s *Server) streamEvents(w http.ResponseWriter, r *http.Request) {
	if user := actorOf(r).ID; !takeStream(user) {
		writeProblem(w, http.StatusTooManyRequests, "too_many_streams", "too many open event streams")
		return
	} else {
		defer releaseStream(user)
	}
	s.sseConnections.Inc()
	defer s.sseConnections.Dec()
	controller := http.NewResponseController(w)
	cookie, _ := r.Cookie("pms_session") // requireUser checked the cookie.
	token := security.TokenHash(cookie.Value)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")

	var cursor *uint64
	if previous := r.Header.Get("Last-Event-ID"); previous != "" {
		epoch, offset, _ := strings.Cut(previous, ":")
		id, err := strconv.ParseUint(offset, 10, 64)
		if epoch != s.eventEpoch || err != nil {
			id = ^uint64(0)
		}
		cursor = &id
	}
	send := func(id uint64, kind string, data any) bool {
		body, err := json.Marshal(data)
		if err != nil {
			return false
		}
		return flushEvent(w, controller, fmt.Sprintf("id: %s:%d\nevent: %s\ndata: %s\n\n", s.eventEpoch, id, kind, body))
	}
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	if cursor != nil && !flushEvent(w, controller, ": connected\n\n") {
		return
	}
	for {
		events, head, resync, changed := s.events.After(cursor)
		if cursor == nil && !send(head, "ready", nil) {
			return
		}
		if resync && !send(head, "resync", map[string]string{"type": "resync", "id": uuid.Nil.String()}) {
			return
		}
		cursor = &head
		for _, event := range events {
			if event.RecipientID != "" && event.RecipientID != actorOf(r).ID {
				continue
			}
			actor, ok := s.streamActor(r, token)
			if !ok {
				return
			}
			if event.RecipientID != "" {
				visible, err := s.canStreamNotification(r, actor, event.EntityID)
				if err != nil {
					return
				}
				if visible && !send(event.ID, event.Type, map[string]string{"type": event.Type, "id": event.EntityID}) {
					return
				}
				continue
			}
			if len(event.AffectedItems) > 0 {
				data, err := s.scheduleEventData(r.Context(), actor, event)
				if err != nil || (data != nil && !send(event.ID, event.Type, data)) {
					return
				}
				continue
			}
			containers, visible, err := s.eventContainers(r, actor, event)
			if err != nil {
				return // reconnect and replay rather than dropping this event
			}
			if !visible {
				continue
			}
			data := map[string]any{"type": event.Type, "id": event.EntityID, "container_ids": containers}
			if event.ItemID != event.EntityID {
				data["item_id"] = event.ItemID
			}
			if event.Version > 0 {
				data["version"] = event.Version
			}
			if !send(event.ID, event.Type, data) {
				return
			}
		}
		select {
		case <-r.Context().Done():
			return
		case <-changed:
		case <-heartbeat.C:
			if _, ok := s.streamActor(r, token); !ok {
				return
			}
			if !flushEvent(w, controller, ": heartbeat\n\n") {
				return
			}
		}
	}
}

func flushEvent(w http.ResponseWriter, controller *http.ResponseController, message string) bool {
	_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := fmt.Fprint(w, message); err != nil || controller.Flush() != nil {
		return false
	}
	_ = controller.SetWriteDeadline(time.Time{})
	return true
}

func (s *Server) streamActor(r *http.Request, token []byte) (policy.Actor, bool) {
	var actor policy.Actor
	err := s.pool.QueryRow(r.Context(), `SELECT u.id::text,u.user_type::text FROM sessions s JOIN users u ON u.id=s.user_id
WHERE s.token_hash=$1 AND u.status='active' AND s.expires_at>now() AND s.absolute_expires_at>now()`, token).Scan(&actor.ID, &actor.Type)
	return actor, err == nil
}

func (s *Server) eventContainers(r *http.Request, actor policy.Actor, event realtime.Event) ([]string, bool, error) {
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)
	visible := false
	if event.Type != "item.deleted" {
		caps, err := policy.ItemCaps(ctx, tx, actor, event.ItemID)
		if err != nil || !caps.Has(policy.View) {
			return nil, false, err
		}
		visible = true
	}
	containers := []string{}
	seen := map[string]bool{}
	for _, location := range event.Locations {
		if event.Type == "item.deleted" {
			caps, err := policy.DeletedItemCapsInSpace(ctx, tx, actor, event.ItemID, location.SpaceID)
			if err != nil {
				return nil, false, err
			}
			if !caps.Has(policy.View) {
				continue
			}
			visible = true
		}
		id := location.ParentID
		var caps policy.Caps
		if id == "" {
			id = location.SpaceID
			space, _, err := policy.SpaceAccessFor(ctx, tx, actor, id)
			if err != nil {
				return nil, false, err
			}
			caps = space.Caps
		} else {
			caps, err = policy.ItemCapsInSpace(ctx, tx, actor, id, location.SpaceID)
			if err != nil {
				return nil, false, err
			}
		}
		if caps.Has(policy.View) && !seen[id] {
			visible = true
			seen[id] = true
			containers = append(containers, id)
		}
	}
	return containers, visible, tx.Commit(ctx)
}
