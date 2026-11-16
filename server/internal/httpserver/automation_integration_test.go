package httpserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/httpserver"
	"github.com/google/uuid"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/automation"
	"github.com/anissaefuddin/pms/server/internal/events"
)

type autoEnv struct {
	f      *fixture
	engine *automation.Engine
	admin  *person
	editor *person
	space  string
}

func newAutoEnv(t *testing.T) *autoEnv {
	t.Helper()
	f := newFixture(t)
	e := &autoEnv{f: f, engine: automation.New(f.pool), admin: f.person("admin"), editor: f.person("regular"), space: f.space("Auto")}
	f.share(e.space, "user", e.editor.id, "full")
	return e
}

func (e *autoEnv) drain(t *testing.T) {
	t.Helper()
	for range 40 {
		n, err := e.engine.Dispatch(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			return
		}
	}
	t.Fatal("events never settled")
}

func (e *autoEnv) rule(t *testing.T, owner *person, name, definition string) string {
	t.Helper()
	made := e.f.call(owner, "POST", "/spaces/"+e.space+"/automation-rules", map[string]any{"name": name, "definition": json.RawMessage(definition)})
	e.f.want(made, 201, "create rule "+name)
	id := made.body["id"].(string)
	e.f.want(e.f.call(owner, "POST", "/automation-rules/"+id+"/publish", nil), 200, "publish "+name)
	e.f.want(e.f.call(owner, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable "+name)
	return id
}

func (e *autoEnv) item(t *testing.T, owner *person, title string, extra map[string]any) map[string]any {
	t.Helper()
	body := map[string]any{"type": "task", "title": title, "locations": []map[string]any{{"space_id": e.space}}}
	for k, v := range extra {
		body[k] = v
	}
	got := e.f.call(owner, "POST", "/items", body)
	e.f.want(got, 201, "item "+title)
	return got.body
}

func (e *autoEnv) runs(rule string) map[string]int {
	out := map[string]int{}
	rows, err := e.f.pool.Query(e.f.ctx, "SELECT status, count(*) FROM automation_runs WHERE rule_id=$1 GROUP BY status", rule)
	if err != nil {
		e.f.t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int
		_ = rows.Scan(&status, &n)
		out[status] = n
	}
	return out
}

func importanceOf(e *autoEnv, id string) string {
	return e.f.queryID("SELECT importance::text FROM items WHERE id=$1", id)
}

// S9.1.1: events are written with the change and only with it; the dispatcher hands each to a handler once, retries failures,
// and several dispatchers never take the same event.
func TestOutboxEmitsWithTheChangeAndDispatchesReliably(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	count := func(kind string) int {
		n, _ := strconv.Atoi(f.queryID("SELECT count(*)::text FROM domain_events WHERE type=$1", kind))
		return n
	}
	item := e.item(t, e.editor, "Outbox", nil)
	id := item["id"].(string)
	if count(events.ItemCreated) != 1 {
		t.Fatalf("item.created events: %d", count(events.ItemCreated))
	}
	version := 1
	patch := func(body map[string]any) reply {
		got := f.call(e.editor, "PATCH", "/items/"+id, body, "If-Match", `"`+strconv.Itoa(version)+`"`)
		if got.status == 200 {
			version++
		}
		return got
	}
	done := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' LIMIT 1")
	f.want(patch(map[string]any{"status_id": done, "assignee_ids": []string{e.editor.id}, "due_date": "2026-12-01"}), 200, "change status, assignee and date")
	for _, kind := range []string{events.ItemUpdated, events.ItemStatusChanged, events.ItemAssigned, events.ItemDueChanged} {
		if count(kind) != 1 {
			t.Errorf("%s events: %d, want 1", kind, count(kind))
		}
	}
	f.want(f.call(e.editor, "POST", "/items/"+id+"/comments", map[string]any{"body": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "hi"}}}}}}), 201, "comment")
	if count(events.CommentCreated) != 1 {
		t.Errorf("comment events: %d", count(events.CommentCreated))
	}
	// A refused change leaves no event behind: the guard rolls the whole transaction back.
	before := f.queryID("SELECT count(*)::text FROM domain_events")
	f.exec("UPDATE statuses SET required_fields='{effort_minutes}' WHERE grp='deferred'")
	deferred := f.queryID("SELECT id::text FROM statuses WHERE grp='deferred' LIMIT 1")
	f.want(patch(map[string]any{"status_id": deferred}), 422, "guard refuses")
	if after := f.queryID("SELECT count(*)::text FROM domain_events"); after != before {
		t.Fatalf("a rolled-back change emitted events: %s -> %s", before, after)
	}

	// Dispatch: a failing handler leaves the events pending, and a later good one handles each exactly once.
	var seen sync.Map
	failing := func(context.Context, events.Event) error { return fmt.Errorf("boom") }
	if n, err := events.Dispatch(f.ctx, f.pool, failing, 100); err != nil || n != 0 {
		t.Fatalf("failing handler: %d %v", n, err)
	}
	pending := f.queryID("SELECT count(*)::text FROM domain_events WHERE dispatched_at IS NULL")
	if pending == "0" || f.queryID("SELECT min(attempts)::text FROM domain_events WHERE dispatched_at IS NULL") != "1" {
		t.Fatalf("failed events must stay pending with the attempt counted: %s", pending)
	}
	var calls atomic.Int32
	good := func(_ context.Context, ev events.Event) error {
		if _, dup := seen.LoadOrStore(ev.ID, true); dup {
			t.Errorf("event %s handled twice", ev.ID)
		}
		calls.Add(1)
		return nil
	}
	// Two dispatchers at once never take the same event.
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = events.Dispatch(f.ctx, f.pool, good, 3) }()
	}
	wg.Wait()
	for {
		n, err := events.Dispatch(f.ctx, f.pool, good, 100)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	if f.queryID("SELECT count(*)::text FROM domain_events WHERE dispatched_at IS NULL") != "0" || int(calls.Load()) != mustInt(before) {
		t.Fatalf("handled %d of %s events", calls.Load(), before)
	}
	// A panic in a handler is an error for that event, not a crash of the dispatcher; after eight attempts it is parked with its error.
	f.exec("INSERT INTO domain_events(event_id,type,item_id,payload) VALUES (gen_random_uuid(),'item.updated',$1,'{}')", id)
	for range 8 {
		_, _ = events.Dispatch(f.ctx, f.pool, func(context.Context, events.Event) error { panic("bad handler") }, 10)
	}
	if f.queryID("SELECT count(*)::text FROM domain_events WHERE dispatched_at IS NOT NULL AND last_error LIKE '%panicked%'") != "1" {
		t.Fatal("a repeatedly failing event must be parked with its error")
	}
}

