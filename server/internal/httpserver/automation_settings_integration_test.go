package httpserver_test

import (
	"fmt"
	"testing"
)

// The workspace time zone is set on the work schedule page and read by automation (decision: local-day deadlines).
func TestWorkspaceTimeZoneIsSetOnTheWorkSchedule(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	got := f.call(e.admin, "GET", "/settings/work-schedule", nil)
	f.want(got, 200, "read the work schedule")
	if got.body["timezone"] != "UTC" {
		t.Fatalf("the zone starts as UTC: %+v", got.body)
	}
	version := fmt.Sprint(int(got.body["version"].(float64)))
	put := map[string]any{"working_days": []int{1, 2, 3, 4, 5}, "daily_hours": 8, "exceptions": []any{}, "timezone": "Asia/Jakarta"}
	f.want(f.call(e.admin, "PUT", "/settings/work-schedule", put, "If-Match", `"`+version+`"`), 200, "set Jakarta")
	if f.queryID("SELECT timezone FROM work_schedules WHERE id=1") != "Asia/Jakarta" {
		t.Fatal("the zone is saved")
	}
	f.want(f.call(e.editor, "PUT", "/settings/work-schedule", put, "If-Match", `"`+fmt.Sprint(version)+`"`), 403, "only an admin sets the zone")
	bad := map[string]any{"working_days": []int{1}, "daily_hours": 8, "exceptions": []any{}, "timezone": "Mars/Olympus"}
	next := fmt.Sprint(int(got.body["version"].(float64)) + 1)
	f.want(f.call(e.admin, "PUT", "/settings/work-schedule", bad, "If-Match", `"`+next+`"`), 422, "an unknown zone is refused")
	omitted := map[string]any{"working_days": []int{1, 2, 3, 4, 5}, "daily_hours": 8, "exceptions": []any{}}
	f.want(f.call(e.admin, "PUT", "/settings/work-schedule", omitted, "If-Match", `"`+next+`"`), 200, "an omitted zone keeps the current one")
	if f.queryID("SELECT timezone FROM work_schedules WHERE id=1") != "Asia/Jakarta" {
		t.Fatal("an omitted zone is not reset")
	}
}

// AUT-T17 (cap): one mention reaches at most 50 people. A mention that would reach more is refused, and nothing is written.
func TestMentionIsRefusedWhenItWouldReachTooManyPeople(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	f.exec(`INSERT INTO users(email,first_name,last_name,password_hash,user_type,status)
SELECT 'crowd'||g||'@example.test','C','P','x','regular','active' FROM generate_series(1,60) g`)
	rule := e.rule(t, e.editor, "Crowd", `{"when":{"type":"item.created"},"then":[{"type":"mention","text":"everyone","followers":true}]}`)
	item := e.item(t, e.admin, "Busy", nil)
	// Followers are read when the rule runs: the 60 added here plus the item's creator, who follows it, makes 61.
	f.exec("INSERT INTO item_followers(item_id,user_id) SELECT $1::uuid, id FROM users WHERE email::text LIKE 'crowd%'", item["id"])
	e.drain(t)

	if e.runs(rule)["denied"] != 1 || e.runs(rule)["done"] != 0 {
		t.Fatalf("the mention is refused: %v", e.runs(rule))
	}
	if f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1", item["id"]) != "0" ||
		f.queryID("SELECT count(*)::text FROM notifications WHERE kind='mention'") != "0" {
		t.Fatal("a refused mention writes no comment and sends no notification")
	}
	if detail := f.queryID("SELECT detail FROM automation_runs WHERE rule_id=$1 AND status='denied'", rule); detail != "this mention would reach 61 people; the limit is 50" {
		t.Fatalf("the refusal says how many and the limit: %q", detail)
	}
}

// AUT-T07: a condition can be "one of" several values (OR-style): the rule runs when the item's importance is any of them.
func TestConditionAcceptsAnyOfValues(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	rule := e.rule(t, e.editor, "Urgent or high", `{"when":{"type":"item.created"},"if":[{"field":"importance","op":"in","value":["high","low"]}],"then":[{"type":"comment","text":"hot"}]}`)
	normal := e.item(t, e.editor, "Normal", nil)
	urgent := e.item(t, e.editor, "High", map[string]any{"importance": "high"})
	e.drain(t)
	if f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1", normal["id"]) != "0" ||
		f.queryID("SELECT count(*)::text FROM comments WHERE item_id=$1", urgent["id"]) != "1" {
		t.Fatalf("only the matching item triggers: %v", e.runs(rule))
	}
	bad := map[string]any{"name": "Bad", "definition": map[string]any{"when": map[string]any{"type": "item.created"}, "if": []any{map[string]any{"field": "importance", "op": "in", "value": []string{"urgent"}}}, "then": []any{map[string]any{"type": "comment", "text": "x"}}}}
	f.want(f.call(e.editor, "POST", "/spaces/"+e.space+"/automation-rules", bad), 422, "an unknown value in a list is refused")
}
