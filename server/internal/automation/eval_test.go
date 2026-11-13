package automation

import (
	"encoding/json"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/events"
)

func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func TestConditionsAreEvaluatedFromATable(t *testing.T) {
	user := "11111111-1111-4111-8111-111111111111"
	fid := "22222222-2222-4222-8222-222222222222"
	facts := Facts{Importance: "high", StatusGroup: "active", ItemType: "bug", Assignees: []string{user}, DueDate: "2026-12-01",
		Fields: map[string]json.RawMessage{fid: raw(5.0)}}
	cases := []struct {
		name string
		c    Condition
		want bool
	}{
		{"importance eq", Condition{"importance", "eq", raw("high")}, true},
		{"importance eq other", Condition{"importance", "eq", raw("low")}, false},
		{"importance neq", Condition{"importance", "neq", raw("low")}, true},
		{"status group", Condition{"status_group", "eq", raw("active")}, true},
		{"status group neq", Condition{"status_group", "neq", raw("completed")}, true},
		{"item type", Condition{"item_type", "eq", raw("bug")}, true},
		{"item type other", Condition{"item_type", "neq", raw("bug")}, false},
		{"assignee has", Condition{"assignee", "has", raw(user)}, true},
		{"assignee has not", Condition{"assignee", "has_not", raw(user)}, false},
		{"assignee empty", Condition{"assignee", "empty", nil}, false},
		{"assignee not empty", Condition{"assignee", "not_empty", nil}, true},
		{"due set", Condition{"due_date", "set", nil}, true},
		{"start unset", Condition{"start_date", "unset", nil}, true},
		{"due before later date", Condition{"due_date", "before", raw("2027-01-01")}, true},
		{"due before today", Condition{"due_date", "before", raw("today")}, false},
		{"due after today", Condition{"due_date", "after", raw("today")}, true},
		{"due after with no date", Condition{"start_date", "after", raw("2020-01-01")}, false},
		{"field eq", Condition{"fields." + fid, "eq", raw(5.0)}, true},
		{"field eq other", Condition{"fields." + fid, "eq", raw(6.0)}, false},
		{"field gt", Condition{"fields." + fid, "gt", raw(4)}, true},
		{"field lt", Condition{"fields." + fid, "lt", raw(4)}, false},
		{"field set", Condition{"fields." + fid, "set", nil}, true},
		{"missing field unset", Condition{"fields.33333333-3333-4333-8333-333333333333", "unset", nil}, true},
		{"missing field neq", Condition{"fields.33333333-3333-4333-8333-333333333333", "neq", raw(1)}, true},
		{"missing field eq", Condition{"fields.33333333-3333-4333-8333-333333333333", "eq", raw(1)}, false},
		{"unknown field never holds", Condition{"title", "eq", raw("x")}, false},
		{"unknown op never holds", Condition{"importance", "gt", raw("high")}, false},
	}
	for _, c := range cases {
		if got := c.c.Holds(facts, "2026-11-13"); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	d := Definition{If: []Condition{{"importance", "eq", raw("high")}, {"item_type", "eq", raw("task")}}}
	if d.AllHold(facts, "2026-11-13") {
		t.Error("one failing condition must fail the rule")
	}
	if !(Definition{}).AllHold(facts, "2026-11-13") {
		t.Error("no conditions always holds")
	}
}

func TestTriggerMatching(t *testing.T) {
	days := 3
	ev := func(kind string, payload map[string]any) events.Event {
		return events.Event{Type: kind, Payload: payload}
	}
	cases := []struct {
		name string
		t    Trigger
		e    events.Event
		f    Facts
		want bool
	}{
		{"status any", Trigger{Type: events.ItemStatusChanged}, ev(events.ItemStatusChanged, map[string]any{"to": "s1"}), Facts{}, true},
		{"status to", Trigger{Type: events.ItemStatusChanged, ToStatusID: "s1"}, ev(events.ItemStatusChanged, map[string]any{"to": "s1"}), Facts{}, true},
		{"status to other", Trigger{Type: events.ItemStatusChanged, ToStatusID: "s2"}, ev(events.ItemStatusChanged, map[string]any{"to": "s1"}), Facts{}, false},
		{"wrong event type", Trigger{Type: events.ItemStatusChanged}, ev(events.ItemAssigned, nil), Facts{}, false},
		{"assigned user", Trigger{Type: events.ItemAssigned, UserID: "u1"}, ev(events.ItemAssigned, map[string]any{"user_ids": []any{"u0", "u1"}}), Facts{}, true},
		{"assigned other user", Trigger{Type: events.ItemAssigned, UserID: "u2"}, ev(events.ItemAssigned, map[string]any{"user_ids": []any{"u1"}}), Facts{}, false},
		{"created of type", Trigger{Type: events.ItemCreated, ItemType: "bug"}, ev(events.ItemCreated, nil), Facts{ItemType: "bug"}, true},
		{"created other type", Trigger{Type: events.ItemCreated, ItemType: "bug"}, ev(events.ItemCreated, nil), Facts{ItemType: "task"}, false},
		{"due in 3 days", Trigger{Type: events.ItemDueReached, DaysBefore: &days}, ev(events.ItemDueReached, map[string]any{"days_before": float64(3)}), Facts{}, true},
		{"due in 2 days", Trigger{Type: events.ItemDueReached, DaysBefore: &days}, ev(events.ItemDueReached, map[string]any{"days_before": float64(2)}), Facts{}, false},
		{"form any", Trigger{Type: events.FormSubmitted}, ev(events.FormSubmitted, map[string]any{"form_id": "f1"}), Facts{}, true},
		{"form other", Trigger{Type: events.FormSubmitted, FormID: "f2"}, ev(events.FormSubmitted, map[string]any{"form_id": "f1"}), Facts{}, false},
	}
	for _, c := range cases {
		if got := c.t.Matches(c.e, c.f); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDefinitionsAreValidatedStrictly(t *testing.T) {
	const user = "11111111-1111-4111-8111-111111111111"
	good := `{"when":{"type":"item.status_changed"},"if":[{"field":"importance","op":"eq","value":"high"}],"then":[{"type":"comment","text":"Escalated"},{"type":"assign","user_ids":["` + user + `"]},{"type":"set_field","field":"importance","value":"low"}]}`
	if _, err := ParseDefinition([]byte(good)); err != nil {
		t.Fatalf("good definition refused: %v", err)
	}
	bad := map[string]string{
		"unknown key":            `{"when":{"type":"item.created"},"then":[{"type":"comment","text":"x"}],"extra":1}`,
		"unknown trigger":        `{"when":{"type":"item.deleted"},"then":[{"type":"comment","text":"x"}]}`,
		"no actions":             `{"when":{"type":"item.created"},"then":[]}`,
		"too many actions":       `{"when":{"type":"item.created"},"then":[` + repeat(`{"type":"comment","text":"x"}`, 11) + `]}`,
		"trigger setting misuse": `{"when":{"type":"item.created","to_status_id":"` + user + `"},"then":[{"type":"comment","text":"x"}]}`,
		"due needs days":         `{"when":{"type":"item.due_reached"},"then":[{"type":"comment","text":"x"}]}`,
		"due days too far":       `{"when":{"type":"item.due_reached","days_before":31},"then":[{"type":"comment","text":"x"}]}`,
		"bad condition field":    `{"when":{"type":"item.created"},"if":[{"field":"title","op":"eq","value":"x"}],"then":[{"type":"comment","text":"x"}]}`,
		"bad condition op":       `{"when":{"type":"item.created"},"if":[{"field":"importance","op":"gt","value":"high"}],"then":[{"type":"comment","text":"x"}]}`,
		"bad importance":         `{"when":{"type":"item.created"},"if":[{"field":"importance","op":"eq","value":"urgent"}],"then":[{"type":"comment","text":"x"}]}`,
		"assignee needs user":    `{"when":{"type":"item.created"},"if":[{"field":"assignee","op":"has","value":"nobody"}],"then":[{"type":"comment","text":"x"}]}`,
		"unknown action":         `{"when":{"type":"item.created"},"then":[{"type":"delete_item"}]}`,
		"set_field title":        `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"title","value":"x"}]}`,
		"set_field no value":     `{"when":{"type":"item.created"},"then":[{"type":"set_field","field":"importance"}]}`,
		"assign nobody":          `{"when":{"type":"item.created"},"then":[{"type":"assign","user_ids":[]}]}`,
		"assign bad id":          `{"when":{"type":"item.created"},"then":[{"type":"assign","user_ids":["x"]}]}`,
		"comment empty":          `{"when":{"type":"item.created"},"then":[{"type":"comment","text":"  "}]}`,
		"comment stray setting":  `{"when":{"type":"item.created"},"then":[{"type":"comment","text":"x","title":"y"}]}`,
		"notify nobody":          `{"when":{"type":"item.created"},"then":[{"type":"notify","message":"x"}]}`,
		"create_item no title":   `{"when":{"type":"item.created"},"then":[{"type":"create_item","title":""}]}`,
		"create_item bad kind":   `{"when":{"type":"item.created"},"then":[{"type":"create_item","title":"x","kind":"epic"}]}`,
		"not json":               `{`,
	}
	for name, def := range bad {
		if _, err := ParseDefinition([]byte(def)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func repeat(s string, n int) string {
	out := s
	for i := 1; i < n; i++ {
		out += "," + s
	}
	return out
}
