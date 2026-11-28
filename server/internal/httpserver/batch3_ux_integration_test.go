package httpserver_test

import (
	"fmt"
	"strings"
	"testing"
)

// Request form: the page can tell a person beforehand that their submission will not assign others; people it does not concern are
// told nothing, and nobody is named.
func TestFormAssignmentNoticeOnlyWhenItApplies(t *testing.T) {
	f := newFixture(t)
	full, editor, reader, outsider, teammate := f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.spaceWithoutAdmins("Notice")
	for p, role := range map[*person]string{full: "full", editor: "editor", reader: "read", teammate: "editor"} {
		f.share(space, "user", p.id, role)
	}
	form := func(name string, assignees []string, ask bool) string {
		els := []map[string]any{{"key": "subject", "kind": "single_line", "label": "Subject"}}
		if ask {
			els = append(els, map[string]any{"key": "who", "kind": "assignee", "label": "Who"})
		}
		made := f.call(full, "POST", "/spaces/"+space+"/forms", map[string]any{"name": name, "definition": map[string]any{"pages": []map[string]any{{"title": "One", "elements": els}}},
			"response": map[string]any{"item_type": "task", "location": map[string]any{"space_id": space}, "assignee_ids": assignees}})
		f.want(made, 201, name)
		id := made.body["id"].(string)
		f.publishForm(full, id)
		f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", id)
		return id
	}
	preset, asks, plain := form("Preset", []string{teammate.id}, false), form("Asks", []string{}, true), form("Plain", []string{}, false)
	for _, c := range []struct {
		who  *person
		form string
		want string
	}{{editor, preset, "without_others"}, {editor, asks, "without_others"}, {editor, plain, ""}, {full, preset, ""}, {full, asks, ""},
		{reader, preset, ""}, {reader, asks, "choices_ignored"}, {outsider, preset, ""}, {outsider, asks, ""}} {
		got := f.call(c.who, "GET", "/forms/"+c.form+"/definition", nil)
		f.want(got, 200, "definition")
		if got.body["assignment_notice"] != c.want {
			t.Errorf("notice = %v, want %q (form %s)", got.body["assignment_notice"], c.want, c.form)
		}
		if s := fmt.Sprint(got.body); len(s) > 0 && (strings.Contains(s, teammate.id) || strings.Contains(s, "assignee_ids")) {
			t.Errorf("the routing is not disclosed: %s", s)
		}
	}
}

// Timesheet steps reach the people they concern, once, and nobody who lost access.
func TestTimesheetStepsNotifyTheRightPeople(t *testing.T) {
	f := newFixture(t)
	reviewer, author, peer := f.person("regular"), f.person("regular"), f.person("regular")
	space := f.spaceWithoutAdmins("Notify")
	f.share(space, "user", reviewer.id, "full")
	f.share(space, "user", author.id, "editor")
	f.share(space, "user", peer.id, "editor")
	task := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','T',$1) RETURNING id::text", reviewer.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", task, space)
	count := func(who *person, kind string) string {
		return f.queryID("SELECT count(*)::text FROM notifications WHERE recipient_id=$1 AND kind=$2", who.id, kind)
	}
	var ids []string
	for _, day := range []string{"2026-11-23", "2026-11-24"} {
		made := f.call(author, "POST", "/time-entries", map[string]any{"item_id": task, "date": day, "minutes": 60})
		f.want(made, 201, "log")
		ids = append(ids, made.body["id"].(string))
	}
	week := map[string]any{"from": "2026-11-23", "to": "2026-11-29"}
	f.want(f.call(author, "POST", "/time-entries/submit", week), 200, "submit")
	f.want(f.call(author, "POST", "/time-entries/submit", week), 200, "submit again: nothing new")
	if count(reviewer, "timesheet_submitted") != "1" || count(peer, "timesheet_submitted") != "0" || count(author, "timesheet_submitted") != "0" {
		t.Fatalf("one notice per task to approvers only: %s %s %s", count(reviewer, "timesheet_submitted"), count(peer, "timesheet_submitted"), count(author, "timesheet_submitted"))
	}
	f.want(f.call(reviewer, "POST", "/time-entries/"+ids[0]+"/review", map[string]any{"decision": "rejected", "note": "Wrong day"}, "If-Match", `"2"`), 200, "reject")
	if got := f.queryID("SELECT snippet FROM notifications WHERE recipient_id=$1 AND kind='timesheet_rejected'", author.id); got != "Wrong day" {
		t.Fatalf("the reason reaches the author: %q", got)
	}
	f.want(f.call(author, "POST", "/time-entries/submit", week), 200, "resubmit")
	if count(reviewer, "timesheet_submitted") != "2" {
		t.Fatalf("a resubmission notifies again: %s", count(reviewer, "timesheet_submitted"))
	}
	f.want(f.call(reviewer, "POST", "/time-entries/"+ids[1]+"/review", map[string]any{"decision": "approved"}, "If-Match", `"2"`), 200, "approve")
	f.want(f.call(reviewer, "PATCH", "/time-entries/"+ids[1], map[string]any{"locked": true}, "If-Match", `"3"`), 200, "lock")
	if count(author, "timesheet_approved") != "1" || count(author, "timesheet_locked") != "1" {
		t.Fatalf("approved %s locked %s", count(author, "timesheet_approved"), count(author, "timesheet_locked"))
	}
	f.want(f.call(author, "GET", "/notifications?limit=20", nil), 200, "inbox")
	// Access lost: the next step is recorded but not told.
	f.exec("DELETE FROM shares WHERE target_id=$1 AND principal_id=$2", space, author.id)
	f.want(f.call(reviewer, "POST", "/time-entries/"+ids[0]+"/review", map[string]any{"decision": "approved"}, "If-Match", `"4"`), 200, "approve the other")
	if count(author, "timesheet_approved") != "1" {
		t.Fatal("someone without access is not told about the task")
	}
}

// A Space whose admin is inactive says so to the people who can open it.
func TestSpaceSaysWhenNoAdminIsActive(t *testing.T) {
	f := newFixture(t)
	admin, creator, member := f.person("admin"), f.person("regular"), f.person("regular")
	made := f.call(creator, "POST", "/spaces", map[string]any{"scope_state": "internal", "name": "Gov"})
	space := made.body["id"].(string)
	f.share(space, "user", member.id, "editor")
	active := func() any { return f.call(member, "GET", "/spaces/"+space, nil).body["has_active_admin"] }
	if active() != true {
		t.Fatalf("with an admin: %v", active())
	}
	f.want(f.call(admin, "POST", "/users/"+creator.id+"/deactivate", nil), 200, "deactivate")
	if active() != false {
		t.Fatalf("without: %v", active())
	}
}
