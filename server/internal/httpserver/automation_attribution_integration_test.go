package httpserver_test

import (
	"testing"
)

// A1 decision: a rule's changes are recorded as the Automation Bot acting for the rule, while the owner stays the actor who was
// authorized. A person's own changes are not labelled.
func TestAutomationChangesAreAttributedToTheBot(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	rule := e.rule(t, e.editor, "Attribute me", `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"importance","value":"high"},{"type":"comment","text":"noted"}]}`)
	item := e.item(t, e.admin, "Attributed", nil)
	e.drain(t)

	id := item["id"]
	if got := f.queryID("SELECT via || '|' || rule_id::text || '|' || actor_id::text FROM activity WHERE item_id=$1 AND action='update' AND via<>''", id); got != "automation_bot|"+rule+"|"+e.editor.id {
		t.Fatalf("the field change is the bot's, for the rule, as its owner: %s", got)
	}
	if got := f.queryID("SELECT via || '|' || rule_id::text || '|' || author_id::text FROM comments WHERE item_id=$1 AND body_text='noted'", id); got != "automation_bot|"+rule+"|"+e.editor.id {
		t.Fatalf("the comment is the bot's, for the rule, as its owner: %s", got)
	}

	// A person's change is not labelled.
	f.want(f.call(e.admin, "PATCH", "/items/"+id.(string), map[string]any{"title": "By hand"}, "If-Match", `"`+f.queryID("SELECT version::text FROM items WHERE id=$1", id)+`"`), 200, "a person edits the item")
	if got := f.queryID("SELECT count(*)::text FROM activity WHERE item_id=$1 AND action='update' AND via='' AND actor_id=$2", id, e.admin.id); got == "0" {
		t.Fatal("a person's own change stays unlabelled and recorded as theirs")
	}
	_ = rule
}
