package httpserver_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

type variedTask struct {
	id         string
	space      int
	group      string
	importance string
	due        *time.Time
	effort     int
	assignees  []int
	followedBy map[int]bool
}

type variedEntry struct {
	task    int
	user    int
	date    time.Time
	minutes int
	billed  bool
}

// TestAnalyticsMatchesRawDataOnVariedFixtures (S5.3.1) generates random but reproducible workspaces, keeps its own
// copy of every row in Go, and checks the engine's numbers against sums computed straight from that copy, for
// an admin and for a member who can see only two of the three Spaces.
func TestAnalyticsMatchesRawDataOnVariedFixtures(t *testing.T) {
	for _, seed := range []int64{1, 7, 2026} {
		t.Run(fmt.Sprintf("seed %d", seed), func(t *testing.T) { checkVariedFixture(t, seed) })
	}
}

func checkVariedFixture(t *testing.T, seed int64) {
	f := newFixture(t)
	rng := rand.New(rand.NewSource(seed))
	people := []*person{f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")}
	admin, member := people[0], people[1]
	spaces := []string{f.space("Varied one"), f.space("Varied two"), f.space("Varied three")}
	f.share(spaces[0], "user", member.id, "read")
	f.share(spaces[1], "user", member.id, "editor")
	statuses := map[string]string{}
	rows, err := f.pool.Query(f.ctx, "SELECT DISTINCT ON (grp) grp::text, id::text FROM statuses ORDER BY grp, name")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var group, id string
		if err := rows.Scan(&group, &id); err != nil {
			t.Fatal(err)
		}
		statuses[group] = id
	}
	rows.Close()
	groups := []string{}
	for group := range statuses {
		groups = append(groups, group)
	}
	sortStrings(groups)
	today, _ := time.Parse(time.DateOnly, f.queryID("SELECT current_date::text"))
	importances := []string{"high", "normal", "low"}

	// Boundary days (today, yesterday, tomorrow and the edges of the combined filter below) are always present, so an
	// off-by-one in "overdue" or a date range cannot hide behind random data.
	edges := []int{0, -1, 1, -30, -31, -29, 30, 31, 29}
	var tasks []variedTask
	for n := 0; n < 150+rng.Intn(100); n++ {
		task := variedTask{space: rng.Intn(3), group: groups[rng.Intn(len(groups))], importance: importances[rng.Intn(3)], effort: []int{0, 30, 60, 120, 480}[rng.Intn(5)], followedBy: map[int]bool{}}
		if rng.Intn(5) > 0 {
			day := today.AddDate(0, 0, rng.Intn(240)-120)
			task.due = &day
		}
		if n < len(edges)*2 { // each edge day twice: once active, once not
			day := today.AddDate(0, 0, edges[n/2])
			task.due, task.group = &day, "active"
			if n%2 == 1 {
				task.group = groups[(n/2)%len(groups)]
			}
			task.effort, task.importance = 60, "high"
			if n%2 == 1 { // the odd one probes the effort limits 30 and 120 and the values just outside them
				task.effort, task.importance = []int{30, 120, 29, 121}[(n/2)%4], "low"
			}
		}
		var due any
		if task.due != nil {
			due = task.due.Format(time.DateOnly)
		}
		task.id = f.queryID(`INSERT INTO items(type,title,created_by,status_id,importance,effort_minutes,due_date) VALUES ('task',$1,$2,$3::uuid,$4,NULLIF($5,0),$6::date) RETURNING id::text`,
			fmt.Sprintf("Task %d", n), admin.id, statuses[task.group], task.importance, task.effort, due)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", task.id, spaces[task.space])
		for _, user := range rng.Perm(len(people))[:rng.Intn(3)] {
			task.assignees = append(task.assignees, user)
			f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", task.id, people[user].id)
		}
		if rng.Intn(3) == 0 {
			task.followedBy[1] = true
			f.exec("INSERT INTO item_followers(item_id,user_id) VALUES ($1,$2)", task.id, member.id)
		}
		tasks = append(tasks, task)
	}
	var entries []variedEntry
	seen := map[string]bool{}
	for n := 0; n < 200; n++ {
		e := variedEntry{task: rng.Intn(len(tasks)), user: rng.Intn(len(people)), date: today.AddDate(0, 0, -rng.Intn(60)), minutes: 15 + rng.Intn(465), billed: rng.Intn(2) == 0}
		key := fmt.Sprint(e.task, e.user, e.date) // one entry per person, task and day
		if seen[key] {
			continue
		}
		seen[key] = true
		billing := "non_billable"
		if e.billed {
			billing = "billable"
		}
		f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes,billing_type) VALUES ($1,$2,$3,$4,$5)", tasks[e.task].id, people[e.user].id, e.date.Format(time.DateOnly), e.minutes, billing)
		entries = append(entries, e)
	}

	// A1.3: the admin (viewer 0) sees only the Space it holds an explicit grant on (Space 0); the member sees Spaces 0 and 1.
	f.share(spaces[0], "user", people[0].id, "full")
	seesSpace := func(viewer int, space int) bool {
		if viewer == 0 {
			return space == 0
		}
		return space < 2
	}
	userKey := func(i int) string { return people[i].id }
	type expectation map[string]float64
	// taskSums groups the visible tasks the way the engine documents it: one row per assignee, "" when there is none.
	taskSums := func(viewer int, match func(variedTask) bool, keys func(variedTask) []string, value func(variedTask) float64) expectation {
		out := expectation{}
		for _, task := range tasks {
			if !seesSpace(viewer, task.space) || !match(task) {
				continue
			}
			for _, key := range keys(task) {
				out[key] += value(task)
			}
		}
		return out
	}
	all := func(variedTask) bool { return true }
	one := func(variedTask) float64 { return 1 }
	single := func(f func(variedTask) string) func(variedTask) []string {
		return func(t variedTask) []string { return []string{f(t)} }
	}
	month := func(t variedTask) string {
		if t.due == nil {
			return ""
		}
		return t.due.Format("2006-01")
	}
	assignees := func(t variedTask) []string {
		if len(t.assignees) == 0 {
			return []string{""}
		}
		keys := []string{}
		for _, a := range t.assignees {
			keys = append(keys, userKey(a))
		}
		return keys
	}
	hasAssignee := func(t variedTask, user int) bool {
		for _, a := range t.assignees {
			if a == user {
				return true
			}
		}
		return false
	}

	for viewer, who := range []*person{admin, member} {
		run := func(q map[string]any) map[string]float64 {
			t.Helper()
			q["space_ids"] = spaces
			r := f.call(who, "POST", "/analytics/query", q)
			f.want(r, 200, fmt.Sprint(q))
			return rowsOf(r)
		}
		check := func(name string, got map[string]float64, want expectation) {
			t.Helper()
			if len(got) != len(want) {
				t.Errorf("seed viewer %d %s: got %v, want %v", viewer, name, got, want)
				return
			}
			for key, value := range want {
				if got[key] != value {
					t.Errorf("viewer %d %s: %q = %v, want %v\n got %v\nwant %v", viewer, name, key, got[key], value, got, want)
				}
			}
		}
		count := func(extra map[string]any) map[string]any {
			q := map[string]any{"reporting_on": "tasks", "metric": "count"}
			for k, v := range extra {
				q[k] = v
			}
			return q
		}
		check("by status group", run(count(map[string]any{"group_by": "status_group"})), taskSums(viewer, all, single(func(t variedTask) string { return t.group }), one))
		check("by importance and status", run(count(map[string]any{"group_by": "importance", "break_by": "status_group"})),
			taskSums(viewer, all, single(func(t variedTask) string { return t.importance + "/" + t.group }), one))
		check("effort by assignee", run(count(map[string]any{"metric": "effort_minutes", "group_by": "assignee"})),
			taskSums(viewer, all, assignees, func(t variedTask) float64 { return float64(t.effort) }))
		check("by space", run(count(map[string]any{"group_by": "space"})), func() expectation {
			out := expectation{}
			for _, task := range tasks {
				if seesSpace(viewer, task.space) {
					out[spaces[task.space]]++
				}
			}
			return out
		}())
		check("active by due month", run(count(map[string]any{"group_by": "due_month", "filters": map[string]any{"status_groups": []string{"active"}}})),
			taskSums(viewer, func(t variedTask) bool { return t.group == "active" }, single(month), one))
		check("overdue", run(count(map[string]any{"filters": map[string]any{"overdue": true}})),
			taskSums(viewer, func(t variedTask) bool { return t.group == "active" && t.due != nil && t.due.Before(today) }, single(func(variedTask) string { return "" }), one))
		check("assigned to me", run(count(map[string]any{"filters": map[string]any{"assignees": []string{"me"}}})),
			taskSums(viewer, func(t variedTask) bool { return hasAssignee(t, viewer) }, single(func(variedTask) string { return "" }), one))
		check("unassigned", run(count(map[string]any{"filters": map[string]any{"unassigned": true}})),
			taskSums(viewer, func(t variedTask) bool { return len(t.assignees) == 0 }, single(func(variedTask) string { return "" }), one))
		if viewer == 1 {
			check("followed", run(count(map[string]any{"filters": map[string]any{"followed": true}})),
				taskSums(viewer, func(t variedTask) bool { return t.followedBy[1] }, single(func(variedTask) string { return "" }), one))
		}
		from, to := today.AddDate(0, 0, -30), today.AddDate(0, 0, 30)
		check("combined filters", run(count(map[string]any{"group_by": "importance", "filters": map[string]any{
			"importance": []string{"high", "low"}, "effort_min": 30, "effort_max": 120, "due_from": from.Format(time.DateOnly), "due_to": to.Format(time.DateOnly)}})),
			taskSums(viewer, func(t variedTask) bool {
				return (t.importance == "high" || t.importance == "low") && t.effort >= 30 && t.effort <= 120 && t.due != nil && !t.due.Before(from) && !t.due.After(to)
			}, single(func(t variedTask) string { return t.importance }), one))

		entrySums := func(match func(variedEntry) bool, value func(variedEntry) float64) expectation {
			out := expectation{}
			for _, e := range entries {
				if seesSpace(viewer, tasks[e.task].space) && match(e) {
					out[userKey(e.user)] += value(e)
				}
			}
			return out
		}
		minutes := func(e variedEntry) float64 { return float64(e.minutes) }
		timeQuery := func(extra map[string]any) map[string]any {
			q := map[string]any{"reporting_on": "time_entries", "metric": "minutes", "group_by": "user"}
			for k, v := range extra {
				q[k] = v
			}
			return q
		}
		check("minutes by user", run(timeQuery(nil)), entrySums(func(variedEntry) bool { return true }, minutes))
		check("billable minutes by user", run(timeQuery(map[string]any{"metric": "billable_minutes"})), entrySums(func(variedEntry) bool { return true }, func(e variedEntry) float64 {
			if e.billed {
				return float64(e.minutes)
			}
			return 0
		}))
		since := today.AddDate(0, 0, -20)
		check("minutes since", run(timeQuery(map[string]any{"filters": map[string]any{"entry_from": since.Format(time.DateOnly), "billing": "billable"}})),
			entrySums(func(e variedEntry) bool { return e.billed && !e.date.Before(since) }, minutes))
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
