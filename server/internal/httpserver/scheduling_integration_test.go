package httpserver_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func scheduledItem(f *fixture, actor *person, space, title, start, due string, duration int) string {
	f.t.Helper()
	id := f.queryID(`
INSERT INTO items(type,title,created_by,start_date,due_date,duration_days,is_milestone,scope_state)
VALUES ('task',$1,$2,$3::text::date,$4::text::date,$5,$5=0,'internal') RETURNING id::text`, title, actor.id, start, due, duration)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
	return id
}

func addSchedulingDependency(f *fixture, actor *person, predecessor, successor, kind string, lag int) reply {
	f.t.Helper()
	res := f.call(actor, http.MethodPost, dependencyPath(successor), map[string]any{
		"predecessor_id": predecessor, "type": kind, "lag": lag,
	})
	f.want(res, http.StatusCreated, "create scheduling dependency")
	return res
}

type scheduledChange struct {
	id, start, due string
	version        int
}

func wantScheduledChanges(t *testing.T, res reply, want ...scheduledChange) {
	t.Helper()
	raw, _ := res.body["affected_items"].([]any)
	if len(raw) != len(want) {
		t.Fatalf("affected_items = %v, want %v", raw, want)
	}
	actual := make(map[string]map[string]any, len(raw))
	for _, entry := range raw {
		item := entry.(map[string]any)
		actual[item["id"].(string)] = item
	}
	for _, change := range want {
		item := actual[change.id]
		if item["start_date"] != change.start || item["due_date"] != change.due || item["version"] != float64(change.version) {
			t.Errorf("item %s = %v, want %s through %s at version %d", change.id, item, change.start, change.due, change.version)
		}
	}
}

func scheduleHoliday(f *fixture, actor *person) {
	f.t.Helper()
	res := f.call(actor, http.MethodPut, "/settings/work-schedule", map[string]any{
		"working_days": []int{1, 2, 3, 4, 5}, "daily_hours": 8,
		"exceptions": []map[string]string{{"date": "2026-01-12", "name": "Office closed"}},
	}, "If-Match", `"1"`)
	f.want(res, http.StatusOK, "set work schedule with holiday")
}

