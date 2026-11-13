package items

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/anissaefuddin/pms/server/internal/events"
	"github.com/jackc/pgx/v5"
)

// emitChange writes the domain events (AUT-01) for one recorded change, in the same transaction as the change.
// Every path that records activity (create, patch, location, delete, restore) passes through here.
func emitChange(ctx context.Context, tx pgx.Tx, id, actor, action string, before, after json.RawMessage) error {
	var was, now map[string]any
	_ = json.Unmarshal(before, &was)
	_ = json.Unmarshal(after, &now)
	emit := func(kind string, payload map[string]any) error {
		return events.Emit(ctx, tx, events.Event{Type: kind, ItemID: id, ActorID: actor, Payload: payload})
	}
	if action == "create" {
		return emit(events.ItemCreated, map[string]any{"type": now["type"], "status_id": now["status_id"]})
	}
	if action != "update" {
		return nil
	}
	changed := []string{}
	for key, value := range now {
		if before, ok := was[key]; !ok || !sameJSON(before, value) {
			changed = append(changed, key)
		}
	}
	for key := range was {
		if _, ok := now[key]; !ok && !slices.Contains(changed, key) {
			changed = append(changed, key)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	slices.Sort(changed)
	if err := emit(events.ItemUpdated, map[string]any{"changed": changed}); err != nil {
		return err
	}
	if slices.Contains(changed, "status_id") {
		if err := emit(events.ItemStatusChanged, map[string]any{"from": was["status_id"], "to": now["status_id"]}); err != nil {
			return err
		}
	}
	if slices.Contains(changed, "assignee_ids") {
		if added := added(was["assignee_ids"], now["assignee_ids"]); len(added) > 0 {
			if err := emit(events.ItemAssigned, map[string]any{"user_ids": added}); err != nil {
				return err
			}
		}
	}
	if slices.Contains(changed, "due_date") {
		return emit(events.ItemDueChanged, map[string]any{"from": was["due_date"], "to": now["due_date"]})
	}
	return nil
}

func sameJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// added lists the ids in next that were not in previous.
func added(previous, next any) []string {
	had := map[string]bool{}
	if list, ok := previous.([]any); ok {
		for _, v := range list {
			if s, ok := v.(string); ok {
				had[s] = true
			}
		}
	}
	out := []string{}
	if list, ok := next.([]any); ok {
		for _, v := range list {
			if s, ok := v.(string); ok && !had[s] {
				out = append(out, s)
			}
		}
	}
	return out
}