func mustInt(s string) int { n, _ := strconv.Atoi(s); return n }

// S9.1.1 + S9.1.2: the effect of a rule happens once however often its event is delivered, including after a crash between
// the effect and the "dispatched" mark.
func TestRuleEffectIsExactlyOnceOnRedelivery(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	rule := e.rule(t, e.editor, "Escalate new bugs", `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"importance","value":"high"},{"type":"comment","text":"Escalated {item.title}"}]}`)
	item := e.item(t, e.editor, "Crash on save", nil)
	id := item["id"].(string)
	e.drain(t)
	if importanceOf(e, id) != "high" {
		t.Fatal("the rule did not run")
	}
	activity := f.queryID("SELECT count(*)::text FROM activity WHERE item_id=$1", id)
	// Simulate a crash after the effects but before the mark: the same events come back.
	f.exec("UPDATE domain_events SET dispatched_at=NULL WHERE item_id=$1", id)
	e.drain(t)
	f.exec("UPDATE domain_events SET dispatched_at=NULL WHERE item_id=$1", id)
	e.drain(t)
	if got := f.queryID("SELECT count(*)::text FROM activity WHERE item_id=$1 AND action='update'", id); got != "1" {
		t.Fatalf("the field was changed %s times; activity was %s", got, activity)
	}
	if got := f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1 AND body_text='Escalated Crash on save'", id); got != "1" {
		t.Fatalf("the comment was posted %s times", got)
	}
	if runs := e.runs(rule); runs["done"] != 2 {
		t.Fatalf("runs: %v", runs)
	}
}

var _ = time.Second