func TestSchedulingChain(t *testing.T) {
	f := newFixture(t)
	admin, observer := f.person("admin"), f.person("regular")
	space := f.space("Scheduling chain")
	f.share(space, "user", admin.id, "full") // A1.2: the schedule owner is an explicit member
	f.share(space, "user", observer.id, "read")
	scheduleHoliday(f, admin)

	a := scheduledItem(f, admin, space, "Predecessor", "2026-01-05", "2026-01-05", 1)
	b := scheduledItem(f, admin, space, "Two-day task", "2026-01-06", "2026-01-07", 2)
	c := scheduledItem(f, admin, space, "Milestone", "2026-01-08", "2026-01-08", 0)
	late := scheduledItem(f, admin, space, "Later task", "2026-01-26", "2026-01-26", 1)
	undated := dependencyItem(f, admin, space, "Undated")
	for _, edge := range [][2]string{{a, b}, {b, c}, {a, late}, {a, undated}, {undated, late}} {
		wantScheduledChanges(t, addSchedulingDependency(f, admin, edge[0], edge[1], "FS", 0))
	}
	patch := func(start, due, version string) reply {
		return f.call(admin, http.MethodPatch, "/items/"+a, map[string]any{
			"start_date": start, "due_date": due,
		}, "If-Match", version)
	}

	// Force a failure at the end of the cascade's activity writes.
	f.exec(fmt.Sprintf("ALTER TABLE activity ADD CONSTRAINT test_milestone_activity_failure CHECK (item_id <> '%s'::uuid) NOT VALID", c))
	rejected := patch("2026-01-09", "2026-01-09", `"1"`)
	if rejected.status < 400 {
		t.Fatalf("cascade succeeded without all activity writes: %+v", rejected)
	}
	f.exec("ALTER TABLE activity DROP CONSTRAINT test_milestone_activity_failure")
	for _, item := range []string{a, b, c} {
		got := f.call(observer, http.MethodGet, "/items/"+item, nil)
		f.want(got, http.StatusOK, "read after rollback")
		if got.header.Get("ETag") != `"1"` {
			t.Fatalf("rollback left item %s at %q", item, got.header.Get("ETag"))
		}
		if count := f.queryID("SELECT count(*)::text FROM activity WHERE item_id=$1", item); count != "0" {
			t.Fatalf("rollback left %s activity rows for item %s", count, item)
		}
	}

	scanner := openScheduleStream(f, observer)

	res := patch("2026-01-09", "2026-01-09", `"1"`)
	f.want(res, http.StatusOK, "move predecessor across weekend and holiday")
	if res.body["id"] != a || res.body["due_date"] != "2026-01-09" || res.header.Get("ETag") != `"2"` {
		t.Fatalf("patched item: %+v", res)
	}
	wantScheduledChanges(t, res,
		scheduledChange{b, "2026-01-13", "2026-01-14", 2},
		scheduledChange{c, "2026-01-15", "2026-01-15", 2},
	)
	pendingEvents := map[string]bool{b: true, c: true}
	for len(pendingEvents) > 0 && scanner.Scan() {
		data, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if !ok {
			continue
		}
		var event struct {
			Type     string `json:"type"`
			Affected []struct {
				ID      string `json:"id"`
				Version int    `json:"version"`
			} `json:"affected_items"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			t.Fatal(err)
		}
		if event.Type != "item.updated" {
			continue
		}
		for _, item := range event.Affected {
			if item.Version == 2 {
				delete(pendingEvents, item.ID)
			}
		}
		break
	}
	if len(pendingEvents) != 0 {
		t.Fatalf("missing committed successor events: %v (%v)", pendingEvents, scanner.Err())
	}
	for _, change := range []scheduledChange{
		{b, "2026-01-13", "2026-01-14", 2}, {c, "2026-01-15", "2026-01-15", 2},
		{late, "2026-01-26", "2026-01-26", 1}, {undated, "", "", 1},
	} {
		current := f.call(observer, http.MethodGet, "/items/"+change.id, nil)
		f.want(current, http.StatusOK, "read committed schedule")
		start, _ := current.body["start_date"].(string)
		due, _ := current.body["due_date"].(string)
		if start != change.start || due != change.due || current.body["version"] != float64(change.version) {
			t.Errorf("item %s: got %v, want %v", change.id, current.body, change)
		}
	}
	for _, item := range []string{b, c} {
		if count := f.queryID(`SELECT count(*)::text FROM activity
WHERE item_id=$1 AND actor_id=$2 AND changes ? 'start_date' AND changes ? 'due_date'`, item, admin.id); count != "1" {
			t.Errorf("item %s: got %s activity entries containing both date changes, want 1", item, count)
		}
	}
	stale := f.call(admin, http.MethodPatch, "/items/"+b, map[string]string{"title": "Stale editor"}, "If-Match", `"1"`)
	f.want(stale, http.StatusConflict, "stale edit after successor version change")
	fresh := patch("2026-01-05", "2026-01-05", `"2"`)
	f.want(fresh, http.StatusOK, "move predecessor earlier")
	wantScheduledChanges(t, fresh)

	extended := f.call(admin, http.MethodPatch, "/items/"+a, map[string]int{"duration_days": 8}, "If-Match", `"3"`)
	f.want(extended, http.StatusOK, "extend predecessor duration")
	if extended.body["start_date"] != "2026-01-05" || extended.body["due_date"] != "2026-01-15" {
		t.Fatalf("duration-only edit did not recalculate due date: %v", extended.body)
	}
	wantScheduledChanges(t, extended,
		scheduledChange{b, "2026-01-16", "2026-01-19", 3},
		scheduledChange{c, "2026-01-20", "2026-01-20", 3},
	)
}

func TestSchedulingDependencyChanges(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Dependency scheduling")
	f.share(space, "user", admin.id, "full") // A1.2: the schedule owner is an explicit member
	scheduleHoliday(f, admin)
	a := scheduledItem(f, admin, space, "Predecessor", "2026-01-13", "2026-01-15", 3)

	for _, tc := range []struct {
		kind, start, due string
		lag              int
	}{
		{"FS", "2026-01-15", "2026-01-16", -1},
		{"SS", "2026-01-15", "2026-01-16", 2},
		{"FF", "2026-01-15", "2026-01-16", 1},
		{"SF", "2026-01-08", "2026-01-09", -1},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			b := scheduledItem(f, admin, space, tc.kind+" successor", "2026-01-05", "2026-01-06", 2)
			res := addSchedulingDependency(f, admin, a, b, tc.kind, tc.lag)
			wantScheduledChanges(t, res, scheduledChange{b, tc.start, tc.due, 2})
			if tc.kind != "FS" {
				return
			}
			c := scheduledItem(f, admin, space, "Indirect successor", "2026-01-19", "2026-01-19", 1)
			wantScheduledChanges(t, addSchedulingDependency(f, admin, b, c, "FS", 0))
			path := "/dependencies/" + res.body["id"].(string)
			changed := f.call(admin, http.MethodPatch, path, map[string]int{"lag": 2})
			f.want(changed, http.StatusOK, "change lag")
			if changed.body["type"] != "FS" || changed.body["lag"] != float64(2) {
				t.Fatalf("updated dependency: %v", changed.body)
			}
			wantScheduledChanges(t, changed,
				scheduledChange{b, "2026-01-20", "2026-01-21", 3},
				scheduledChange{c, "2026-01-22", "2026-01-22", 2},
			)
			eased := f.call(admin, http.MethodPatch, path, map[string]string{"type": "SS"})
			f.want(eased, http.StatusOK, "change dependency type")
			if eased.body["type"] != "SS" || eased.body["lag"] != float64(2) {
				t.Fatalf("updated dependency: %v", eased.body)
			}
			wantScheduledChanges(t, eased)
			f.want(f.call(admin, http.MethodDelete, path, nil), http.StatusNoContent, "remove dependency")
			for _, change := range []scheduledChange{{b, "2026-01-20", "2026-01-21", 3}, {c, "2026-01-22", "2026-01-22", 2}} {
				item := f.call(admin, http.MethodGet, "/items/"+change.id, nil)
				f.want(item, http.StatusOK, "read after removing dependency")
				if item.body["start_date"] != change.start || item.body["due_date"] != change.due || item.body["version"] != float64(change.version) {
					t.Fatalf("removing a constraint changed item %s: %v", change.id, item.body)
				}
			}
		})
	}
}

func TestSchedulingDirectEditAndRecycleBin(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Active schedule")
	f.share(space, "user", admin.id, "full") // A1.2: the schedule owner is an explicit member
	a := scheduledItem(f, admin, space, "Predecessor", "2026-01-07", "2026-01-07", 1)
	b := scheduledItem(f, admin, space, "Milestone", "2026-01-08", "2026-01-08", 0)
	c := scheduledItem(f, admin, space, "Successor", "2026-01-09", "2026-01-09", 1)
	addSchedulingDependency(f, admin, a, b, "FS", 0)
	addSchedulingDependency(f, admin, b, c, "FS", 0)

	patch := func(id, day, version string) reply {
		return f.call(admin, http.MethodPatch, "/items/"+id, map[string]string{
			"start_date": day, "due_date": day,
		}, "If-Match", version)
	}
	clamped := patch(b, "2026-01-06", `"1"`)
	f.want(clamped, http.StatusOK, "direct edit respects the incoming constraint")
	if clamped.header.Get("ETag") != `"2"` || clamped.body["start_date"] != "2026-01-08" || clamped.body["due_date"] != "2026-01-08" {
		t.Fatalf("clamp should increment the directly edited milestone once: %+v", clamped)
	}
	wantScheduledChanges(t, clamped)
	f.want(f.call(admin, http.MethodDelete, "/items/"+b, nil), http.StatusNoContent, "move intermediate milestone to Recycle bin")
	moved := patch(a, "2026-01-12", `"1"`)
	f.want(moved, http.StatusOK, "edit with a dormant link")
	wantScheduledChanges(t, moved)
	restored := f.call(admin, http.MethodPost, "/recycle-bin/"+b+"/restore", nil)
	f.want(restored, http.StatusOK, "restore intermediate milestone")
	moved = patch(a, "2026-01-13", `"2"`)
	f.want(moved, http.StatusOK, "edit after links reactivate")
	wantScheduledChanges(t, moved,
		scheduledChange{b, "2026-01-14", "2026-01-14", int(restored.body["version"].(float64)) + 1},
		scheduledChange{c, "2026-01-15", "2026-01-15", 2},
	)
}

func TestConcurrentSchedulingEdits(t *testing.T) {
	f := newFixture(t)
	first, second := f.person("admin"), f.person("admin")
	space := f.space("Concurrent scheduling")
	f.share(space, "user", first.id, "full") // A1.2: the two concurrent editors are explicit members
	f.share(space, "user", second.id, "full")
	a := scheduledItem(f, first, space, "First predecessor", "2026-01-05", "2026-01-05", 1)
	c := scheduledItem(f, first, space, "Second predecessor", "2026-01-05", "2026-01-05", 1)
	b := scheduledItem(f, first, space, "Successor", "2026-01-06", "2026-01-06", 1)
	addSchedulingDependency(f, first, a, b, "FS", 0)
	addSchedulingDependency(f, first, c, b, "FS", 0)

	type outcome struct {
		status int
		err    error
	}
	start := make(chan struct{})
	results := make(chan outcome, 2)
	for _, edit := range []struct {
		actor    *person
		id, date string
	}{{first, a, "2026-01-07"}, {second, c, "2026-01-08"}} {
		go func() {
			<-start
			body, _ := json.Marshal(map[string]string{"start_date": edit.date, "due_date": edit.date})
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPatch, f.srv.URL+"/api/v1/items/"+edit.id, bytes.NewReader(body))
			if err != nil {
				results <- outcome{err: err}
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", publicURL)
			req.Header.Set("If-Match", `"1"`)
			req.AddCookie(edit.actor.cookie)
			res, err := f.srv.Client().Do(req)
			if err != nil {
				results <- outcome{err: err}
				return
			}
			res.Body.Close()
			results <- outcome{status: res.StatusCode}
		}()
	}
	close(start)
	var statuses []int
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		statuses = append(statuses, result.status)
	}
	slices.Sort(statuses)
	if statuses[0] != http.StatusOK || (statuses[1] != http.StatusOK && statuses[1] != http.StatusConflict) {
		t.Fatalf("concurrent responses: %v, want successful serialization or one conflict", statuses)
	}

	left := f.call(first, http.MethodGet, "/items/"+a, nil)
	right := f.call(first, http.MethodGet, "/items/"+c, nil)
	tail := f.call(first, http.MethodGet, "/items/"+b, nil)
	f.want(left, http.StatusOK, "read first concurrent predecessor")
	f.want(right, http.StatusOK, "read second concurrent predecessor")
	f.want(tail, http.StatusOK, "read concurrent successor")
	latest := max(left.body["due_date"].(string), right.body["due_date"].(string))
	expected := map[string]string{"2026-01-07": "2026-01-08", "2026-01-08": "2026-01-09"}[latest]
	if expected == "" || tail.body["start_date"] != expected || tail.body["due_date"] != expected {
		t.Fatalf("concurrent edits did not commit as one schedule: predecessors %v %v, successor %v", left.body, right.body, tail.body)
	}
	stale := f.call(second, http.MethodPatch, "/items/"+b, map[string]string{"title": "Stale"}, "If-Match", `"1"`)
	f.want(stale, http.StatusConflict, "version from before shared successor cascade")
}

func TestSchedulingHiddenSuccessor(t *testing.T) {
	f := newFixture(t)
	admin, editor := f.person("admin"), f.person("regular")
	rootObserver, tailObserver := f.person("regular"), f.person("regular")
	source, hidden, visible := f.space("Source"), f.space("Hidden"), f.space("Visible")
	for _, planned := range []string{source, hidden, visible} {
		f.share(planned, "user", admin.id, "full") // A1.2: the planner owns every schedule in this scenario
	}
	f.share(source, "user", editor.id, "editor")
	f.share(visible, "user", editor.id, "read")
	f.share(source, "user", rootObserver.id, "read")
	f.share(visible, "user", tailObserver.id, "read")
	a := scheduledItem(f, admin, source, "Editable predecessor", "2026-01-05", "2026-01-05", 1)
	b := scheduledItem(f, admin, hidden, "Hidden successor", "2026-01-06", "2026-01-06", 1)
	c := scheduledItem(f, admin, visible, "Read-only successor", "2026-01-07", "2026-01-07", 1)
	for _, edge := range [][2]string{{a, b}, {b, c}} {
		dep := addSchedulingDependency(f, admin, edge[0], edge[1], "FS", 0)
		path := "/dependencies/" + dep.body["id"].(string)
		update := f.call(editor, http.MethodPatch, path, map[string]int{"lag": 1})
		f.want(update, http.StatusNotFound, "dependency edit with a hidden endpoint")
	}
	rootStream := openScheduleStream(f, rootObserver)
	tailStream := openScheduleStream(f, tailObserver)

	res := f.call(editor, http.MethodPatch, "/items/"+a, map[string]string{
		"start_date": "2026-01-08", "due_date": "2026-01-08",
	}, "If-Match", `"1"`)
	f.want(res, http.StatusOK, "reschedule through hidden successor")
	wantScheduledChanges(t, res, scheduledChange{c, "2026-01-12", "2026-01-12", 2})
	for _, observer := range []struct {
		stream     *bufio.Scanner
		item, room string
	}{{rootStream, a, source}, {tailStream, c, visible}} {
		var received bool
		for observer.stream.Scan() {
			data, ok := strings.CutPrefix(observer.stream.Text(), "data: {")
			if !ok {
				continue
			}
			var event map[string]any
			if err := json.Unmarshal([]byte("{"+data), &event); err != nil {
				t.Fatal(err)
			}
			if event["type"] != "item.updated" {
				continue
			}
			containers, _ := event["container_ids"].([]any)
			if event["id"] != observer.item || event["version"] != float64(2) || event["affected_items"] != nil ||
				len(containers) != 1 || containers[0] != observer.room {
				t.Fatalf("event disclosed another item or container: %v", event)
			}
			received = true
			break
		}
		if !received {
			t.Fatalf("missing event for %s: %v", observer.item, observer.stream.Err())
		}
	}
	f.want(f.call(editor, http.MethodGet, "/items/"+b, nil), http.StatusNotFound, "hidden successor remains hidden")
	hiddenItem := f.call(admin, http.MethodGet, "/items/"+b, nil)
	f.want(hiddenItem, http.StatusOK, "inspect hidden successor as admin")
	if hiddenItem.body["due_date"] != "2026-01-09" || hiddenItem.header.Get("ETag") != `"2"` {
		t.Fatalf("hidden successor did not shift: %v", hiddenItem.body)
	}
}

func openScheduleStream(f *fixture, observer *person) *bufio.Scanner {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(f.t.Context(), 5*time.Second)
	f.t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.srv.URL+"/api/v1/events", nil)
	if err != nil {
		f.t.Fatal(err)
	}
	req.AddCookie(observer.cookie)
	stream, err := f.srv.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { stream.Body.Close() })
	if stream.StatusCode != http.StatusOK || stream.Header.Get("Content-Type") != "text/event-stream" {
		f.t.Fatalf("event stream: %d %v", stream.StatusCode, stream.Header)
	}
	return bufio.NewScanner(stream.Body)
}

func TestScheduling500TaskPerformance(t *testing.T) {
	if os.Getenv("PMS_TEST_SCHEDULING_PERFORMANCE") != "1" {
		t.Skip("set PMS_TEST_SCHEDULING_PERFORMANCE=1 to measure p95 scheduling latency against PostgreSQL")
	}
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Scheduling benchmark")
	f.exec(`
WITH tasks AS (
  INSERT INTO items(type,title,created_by,start_date,due_date,duration_days,scope_state)
  SELECT 'task','Scheduling task ' || n,$1,DATE '2026-01-05',DATE '2026-01-05',1,'internal'
  FROM generate_series(1,500) n
  RETURNING id
)
INSERT INTO item_locations(item_id,space_id,rank)
SELECT id,$2,'U' FROM tasks`, admin.id, space)
	f.exec(`
WITH chain AS (
  SELECT id,lag(id) OVER (ORDER BY substring(title FROM '[0-9]+$')::int) AS predecessor FROM items
)
INSERT INTO item_dependencies(predecessor_id,successor_id,type)
SELECT predecessor,id,'FS' FROM chain WHERE predecessor IS NOT NULL`)
	head := f.queryID("SELECT id::text FROM items WHERE title='Scheduling task 1'")
	body := map[string]string{"start_date": "2026-01-06", "due_date": "2026-01-06"}
	const samples = 20
	durations := make([]time.Duration, 0, samples)
	for i := range samples + 1 {
		f.exec("UPDATE items SET start_date=DATE '2026-01-05',due_date=DATE '2026-01-05',version=1")
		start := time.Now()
		res := f.call(admin, http.MethodPatch, "/items/"+head, body, "If-Match", `"1"`)
		elapsed := time.Since(start)
		f.want(res, http.StatusOK, "schedule 500-task chain")
		if affected, _ := res.body["affected_items"].([]any); len(affected) != 499 {
			t.Fatalf("scheduled %d successors, want 499", len(affected))
		}
		if i > 0 { // Exclude the warm-up request.
			durations = append(durations, elapsed)
		}
	}
	slices.Sort(durations)
	p95 := durations[(95*samples+99)/100-1]
	t.Logf("500-task chain, %d HTTP samples: p95=%s", samples, p95)
	if p95 > 300*time.Millisecond {
		t.Errorf("500-task scheduling p95 = %s, want <= 300ms", p95)
	}
}
