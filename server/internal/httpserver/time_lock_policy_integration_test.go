package httpserver_test

import (
	"fmt"
	"sync"
	"testing"
)

// DEC-06 Rev.1 (DEF-031): only approved time is cost. Lock freezes an entry; it neither approves nor un-approves it.
func TestLockingIsNotApproval(t *testing.T) {
	f := newFixture(t)
	manager, member := f.person("admin"), f.person("regular")
	space := f.spaceWithoutAdmins("Lock policy")
	f.share(space, "user", manager.id, "full")
	f.share(space, "user", member.id, "editor")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','P',$1) RETURNING id::text", manager.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", project, space)
	task := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','T',$1) RETURNING id::text", manager.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'A')", task, space, project)
	f.want(f.call(manager, "POST", "/rates", map[string]any{"user_id": member.id, "rate_minor": 6000, "effective_from": "2026-01-01"}), 201, "rate")

	tag := func(version int) string { return fmt.Sprintf(`"%d"`, version) }
	log := func(date string) string {
		made := f.call(member, "POST", "/time-entries", map[string]any{"item_id": task, "date": date, "minutes": 60, "billing_type": "billable"})
		f.want(made, 201, "log")
		return made.body["id"].(string)
	}
	submit := func(date string) {
		f.want(f.call(member, "POST", "/time-entries/submit", map[string]any{"from": date, "to": date}), 200, "submit")
	}
	version := func(id string) int {
		return int(f.call(manager, "GET", "/time-entries/"+id, nil).body["version"].(float64))
	}
	lock := func(p *person, id string, v int, on bool) reply {
		return f.call(p, "PATCH", "/time-entries/"+id, map[string]any{"locked": on}, "If-Match", tag(v))
	}
	review := func(id string, v int, decision string) reply {
		return f.call(manager, "POST", "/time-entries/"+id+"/review", map[string]any{"decision": decision, "note": "x"}, "If-Match", tag(v))
	}
	cost := func(want float64, when string) {
		t.Helper()
		got := f.call(manager, "GET", "/items/"+project+"/time-summary", nil)
		f.want(got, 200, "summary")
		if got.body["cost_status"] != "full" || got.body["actual_cost_minor"] != want {
			t.Fatalf("%s: want %v, got %v", when, want, got.body)
		}
	}

	waiting := log("2026-11-02")
	submit("2026-11-02")
	f.want(lock(manager, waiting, version(waiting), true), 200, "lock a submitted entry")
	cost(0, "locked without approval")
	f.want(review(waiting, version(waiting), "approved"), 409, "a locked entry cannot be approved")

	done := log("2026-11-03")
	submit("2026-11-03")
	f.want(review(done, version(done), "approved"), 200, "approve")
	cost(6000, "approved")
	f.want(lock(manager, done, version(done), true), 200, "lock approved")
	cost(6000, "approved then locked")
	f.want(review(done, version(done), "rejected"), 409, "no reversal while locked")
	f.want(f.call(member, "DELETE", "/time-entries/"+done, nil, "If-Match", tag(version(done))), 409, "locked cannot be deleted")
	f.want(lock(member, done, version(done), false), 403, "unlock needs Full")
	f.want(lock(manager, done, version(done), false), 200, "unlock")
	cost(6000, "unlock keeps the approval")
	f.want(f.call(member, "PATCH", "/time-entries/"+done, map[string]any{"minutes": 90}, "If-Match", tag(version(done))), 409, "still frozen: approved")
	f.want(review(done, version(done), "rejected"), 200, "reversal")
	cost(0, "reversed")

	// Approve and lock at the same version: exactly one wins, and the cost follows the approval alone.
	race := log("2026-11-04")
	submit("2026-11-04")
	v := version(race)
	var wg sync.WaitGroup
	var a, l reply
	wg.Add(2)
	go func() { defer wg.Done(); a = review(race, v, "approved") }()
	go func() { defer wg.Done(); l = lock(manager, race, v, true) }()
	wg.Wait()
	if (a.status == 200) == (l.status == 200) || a.status >= 500 || l.status >= 500 {
		t.Fatalf("one wins, the other conflicts: approve %d, lock %d", a.status, l.status)
	}
	if a.status == 200 {
		cost(6000, "approval won")
	} else {
		cost(0, "lock won")
	}
}
