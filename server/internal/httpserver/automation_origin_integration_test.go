package httpserver_test

import (
	"context"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/events"
	"github.com/google/uuid"
)

// emitCreated writes an item.created event with the given origin Space (empty for none), as an emitter that knows its Space would.
func (w wsEnv) emitCreated(t *testing.T, item, origin string) string {
	t.Helper()
	id := uuid.New()
	tx, err := w.f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := events.Emit(context.Background(), tx, events.Event{ID: id, Type: events.ItemCreated, ItemID: item, OriginSpaceID: origin}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return id.String()
}

// Missing origin, one target matched: the owning Space is unambiguous, so the rule runs there (an event written before origins existed).
func TestMissingOriginRunsInTheOwningSpaceWhenUnambiguous(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "Old event", completeDef, w.space, w.other)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")
	w.itemIn(t, w.space, "Only here")
	w.drain(t)
	if f.queryID("SELECT space_id::text FROM automation_runs WHERE rule_id=$1 AND status='done'", id) != w.space {
		t.Fatal("the run executes in the one Space the item is in")
	}
}

// Missing origin, item in both targets: no origin and no single owning Space is ambiguous, so the event is skipped and recorded.
func TestMissingOriginAndTwoSpacesIsSkippedAsAmbiguous(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "Ambiguous", completeDef, w.space, w.other)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")
	w.wsItemInBoth(t, "Two homes, no origin")
	w.drain(t)
	if w.runs(id)["done"] != 0 {
		t.Fatalf("nothing runs when the context is ambiguous: %v", w.runs(id))
	}
	if f.queryID("SELECT reason FROM automation_runs WHERE rule_id=$1 AND status='skipped'", id) != "ambiguous_context" {
		t.Fatal("the skip says why")
	}
	if f.queryID("SELECT count(*)::text FROM automation_runs WHERE rule_id=$1 AND space_id IS NULL AND cardinality(matched_space_ids)=2", id) != "1" {
		t.Fatal("the skip records both matched Spaces and no execution Space")
	}
}

// Invalid origin: the origin is a Space that is not a target. The run is not moved to a target Space to make it run.
func TestInvalidOriginIsSkippedWithoutFallback(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "Foreign origin", completeDef, w.space, w.other)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")
	item := w.wsItemInBoth(t, "Foreign origin event")
	outsider := f.space("Outsider") // a third Space, not a target
	w.emitCreated(t, item, outsider)
	w.drain(t)
	if w.runs(id)["done"] != 0 {
		t.Fatalf("an origin outside the targets runs nothing, in any Space: %v", w.runs(id))
	}
	if f.queryID("SELECT reason FROM automation_runs WHERE rule_id=$1 AND status='skipped' ORDER BY created_at DESC LIMIT 1", id) != "origin_outside_targets" {
		t.Fatal("the skip names the origin problem")
	}
}

// Valid origin in a multi-Space item: the rule runs once, in the origin, and the change it makes carries that origin. A redelivered
// event does not run it again.
func TestValidOriginRunsOnceAndStampsItsChanges(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "Origin wins", completeDef, w.space, w.other)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")
	item := w.wsItemInBoth(t, "Origin event")
	evID := w.emitCreated(t, item, w.other)
	w.drain(t)

	if w.runs(id)["done"] != 1 || f.queryID("SELECT space_id::text FROM automation_runs WHERE rule_id=$1 AND status='done'", id) != w.other {
		t.Fatalf("one run, in the origin: %v", w.runs(id))
	}
	if f.queryID("SELECT origin_space_id::text FROM domain_events WHERE type='item.status_changed' AND caused_by_run IS NOT NULL AND item_id=$1", item) != w.other {
		t.Fatal("the change the rule made carries the Space it ran in as its origin")
	}
	ev := events.Event{ID: uuid.MustParse(evID), Type: events.ItemCreated, ItemID: item, OriginSpaceID: w.other}
	if err := w.engine.Handle(f.ctx, ev); err != nil {
		t.Fatal(err)
	}
	if w.runs(id)["done"] != 1 {
		t.Fatalf("a redelivered event does not run twice: %v", w.runs(id))
	}
}

// Negative: the origin is the primary Space and the owner has lost access there. The run is refused in that Space; the other target
// the owner can still act in is not used instead.
func TestOriginWithoutAccessIsRefusedWithoutFallback(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	id := w.wsRule(t, "No fallback origin", completeDef, w.space, w.other)
	f.want(f.call(w.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")
	item := w.wsItemInBoth(t, "Refused origin")
	f.exec("DELETE FROM shares WHERE target_id=$1 AND principal_id=$2", w.space, w.editor.id)
	w.emitCreated(t, item, w.space)
	w.drain(t)

	if f.queryID("SELECT count(*)::text FROM automation_runs WHERE rule_id=$1 AND status='denied' AND reason='owner_no_access' AND space_id=$2", id, w.space) == "0" {
		t.Fatal("the refusal is recorded in the origin Space with its reason")
	}
	if f.queryID("SELECT count(*)::text FROM automation_runs WHERE rule_id=$1 AND space_id=$2 AND status='done'", id, w.other) != "0" {
		t.Fatal("the run is not moved to the other target")
	}
}

// Origins are stored only when the emitter knows them; an event without one is still a valid event (backward compatibility).
func TestEventOriginIsStoredWhenKnown(t *testing.T) {
	w := newWsEnv(t)
	f := w.f
	item := w.itemIn(t, w.space, "Plain")
	if f.queryID("SELECT COALESCE(origin_space_id::text,'') FROM domain_events WHERE item_id=$1 AND type='item.created'", item) != "" {
		t.Fatal("a change with no known origin has none")
	}
	id := w.emitCreated(t, item, w.space)
	if f.queryID("SELECT origin_space_id::text FROM domain_events WHERE event_id=$1", id) != w.space {
		t.Fatal("the origin is stored with the event")
	}
}