// S9.1.2: rules are created as drafts, published as immutable versions, validated against a closed schema, and need Edit on their Space.
func TestRuleLifecycleVersionsAndValidation(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	reader, outsider := f.person("regular"), f.person("regular")
	f.share(e.space, "user", reader.id, "read")
	def := `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"importance","value":"high"}]}`
	body := func(def string) map[string]any {
		return map[string]any{"name": "Rule", "definition": json.RawMessage(def)}
	}
	f.want(f.call(reader, "POST", "/spaces/"+e.space+"/automation-rules", body(def)), 403, "a reader cannot create rules")
	f.want(f.call(outsider, "POST", "/spaces/"+e.space+"/automation-rules", body(def)), 404, "no access is 404")
	f.want(f.call(nil, "POST", "/spaces/"+e.space+"/automation-rules", body(def)), 401, "anonymous")
	for name, bad := range map[string]string{
		"unknown trigger": `{"when":{"type":"item.deleted"},"then":[{"type":"comment","text":"x"}]}`,
		"no actions":      `{"when":{"type":"item.created"},"then":[]}`,
		"unknown key":     `{"when":{"type":"item.created"},"then":[{"type":"comment","text":"x"}],"run":"rm"}`,
		"missing status":  `{"when":{"type":"item.status_changed","to_status_id":"00000000-0000-4000-8000-000000000001"},"then":[{"type":"comment","text":"x"}]}`,
		"inactive user":   `{"when":{"type":"item.created"},"then":[{"type":"assign","user_ids":["00000000-0000-4000-8000-000000000002"]}]}`,
		"unknown type":    `{"when":{"type":"item.created"},"then":[{"type":"create_item","title":"x","type_key":"nope"}]}`,
	} {
		if got := f.call(e.editor, "POST", "/spaces/"+e.space+"/automation-rules", body(bad)); got.status != 422 {
			t.Fatalf("%s accepted: %+v", name, got)
		}
	}
	f.want(f.call(e.editor, "POST", "/spaces/"+e.space+"/automation-rules", map[string]any{"name": " ", "definition": json.RawMessage(def)}), 422, "blank name")
	f.want(f.call(e.editor, "POST", "/spaces/"+e.space+"/automation-rules", map[string]any{"name": "x", "container_id": e.item(t, e.editor, "task", nil)["id"], "definition": json.RawMessage(def)}), 422, "container must be a project or folder")

	made := f.call(e.editor, "POST", "/spaces/"+e.space+"/automation-rules", body(def))
	f.want(made, 201, "create")
	id := made.body["id"].(string)
	f.want(f.call(e.editor, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 422, "cannot enable before publishing")
	f.want(f.call(e.editor, "POST", "/automation-rules/"+id+"/publish", nil), 200, "publish v1")
	f.want(f.call(e.editor, "POST", "/automation-rules/"+id+"/publish", nil), 422, "nothing left to publish")
	// Editing starts a new draft; the published version is untouched until the draft is published.
	v2 := `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"importance","value":"low"}]}`
	f.want(f.call(e.editor, "PUT", "/automation-rules/"+id+"/draft", map[string]any{"definition": json.RawMessage(v2)}), 200, "draft v2")
	got := f.call(e.editor, "GET", "/automation-rules/"+id, nil)
	versions := got.body["versions"].([]any)
	if len(versions) != 2 || versions[0].(map[string]any)["status"] != "draft" || versions[1].(map[string]any)["status"] != "published" {
		t.Fatalf("versions: %+v", versions)
	}
	f.want(f.call(e.editor, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": true}), 200, "enable")
	e.item(t, e.editor, "Under v1", nil)
	e.drain(t)
	if n := f.queryID("SELECT count(*)::text FROM items WHERE title='Under v1' AND importance='high'"); n != "1" {
		t.Fatal("the published version must run, not the draft")
	}
	f.want(f.call(e.editor, "POST", "/automation-rules/"+id+"/publish", nil), 200, "publish v2")
	if f.queryID("SELECT count(*)::text FROM automation_rule_versions WHERE rule_id=$1 AND status='retired'", id) != "1" {
		t.Fatal("the previous version must be retired")
	}
	e.item(t, e.editor, "Under v2", nil)
	e.drain(t)
	if f.queryID("SELECT importance::text FROM items WHERE title='Under v2'") != "low" {
		t.Fatal("the new version must run after publishing")
	}
	// Runs record which version acted.
	if n := f.queryID("SELECT count(DISTINCT rule_version_id)::text FROM automation_runs WHERE rule_id=$1", id); n != "2" {
		t.Fatalf("runs by version: %s", n)
	}
	// Only the owner or an admin may change a rule; a reader or outsider cannot even read it.
	other := f.person("regular")
	f.share(e.space, "user", other.id, "full")
	f.want(f.call(other, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": false}), 403, "another editor is not the owner")
	f.want(f.call(e.admin, "PATCH", "/automation-rules/"+id, map[string]any{"enabled": false}), 200, "an admin may switch it off")
	f.want(f.call(reader, "GET", "/automation-rules/"+id, nil), 403, "reader")
	f.want(f.call(outsider, "GET", "/automation-rules/"+id, nil), 404, "outsider")
	f.want(f.call(e.editor, "DELETE", "/automation-rules/"+id, nil), 204, "owner deletes")
	f.want(f.call(e.editor, "GET", "/automation-rules/"+id, nil), 404, "gone")
}

// Every trigger and every action does what it says, and conditions decide whether a rule acts.
func TestTriggersConditionsAndActions(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	watcher := f.person("regular")
	f.share(e.space, "user", watcher.id, "read")
	stat := func(group string) string {
		return f.queryID("SELECT id::text FROM statuses WHERE grp=$1::status_group ORDER BY position LIMIT 1", group)
	}
	done := stat("completed")
	bugType := f.call(e.admin, "POST", "/item-types", map[string]any{"key": "bug", "name": "Bug", "base": "task"})
	f.want(bugType, 201, "bug type")

	// status trigger + condition + set_field / comment / notify
	r1 := e.rule(t, e.editor, "On done", `{"when":{"type":"item.status_changed","to_status_id":"`+done+`"},"if":[{"field":"importance","op":"eq","value":"high"}],
 "then":[{"type":"comment","text":"{item.title} is done"},{"type":"notify","user_ids":["`+watcher.id+`"],"message":"{item.title} finished"},{"type":"set_field","field":"due_date","value":"2026-12-31"}]}`)
	hot := e.item(t, e.editor, "Hot", map[string]any{"importance": "high"})
	cold := e.item(t, e.editor, "Cold", nil)
	for _, it := range []map[string]any{hot, cold} {
		f.want(f.call(e.editor, "PATCH", "/items/"+it["id"].(string), map[string]any{"status_id": done}, "If-Match", `"1"`), 200, "complete")
	}
	e.drain(t)
	if f.queryID("SELECT due_date::text FROM items WHERE title='Hot'") != "2026-12-31" || f.queryID("SELECT count(*)::text FROM items WHERE title='Cold' AND due_date IS NOT NULL") != "0" {
		t.Fatal("the condition must separate Hot from Cold")
	}
	if f.queryID("SELECT count(*)::text FROM comments WHERE body_text='Hot is done'") != "1" || f.queryID("SELECT count(*)::text FROM comments WHERE body_text='Cold is done'") != "0" {
		t.Fatal("comment action")
	}
	if f.queryID("SELECT count(*)::text FROM notifications WHERE kind='automation' AND recipient_id=$1 AND snippet='Hot finished'", watcher.id) != "1" {
		t.Fatal("notify action")
	}
	if runs := e.runs(r1); runs["skipped"] != 1 || runs["done"] != 3 {
		t.Fatalf("runs: %v", runs)
	}

	// assigned trigger + assign action; created trigger limited to a type + create_item
	r2 := e.rule(t, e.editor, "Pair up", `{"when":{"type":"item.assigned","user_id":"`+watcher.id+`"},"then":[{"type":"assign","user_ids":["`+e.admin.id+`"]}]}`)
	f.share(e.space, "user", e.admin.id, "full")
	pair := e.item(t, e.editor, "Pair", nil)
	f.want(f.call(e.editor, "PATCH", "/items/"+pair["id"].(string), map[string]any{"assignee_ids": []string{watcher.id}}, "If-Match", `"1"`), 200, "assign the watcher")
	e.drain(t)
	if f.queryID("SELECT count(*)::text FROM item_assignees WHERE item_id=$1", pair["id"]) != "2" {
		t.Fatalf("assign action: %v", e.runs(r2))
	}
	e.rule(t, e.editor, "Bug triage", `{"when":{"type":"item.created","item_type":"bug"},"then":[{"type":"create_item","title":"Triage {item.title}","kind":"task"}]}`)
	e.item(t, e.editor, "Plain", nil)
	e.item(t, e.editor, "Login broken", map[string]any{"type_key": "bug"})
	e.drain(t)
	if f.queryID("SELECT count(*)::text FROM items WHERE title='Triage Login broken'") != "1" || f.queryID("SELECT count(*)::text FROM items WHERE title LIKE 'Triage Plain%'") != "0" {
		t.Fatal("created trigger with item type and create_item")
	}
	if f.queryID("SELECT count(*)::text FROM item_locations l JOIN items i ON i.id=l.item_id WHERE i.title='Triage Login broken' AND l.space_id=$1", e.space) != "1" {
		t.Fatal("the new item belongs in the rule's Space")
	}

	// due trigger: the scheduler emits one event per item and day, and runs twice emit nothing more
	in3 := time.Date(2026, 12, 4, 0, 0, 0, 0, time.UTC)
	e.engine.Now = func() time.Time { return time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC) }
	e.rule(t, e.editor, "Due soon", `{"when":{"type":"item.due_reached","days_before":3},"then":[{"type":"set_field","field":"importance","value":"high"}]}`)
	soon := e.item(t, e.editor, "Soon", map[string]any{"due_date": in3.Format("2006-01-02")})
	later := e.item(t, e.editor, "Later", map[string]any{"due_date": "2026-12-20"})
	n1, err := e.engine.EmitDue(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	n2, _ := e.engine.EmitDue(f.ctx)
	e.drain(t)
	if n1 != 1 || n2 != 1 || f.queryID("SELECT count(*)::text FROM domain_events WHERE type='item.due_reached'") != "1" {
		t.Fatalf("due events: %d then %d", n1, n2)
	}
	if importanceOf(e, soon["id"].(string)) != "high" || importanceOf(e, later["id"].(string)) != "normal" {
		t.Fatal("the due trigger acted on the wrong items")
	}

	// form trigger
	form := f.call(e.editor, "POST", "/spaces/"+e.space+"/forms", map[string]any{"name": "Intake", "definition": map[string]any{"pages": []map[string]any{{"title": "p", "elements": []map[string]any{
		{"key": "subject", "kind": "single_line", "label": "Subject", "required": true}}}}}, "response": map[string]any{"item_type": "task", "location": map[string]any{"space_id": e.space}, "title_question": "subject"}})
	f.want(form, 201, "form")
	formID := form.body["id"].(string)
	f.publishForm(e.editor, formID)
	f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", formID)
	e.rule(t, e.editor, "Form intake", `{"when":{"type":"form.submitted","form_id":"`+formID+`"},"then":[{"type":"comment","text":"Received"}]}`)
	f.want(f.submit(watcher, formID, map[string]any{"subject": "Need a laptop"}), 201, "submit")
	e.drain(t)
	if f.queryID("SELECT count(*)::text FROM comments c JOIN items i ON i.id=c.item_id WHERE i.title='Need a laptop' AND c.body_text='Received'") != "1" {
		t.Fatal("form.submitted trigger")
	}
}

// S9.1.3: rules cannot loop, run only with their owner's rights, stay under quota, and stay correct when two run at once.
func TestRulesCannotLoopForever(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	// Two rules that undo each other on a status: each acts once, then the chain stops at the rule already on the way.
	s1 := f.queryID("SELECT id::text FROM statuses WHERE grp='deferred' LIMIT 1")
	s2 := f.queryID("SELECT id::text FROM statuses WHERE grp='cancelled' LIMIT 1")
	ping := e.rule(t, e.editor, "To cancelled", `{"when":{"type":"item.status_changed","to_status_id":"`+s1+`"},"then":[{"type":"set_field","field":"status_id","value":"`+s2+`"}]}`)
	pong := e.rule(t, e.editor, "To deferred", `{"when":{"type":"item.status_changed","to_status_id":"`+s2+`"},"then":[{"type":"set_field","field":"status_id","value":"`+s1+`"}]}`)
	item := e.item(t, e.editor, "Ping pong", nil)
	f.want(f.call(e.editor, "PATCH", "/items/"+item["id"].(string), map[string]any{"status_id": s1}, "If-Match", `"1"`), 200, "start the chain")
	e.drain(t)
	if n := f.queryID("SELECT count(*)::text FROM domain_events WHERE type='item.status_changed' AND item_id=$1", item["id"]); mustInt(n) > 4 {
		t.Fatalf("the status changed %s times: the loop was not stopped", n)
	}
	if e.runs(ping)["loop_blocked"]+e.runs(pong)["loop_blocked"] == 0 {
		t.Fatal("a loop_blocked outcome must be recorded")
	}

	// A rule that triggers itself is stopped after its own effect: assigning someone fires "assigned" again.
	x := f.person("regular")
	f.share(e.space, "user", x.id, "full")
	self := e.rule(t, e.editor, "Self", `{"when":{"type":"item.assigned"},"then":[{"type":"assign","user_ids":["`+x.id+`"]}]}`)
	one := e.item(t, e.editor, "Self trigger", nil)
	f.want(f.call(e.editor, "PATCH", "/items/"+one["id"].(string), map[string]any{"assignee_ids": []string{e.editor.id}}, "If-Match", `"1"`), 200, "assign")
	e.drain(t)
	if f.queryID("SELECT count(*)::text FROM item_assignees WHERE item_id=$1", one["id"]) != "2" || e.runs(self)["loop_blocked"] != 1 {
		t.Fatalf("self trigger: %v", e.runs(self))
	}

	f.exec("DELETE FROM automation_rules WHERE id=ANY($1)", []string{self, ping, pong}) // keep the earlier rules out of the relay
	// A long relay of different rules is cut at the depth limit: assigning user i fires the rule that assigns user i+1.
	people := make([]*person, 8)
	for i := range people {
		people[i] = f.person("regular")
		f.share(e.space, "user", people[i].id, "full")
	}
	var last string
	for i := 0; i < 7; i++ {
		last = e.rule(t, e.editor, fmt.Sprintf("Relay %d", i), `{"when":{"type":"item.assigned","user_id":"`+people[i].id+`"},"then":[{"type":"assign","user_ids":["`+people[i+1].id+`"]}]}`)
	}
	relay := e.item(t, e.editor, "Relay", nil)
	f.want(f.call(e.editor, "PATCH", "/items/"+relay["id"].(string), map[string]any{"assignee_ids": []string{people[0].id}}, "If-Match", `"1"`), 200, "start the relay")
	e.drain(t)
	got := mustInt(f.queryID("SELECT count(*)::text FROM item_assignees WHERE item_id=$1", relay["id"]))
	if got != 1+events.MaxDepth {
		t.Fatalf("the relay assigned %d people, want %d (initial + %d hops)", got, 1+events.MaxDepth, events.MaxDepth)
	}
	if e.runs(last)["done"] != 0 {
		t.Fatal("rules beyond the depth limit must not run")
	}
	if f.queryID("SELECT count(*)::text FROM automation_runs WHERE status='loop_blocked' AND detail LIKE 'chain depth%'") == "0" {
		t.Fatal("the depth limit must be recorded")
	}
}

func TestRulesRunWithTheirOwnersRightsAndCurrentStatus(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	rule := e.rule(t, e.editor, "Escalate", `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"importance","value":"high"}]}`)
	first := e.item(t, e.admin, "Before", nil)
	e.drain(t)
	if importanceOf(e, first["id"].(string)) != "high" {
		t.Fatal("the owner's rule should run while the owner has access")
	}
	// The owner loses access to the Space: the action is refused by policy, nothing changes, and the refusal is on record.
	f.exec("DELETE FROM shares WHERE target_id=$1 AND principal_id=$2", e.space, e.editor.id)
	second := e.item(t, e.admin, "After", nil)
	e.drain(t)
	if importanceOf(e, second["id"].(string)) != "normal" || e.runs(rule)["denied"] != 1 {
		t.Fatalf("a rule must not act beyond its owner's rights: importance %s, runs %v", importanceOf(e, second["id"].(string)), e.runs(rule))
	}
	// A deactivated owner stops all of their rules.
	f.share(e.space, "user", e.editor.id, "full")
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", e.editor.id)
	third := e.item(t, e.admin, "Deactivated", nil)
	e.drain(t)
	if importanceOf(e, third["id"].(string)) != "normal" || e.runs(rule)["denied"] != 2 {
		t.Fatalf("deactivated owner: runs %v", e.runs(rule))
	}
	// A guard stays in force: an automation cannot move an item into a guarded status it does not satisfy.
	f.exec("UPDATE users SET status='active' WHERE id=$1", e.editor.id)
	done := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' LIMIT 1")
	f.exec("UPDATE statuses SET required_fields='{due_date}' WHERE id=$1", done)
	guarded := e.rule(t, e.editor, "Auto complete", `{"when":{"type":"item.assigned"},"then":[{"type":"set_field","field":"status_id","value":"`+done+`"}]}`)
	item := e.item(t, e.admin, "Needs a date", nil)
	f.want(f.call(e.admin, "PATCH", "/items/"+item["id"].(string), map[string]any{"assignee_ids": []string{e.admin.id}}, "If-Match", `"1"`), 200, "assign")
	e.drain(t)
	if e.runs(guarded)["denied"] != 1 || f.queryID("SELECT count(*)::text FROM items i JOIN statuses s ON s.id=i.status_id WHERE i.id=$1 AND s.grp='completed'", item["id"]) != "0" {
		t.Fatalf("guard: %v", e.runs(guarded))
	}
}

func TestQuotasAndParallelRules(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	// Per-rule quota: a rule that already did its hourly share is blocked.
	busy := e.rule(t, e.editor, "Busy", `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"importance","value":"high"}]}`)
	f.exec(`INSERT INTO automation_runs(id,rule_id,rule_version_id,event_id,action_index,status)
SELECT gen_random_uuid(),$1,(SELECT current_version_id FROM automation_rules WHERE id=$1),gen_random_uuid(),0,'done' FROM generate_series(1,$2)`, busy, automation.RuleQuotaPerHour)
	capped := e.item(t, e.editor, "Over quota", nil)
	e.drain(t)
	if importanceOf(e, capped["id"].(string)) != "normal" || e.runs(busy)["quota_blocked"] != 1 {
		t.Fatalf("rule quota: %v", e.runs(busy))
	}
	// Per-Space quota: another rule in the same Space is blocked once the Space as a whole is busy.
	f.exec(`INSERT INTO automation_runs(id,rule_id,rule_version_id,event_id,action_index,status)
SELECT gen_random_uuid(),$1,(SELECT current_version_id FROM automation_rules WHERE id=$1),gen_random_uuid(),0,'done' FROM generate_series(1,$2)`, busy, automation.SpaceQuotaPerHour)
	f.exec("DELETE FROM automation_rules WHERE id=$1", busy)
	calm := e.rule(t, e.editor, "Calm", `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"importance","value":"low"}]}`)
	f.exec(`INSERT INTO automation_runs(id,rule_id,rule_version_id,event_id,action_index,status)
SELECT gen_random_uuid(),(SELECT id FROM automation_rules WHERE name='Calm'),(SELECT current_version_id FROM automation_rules WHERE name='Calm'),gen_random_uuid(),0,'done' FROM generate_series(1,$1)`, automation.SpaceQuotaPerHour)
	space := e.item(t, e.editor, "Busy space", nil)
	e.drain(t)
	if importanceOf(e, space["id"].(string)) != "normal" || e.runs(calm)["quota_blocked"] != 1 {
		t.Fatalf("space quota: %v", e.runs(calm))
	}
	f.exec("DELETE FROM automation_rules WHERE id=$1", calm)
	f.exec("DELETE FROM automation_runs")

	// Two rules and two deliveries of the same event at the same moment: both effects land, each exactly once.
	a := e.rule(t, e.editor, "Importance", `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"importance","value":"high"}]}`)
	b := e.rule(t, e.editor, "Due date", `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"due_date","value":"2026-12-24"},{"type":"set_field","field":"effort_minutes","value":90}]}`)
	for i := range 5 {
		item := e.item(t, e.editor, "Parallel "+strconv.Itoa(i), nil)
		ev := pendingEvent(t, e, item["id"].(string), events.ItemCreated)
		var wg sync.WaitGroup
		for range 3 {
			wg.Add(1)
			go func() { defer wg.Done(); _ = e.engine.Handle(f.ctx, ev) }()
		}
		wg.Wait()
		_ = e.engine.Handle(f.ctx, ev) // and a late redelivery, for good measure
		row := f.queryID("SELECT concat(importance::text,'|',due_date::text,'|',effort_minutes::text) FROM items WHERE id=$1", item["id"])
		if row != "high|2026-12-24|90" {
			t.Fatalf("item %d ended as %s: concurrent rules lost an update (%v / %v)", i, row, e.runs(a), e.runs(b))
		}
	}
	if e.runs(a)["done"] != 5 || e.runs(b)["done"] != 10 {
		t.Fatalf("each action must be done exactly once per event: %v %v", e.runs(a), e.runs(b))
	}
}

func pendingEvent(t *testing.T, e *autoEnv, item, kind string) events.Event {
	t.Helper()
	var ev events.Event
	var payload []byte
	err := e.f.pool.QueryRow(e.f.ctx, `SELECT event_id,type,COALESCE(item_id::text,''),payload,depth,chain FROM domain_events WHERE item_id=$1 AND type=$2 ORDER BY id LIMIT 1`, item, kind).
		Scan(&ev.ID, &ev.Type, &ev.ItemID, &payload, &ev.Depth, &ev.Chain)
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(payload, &ev.Payload)
	return ev
}

// S9.1.4: the log shows what every rule version did, parked events can be replayed by an administrator, and replaying twice
// changes nothing twice.
func TestRunLogDeadEventsAndReplay(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	rule := e.rule(t, e.editor, "Escalate", `{"when":{"type":"item.created"},"if":[{"field":"item_type","op":"eq","value":"task"}],"then":[{"type":"set_field","field":"importance","value":"high"},{"type":"comment","text":"Escalated"}]}`)
	for i := range 3 {
		e.item(t, e.editor, "Log "+strconv.Itoa(i), nil)
	}
	e.drain(t)
	runs := f.call(e.editor, "GET", "/automation-rules/"+rule+"/runs?limit=4", nil)
	if runs.status != 200 || len(runs.items()) != 4 || runs.body["next_cursor"] == nil {
		t.Fatalf("first page: %+v", runs)
	}
	first := runs.items()[0]
	if first["event_type"] != "item.created" || first["status"] != "done" || first["version"] != float64(1) || first["item_title"] == nil || first["actor_id"] != e.editor.id {
		t.Fatalf("a log line must say what happened, to what, and as whom: %+v", first)
	}
	rest := f.call(e.editor, "GET", "/automation-rules/"+rule+"/runs?cursor="+runs.body["next_cursor"].(string), nil)
	if len(rest.items()) != 2 || rest.body["next_cursor"] != nil {
		t.Fatalf("second page: %+v", rest)
	}
	f.want(f.call(e.editor, "GET", "/automation-rules/"+rule+"/runs?status=skipped", nil), 200, "filter")
	f.want(f.call(e.editor, "GET", "/automation-rules/"+rule+"/runs?limit=0", nil), 422, "limit")
	f.want(f.call(e.editor, "GET", "/automation-rules/"+rule+"/runs?cursor=x", nil), 422, "cursor")
	outsider := f.person("regular")
	f.want(f.call(outsider, "GET", "/automation-rules/"+rule+"/runs", nil), 404, "outsider")
	f.want(f.call(f.person("regular"), "GET", "/automation/dead-events", nil), 403, "dead events are for administrators")

	// An event that cannot be handled is parked; once the cause is fixed an admin replays it and only the missing part happens.
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", e.editor.id)
	item := e.item(t, e.admin, "Parked", nil)
	e.drain(t)
	if importanceOf(e, item["id"].(string)) != "normal" {
		t.Fatal("the deactivated owner's rule must not act")
	}
	f.exec("UPDATE users SET status='active' WHERE id=$1", e.editor.id)
	ev := pendingEvent(t, e, item["id"].(string), events.ItemCreated)
	f.exec("UPDATE domain_events SET last_error='boom', attempts=8 WHERE event_id=$1", ev.ID)
	dead := f.call(e.admin, "GET", "/automation/dead-events", nil)
	if dead.status != 200 || len(dead.items()) != 1 || dead.items()[0]["last_error"] != "boom" {
		t.Fatalf("dead events: %+v", dead)
	}
	for range 2 { // replaying twice must not do anything twice
		f.want(f.call(e.admin, "POST", "/automation/events/"+ev.ID.String()+"/replay", nil), 200, "replay")
		e.drain(t)
	}
	if importanceOf(e, item["id"].(string)) != "high" || f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1 AND body_text='Escalated'", item["id"]) != "1" {
		t.Fatalf("the replay must finish the work exactly once: %v", e.runs(rule))
	}
	if d := f.call(e.admin, "GET", "/automation/dead-events", nil); len(d.items()) != 0 {
		t.Fatalf("a replayed event is no longer dead: %+v", d)
	}
	f.want(f.call(e.editor, "POST", "/automation/events/"+ev.ID.String()+"/replay", nil), 403, "only admins replay")
	f.want(f.call(e.admin, "POST", "/automation/events/"+uuid.NewString()+"/replay", nil), 404, "unknown event")
}

// AUT-05: a dry run reports which conditions hold and what would be done, and changes nothing.
func TestDryRunReportsWithoutActing(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	hot := e.item(t, e.editor, "Hot", map[string]any{"importance": "high"})
	def := `{"when":{"type":"item.created"},"if":[{"field":"importance","op":"eq","value":"high"},{"field":"assignee","op":"empty"}],"then":[{"type":"comment","text":"Look at {item.title}"},{"type":"set_field","field":"importance","value":"low"}]}`
	call := func(p *person, body map[string]any) reply {
		return f.call(p, "POST", "/spaces/"+e.space+"/automation-rules/dry-run", body)
	}
	got := call(e.editor, map[string]any{"definition": json.RawMessage(def), "item_id": hot["id"]})
	f.want(got, 200, "dry run")
	if got.body["would_run"] != true || len(got.body["actions"].([]any)) != 2 || got.body["actions"].([]any)[0] != "Comment: Look at Hot" {
		t.Fatalf("dry run: %+v", got.body)
	}
	if importanceOf(e, hot["id"].(string)) != "high" || f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1", hot["id"]) != "0" || f.queryID("SELECT count(*)::text FROM automation_runs") != "0" {
		t.Fatal("a dry run must change nothing")
	}
	f.want(f.call(e.editor, "PATCH", "/items/"+hot["id"].(string), map[string]any{"assignee_ids": []string{e.editor.id}}, "If-Match", `"1"`), 200, "assign")
	again := call(e.editor, map[string]any{"definition": json.RawMessage(def), "item_id": hot["id"]})
	conds := again.body["conditions"].([]any)
	if again.body["would_run"] != false || conds[0].(map[string]any)["holds"] != true || conds[1].(map[string]any)["holds"] != false || len(again.body["actions"].([]any)) != 0 {
		t.Fatalf("a failing condition must be shown: %+v", again.body)
	}
	other := f.space("Other")
	f.want(call(e.editor, map[string]any{"definition": json.RawMessage(def), "item_id": hot["id"]}), 200, "own space")
	_ = other
	f.want(f.call(e.editor, "POST", "/spaces/"+other+"/automation-rules/dry-run", map[string]any{"definition": json.RawMessage(def), "item_id": hot["id"]}), 404, "a Space the user cannot edit")
	f.want(call(f.person("regular"), map[string]any{"definition": json.RawMessage(def), "item_id": hot["id"]}), 404, "no access")
	f.want(call(e.editor, map[string]any{"definition": json.RawMessage(`{"when":{"type":"nope"},"then":[]}`), "item_id": hot["id"]}), 422, "invalid definition")
	f.want(call(e.editor, map[string]any{"definition": json.RawMessage(def), "item_id": uuid.NewString()}), 404, "unknown item")
}

// Scrape the same endpoint and metric names used by the Grafana alerts.
func TestAutomationAlertMetrics(t *testing.T) {
	e := newAutoEnv(t)
	_ = e.engine.Option() // registers the production collectors
	e.f.exec(`INSERT INTO domain_events(event_id,type,dispatched_at,last_error)
SELECT gen_random_uuid(),'item.created',NULL,NULL FROM generate_series(1,201)
UNION ALL SELECT gen_random_uuid(),'item.created',now(),'fixture failure'`)
	check := func(want ...string) {
		t.Helper()
		rec := httptest.NewRecorder()
		httpserver.Metrics().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
		for _, sample := range want {
			if rec.Code != 200 || !strings.Contains(rec.Body.String(), sample+"\n") {
				t.Fatalf("scrape status %d, missing sample %q", rec.Code, sample)
			}
		}
	}
	check("pms_automation_events_pending 201", "pms_automation_events_dead 1")
	e.f.exec("DELETE FROM domain_events")
	check("pms_automation_events_pending 0", "pms_automation_events_dead 0")
}
