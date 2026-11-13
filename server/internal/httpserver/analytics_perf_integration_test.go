package httpserver_test

import (
	"fmt"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/analytics"
)

// TestDashboardAndReportP95TenThousandItems (S5.3.1) measures every widget kind and every report template over HTTP
// on 10,000 Items with 20,000 time entries, for an admin and for a member who sees half of the Spaces (so the policy
// check does real work). Run it with PMS_PERF=1; it is gated because it adds about a minute to the suite.
func TestDashboardAndReportP95TenThousandItems(t *testing.T) {
	if os.Getenv("PMS_PERF") != "1" {
		t.Skip("set PMS_PERF=1 to measure widgets and reports on 10,000 items")
	}
	const budget = 1500 * time.Millisecond // the same budget PRD §6 gives Table and Board views
	f := newFixture(t)
	admin, member, other, third := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	spaces := []string{f.space("Perf one"), f.space("Perf two"), f.space("Perf three"), f.space("Perf four")}
	f.share(spaces[0], "user", member.id, "read")
	f.share(spaces[1], "user", member.id, "editor")
	f.exec(`WITH s AS (SELECT array_agg(id ORDER BY grp, name) ids FROM statuses),
dataset AS (
  INSERT INTO items(type,title,created_by,status_id,importance,effort_minutes,due_date)
  SELECT CASE WHEN n % 10 = 0 THEN 'project'::item_type ELSE 'task'::item_type END, 'Item ' || n, $1,
         s.ids[1 + n % cardinality(s.ids)], (ARRAY['high','normal','low'])[1 + n % 3], (n % 5) * 30,
         CASE WHEN n % 7 = 0 THEN NULL ELSE current_date + (n % 240 - 120) END
  FROM generate_series(1, 10000) n, s RETURNING id, seq
) INSERT INTO item_locations(item_id,space_id,rank)
SELECT id, (ARRAY[$2,$3,$4,$5]::uuid[])[1 + seq % 4], 'U' FROM dataset`, admin.id, spaces[0], spaces[1], spaces[2], spaces[3])
	users := []string{member.id, other.id, third.id}
	f.exec(`INSERT INTO item_assignees(item_id,user_id) SELECT id, ($1::uuid[])[1 + seq % 3] FROM items WHERE seq % 3 <> 0`, users)
	f.exec(`INSERT INTO item_assignees(item_id,user_id) SELECT id, ($1::uuid[])[1 + (seq + 1) % 3] FROM items WHERE seq % 5 = 0 AND (seq + 1) % 3 <> seq % 3`, users)
	f.exec(`INSERT INTO item_followers(item_id,user_id) SELECT id, $1 FROM items WHERE seq % 5 = 0`, member.id)
	f.exec(`INSERT INTO item_stars(user_id,item_id) SELECT $1, id FROM items WHERE seq % 4 = 0`, member.id)
	f.exec(`INSERT INTO time_entries(item_id,user_id,entry_date,minutes,billing_type)
SELECT i.id, ($1::uuid[])[1 + (i.seq + d) % 3], current_date - d, 30 + (d * 7) % 200, CASE WHEN d % 2 = 0 THEN 'billable' ELSE 'non_billable' END
FROM items i CROSS JOIN generate_series(0, 3) d WHERE i.seq % 2 = 0 AND i.type = 'task'`, users)
	for _, table := range []string{"items", "item_locations", "item_assignees", "item_followers", "item_stars", "time_entries"} {
		f.exec("ANALYZE " + table)
	}
	if n := f.queryID("SELECT count(*)::text FROM items"); n != "10000" {
		t.Fatalf("items = %s", n)
	}

	type result struct {
		name string
		p95  time.Duration
	}
	var results []result
	measure := func(name string, fn func()) {
		fn() // warm-up
		samples := make([]time.Duration, 20)
		for i := range samples {
			start := time.Now()
			fn()
			samples[i] = time.Since(start)
		}
		slices.Sort(samples)
		p95 := samples[18] // nearest rank for 20 samples
		results = append(results, result{name, p95})
		if p95 >= budget {
			t.Errorf("%s: p95 = %v, want < %v", name, p95, budget)
		}
	}
	ok := func(r reply, what string) {
		t.Helper()
		if r.status != 200 {
			t.Fatalf("%s: status %d %v", what, r.status, r.body)
		}
	}

	for _, who := range []struct {
		name string
		p    *person
	}{{"admin", admin}, {"member", member}} {
		board := f.call(who.p, "POST", "/dashboards", map[string]any{"name": "Perf " + who.name})
		f.want(board, 201, "dashboard")
		base := "/dashboards/" + board.body["id"].(string)
		query := func(extra map[string]any) map[string]any {
			q := map[string]any{"reporting_on": "tasks", "metric": "count"}
			for k, v := range extra {
				q[k] = v
			}
			return map[string]any{"query": q}
		}
		widgets := []struct {
			name, typ string
			config    map[string]any
		}{}
		for _, list := range []string{"todo_this_week", "to_review", "completed", "assigned_to_me"} {
			widgets = append(widgets, struct {
				name, typ string
				config    map[string]any
			}{"list " + list, "task_list", map[string]any{"list": list}})
		}
		for key := range analytics.ListPresets {
			if key == "todo_this_week" || key == "to_review" || key == "completed" || key == "assigned_to_me" {
				continue
			}
			widgets = append(widgets, struct {
				name, typ string
				config    map[string]any
			}{"template " + key, "task_list", map[string]any{"template": key}})
		}
		widgets = append(widgets,
			struct {
				name, typ string
				config    map[string]any
			}{"pie by status group", "pie", query(map[string]any{"group_by": "status_group"})},
			struct {
				name, typ string
				config    map[string]any
			}{"column by importance x status", "column", query(map[string]any{"group_by": "importance", "break_by": "status_group"})},
			struct {
				name, typ string
				config    map[string]any
			}{"bar effort by assignee", "bar", query(map[string]any{"metric": "effort_minutes", "group_by": "assignee"})},
			struct {
				name, typ string
				config    map[string]any
			}{"line by due month", "line", query(map[string]any{"group_by": "due_month"})},
			struct {
				name, typ string
				config    map[string]any
			}{"table by space", "table", query(map[string]any{"group_by": "space"})},
			struct {
				name, typ string
				config    map[string]any
			}{"indicator overdue", "indicator", query(map[string]any{"filters": map[string]any{"overdue": true}})},
			struct {
				name, typ string
				config    map[string]any
			}{"time by person", "table", map[string]any{"query": map[string]any{"reporting_on": "time_entries", "metric": "minutes", "group_by": "user"}}},
		)
		for i, w := range widgets {
			version := f.queryID("SELECT version::text FROM dashboards WHERE id=$1", board.body["id"])
			f.want(f.call(who.p, "POST", base+"/widgets", map[string]any{"name": w.name, "type": w.typ, "position": i, "config": w.config}, "If-Match", `"`+version+`"`), 201, w.name)
			id := f.queryID("SELECT id::text FROM widgets WHERE dashboard_id=$1 AND name=$2", board.body["id"], w.name)
			measure(who.name+" widget "+w.name, func() { ok(f.call(who.p, "GET", base+"/widgets/"+id+"/data", nil), w.name) })
		}
		for _, tpl := range analytics.Templates {
			created := f.call(who.p, "POST", "/reports", map[string]any{"name": who.name + " " + tpl.Key, "template": tpl.Key})
			f.want(created, 201, tpl.Key)
			path := "/reports/" + created.body["id"].(string)
			measure(who.name+" report "+tpl.Key, func() { ok(f.call(who.p, "GET", path+"/result", nil), tpl.Key) })
		}
		// The whole dashboard as the browser loads it: every widget, six requests at a time.
		measure(who.name+" whole dashboard ("+fmt.Sprint(len(widgets))+" widgets)", func() {
			rows, err := f.pool.Query(f.ctx, "SELECT id::text FROM widgets WHERE dashboard_id=$1", board.body["id"])
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for rows.Next() {
				var id string
				_ = rows.Scan(&id)
				ids = append(ids, id)
			}
			rows.Close()
			// A browser fetches six at a time.
			var wg sync.WaitGroup
			slots := make(chan struct{}, 6)
			for _, id := range ids {
				wg.Add(1)
				slots <- struct{}{}
				go func() {
					defer func() { <-slots; wg.Done() }()
					if r := f.call(who.p, "GET", base+"/widgets/"+id+"/data", nil); r.status != 200 {
						t.Errorf("widget %s: status %d", id, r.status)
					}
				}()
			}
			wg.Wait()
		})
	}
	slices.SortFunc(results, func(a, b result) int { return int(b.p95 - a.p95) })
	for _, r := range results {
		t.Logf("p95 %8.1f ms  %s", float64(r.p95.Microseconds())/1000, r.name)
	}
}
