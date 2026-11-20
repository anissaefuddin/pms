package httpserver_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// A4: a rule can notice that an item has gone quiet, notify people who can see the item, and move an item into a project.
func TestInactiveItemFiresOncePerQuietSpell(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	e.rule(t, e.editor, "Stale", `{"when":{"type":"item.inactive","days":7},"then":[{"type":"comment","text":"Still waiting?"}]}`)
	item := e.item(t, e.admin, "Quiet", nil)
	id := item["id"].(string)
	f.exec("UPDATE activity SET created_at=now()-interval '10 days' WHERE item_id=$1", id)
	f.exec("UPDATE items SET created_at=now()-interval '10 days' WHERE id=$1", id)

	if n, err := e.engine.EmitInactive(f.ctx); err != nil || n != 1 {
		t.Fatalf("one quiet item: %d %v", n, err)
	}
	e.drain(t)
	if f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1 AND body_text='Still waiting?' AND via='automation_bot'", id) != "1" {
		t.Fatal("the rule commented once, as the bot")
	}
	if n, _ := e.engine.EmitInactive(f.ctx); n != 0 {
		t.Fatalf("the same quiet spell is not reported twice: %d", n)
	}

	// Activity ends the quiet spell; the item is no longer quiet, so nothing is emitted.
	f.want(f.call(e.admin, "PATCH", "/items/"+id, map[string]any{"title": "Moving again"}, "If-Match", `"`+f.queryID("SELECT version::text FROM items WHERE id=$1", id)+`"`), 200, "activity")
	if n, _ := e.engine.EmitInactive(f.ctx); n != 0 {
		t.Fatalf("an item with recent activity is not inactive: %d", n)
	}
}

// A4 / AUT-T17: a mention notifies only people who can see the item; the comment records the mention as a person's would.
func TestMentionNotifiesOnlyPeopleWhoCanSeeTheItem(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	reader := f.person("regular")
	f.share(e.space, "user", reader.id, "read")
	outsider := f.person("regular")
	mention := e.rule(t, e.editor, "Ask", fmt.Sprintf(`{"when":{"type":"item.created"},"then":[{"type":"mention","text":"Please look at {item.title}","user_ids":["%s","%s"]}]}`, reader.id, outsider.id))
	item := e.item(t, e.admin, "Review", nil)
	e.drain(t)

	if e.runs(mention)["done"] != 1 {
		t.Fatalf("the mention ran: %v", e.runs(mention))
	}
	if f.queryID("SELECT count(*)::text FROM comment_mentions m JOIN comments c ON c.id=m.comment_id WHERE c.item_id=$1 AND c.via='automation_bot'", item["id"]) != "2" {
		t.Fatal("both people are mentioned in the comment")
	}
	if f.queryID("SELECT count(*)::text FROM notifications WHERE kind='mention' AND recipient_id=$1", reader.id) != "1" {
		t.Fatal("the reader is notified")
	}
	if f.queryID("SELECT count(*)::text FROM notifications WHERE kind='mention' AND recipient_id=$1", outsider.id) != "0" {
		t.Fatal("a person without access to the item is not notified")
	}
}

// A4: a move puts the item into the chosen project; a target that is not a project in this Space is refused and recorded.
func TestMoveItemIntoAProject(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	alpha := f.call(e.editor, "POST", "/items", map[string]any{"type": "project", "title": "Alpha", "locations": []map[string]any{{"space_id": e.space}}})
	f.want(alpha, 201, "project Alpha")
	beta := f.call(e.editor, "POST", "/items", map[string]any{"type": "project", "title": "Beta", "locations": []map[string]any{{"space_id": e.space}}})
	f.want(beta, 201, "project Beta")
	betaID := beta.body["id"].(string)

	move := e.rule(t, e.editor, "Sort", fmt.Sprintf(`{"when":{"type":"item.created"},"if":[{"field":"item_type","op":"eq","value":"task"}],"then":[{"type":"move","container_id":"%s"}]}`, betaID))
	mover := e.item(t, e.admin, "Mover", nil)
	e.drain(t)
	if got := f.queryID("SELECT parent_item_id::text FROM item_locations WHERE item_id=$1 AND space_id=$2 AND deleted_root_id IS NULL", mover["id"], e.space); got != betaID {
		t.Fatalf("the item is in Beta: %s, runs %v", got, e.runs(move))
	}

	lost := e.rule(t, e.editor, "Lost", fmt.Sprintf(`{"when":{"type":"item.created"},"if":[{"field":"item_type","op":"eq","value":"task"}],"then":[{"type":"move","container_id":"%s"}]}`, uuid.NewString()))
	e.item(t, e.admin, "Lost item", nil)
	e.drain(t)
	var reason string
	f.pool.QueryRow(f.ctx, "SELECT reason FROM automation_runs WHERE rule_id=$1 AND action_index=0 AND status='denied'", lost).Scan(&reason)
	if reason != "not_found" {
		t.Fatalf("a target that is not a project here is refused: %q", reason)
	}
	f.want(f.call(e.editor, "POST", "/spaces/"+e.space+"/automation-rules", map[string]any{"name": "Bad move", "definition": map[string]any{"when": map[string]any{"type": "item.created"}, "if": []any{}, "then": []any{map[string]any{"type": "move"}}}}), 422, "move needs a target")
}

// A3: the dry run explains, step by step, what the rule would do with the item.
func TestDryRunTraceExplainsEachStep(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	hot := e.item(t, e.editor, "Trace me", map[string]any{"importance": "normal"})
	def := `{"when":{"type":"item.created"},"if":[{"field":"importance","op":"eq","value":"high"}],"then":[{"type":"comment","text":"x"}]}`
	got := f.call(e.editor, "POST", "/spaces/"+e.space+"/automation-rules/dry-run", map[string]any{"definition": jsonRaw(def), "item_id": hot["id"]})
	f.want(got, 200, "dry run")
	trace, _ := got.body["trace"].([]any)
	if len(trace) != 3 || trace[0] != "When item.created happens to this item" || trace[2] != "Stops: a condition does not hold, so no action runs" {
		t.Fatalf("the trace names each step: %+v", trace)
	}
}

func jsonRaw(s string) json.RawMessage { return json.RawMessage(s) }
