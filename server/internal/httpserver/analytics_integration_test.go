package httpserver_test

import (
	"fmt"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/analytics"
)

// rowsOf flattens a result to "group/break=value" strings so expectations read like the raw data.
func rowsOf(r reply) map[string]float64 {
	out := map[string]float64{}
	for _, raw := range r.body["rows"].([]any) {
		row := raw.(map[string]any)
		key := row["group"].(map[string]any)["key"].(string)
		if b, ok := row["break"].(map[string]any); ok {
			key += "/" + b["key"].(string)
		}
		out[key] = row["value"].(float64)
	}
	return out
}

func TestAnalyticsEngineMatchesRawDataAndPolicy(t *testing.T) {
	f := newFixture(t)
	admin, member, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	shared, secret := f.space("Analytics shared"), f.space("Analytics secret")
	f.share(shared, "user", member.id, "read")
	active := f.queryID("SELECT id::text FROM statuses WHERE grp='active' ORDER BY name LIMIT 1")
	done := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' ORDER BY name LIMIT 1")
	task := func(space, title, status, importance string, effort int, due string) string {
		id := f.queryID(`INSERT INTO items(type,title,created_by,status_id,importance,effort_minutes,due_date) VALUES ('task',$1,$2,$3,$4,$5,NULLIF($6,'')::date) RETURNING id::text`,
			title, admin.id, status, importance, effort, due)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
		return id
	}
	a := task(shared, "Late and mine", active, "high", 60, "2020-01-01")
	b := task(shared, "Done by admin", done, "normal", 30, "2020-02-01")
	task(shared, "Unassigned", active, "low", 10, "2099-01-01")
	d := task(secret, "Secret", active, "normal", 500, "2020-03-01")
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", a, member.id)
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", b, admin.id)
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES ($1,$2)", d, admin.id)
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes,billing_type) VALUES ($1,$2,'2026-05-04',90,'billable')", a, member.id)
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,'2026-05-05',30)", d, admin.id)

	query := func(p *person, q map[string]any) reply { return f.call(p, "POST", "/analytics/query", q) }
	count := map[string]any{"reporting_on": "tasks", "metric": "count", "space_ids": []string{shared, secret}}
	with := func(extra map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range count {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}

	// The total follows policy: the member cannot see the secret Space, the admin can.
	if r := query(member, count); r.body["total"] != 3.0 {
		t.Fatalf("member total = %v, want 3 (raw rows in the shared Space)", r.body["total"])
	}
	if r := query(admin, count); r.body["total"] != 4.0 {
		t.Fatalf("admin total = %v, want 4", r.body["total"])
	}
	if r := query(outsider, count); r.body["total"] != 0.0 {
		t.Fatalf("outsider total = %v, want 0", r.body["total"])
	}
	raw := f.queryID(`SELECT count(*)::text FROM items i JOIN item_locations l ON l.item_id=i.id WHERE l.space_id=$1 AND i.deleted_at IS NULL`, shared)
	if fmt.Sprint(query(member, count).body["total"]) != raw {
		t.Fatalf("total differs from raw count %s", raw)
	}

	byGroup := rowsOf(query(admin, with(map[string]any{"group_by": "status_group"})))
	if byGroup["active"] != 3 || byGroup["completed"] != 1 {
		t.Fatalf("by status group = %v", byGroup)
	}
	if got := rowsOf(query(member, with(map[string]any{"group_by": "importance"}))); got["high"] != 1 || got["normal"] != 1 || got["low"] != 1 {
		t.Fatalf("by importance = %v", got)
	}
	effort := rowsOf(query(admin, with(map[string]any{"metric": "effort_minutes", "group_by": "importance"})))
	if effort["high"] != 60 || effort["normal"] != 530 || effort["low"] != 10 {
		t.Fatalf("effort by importance = %v", effort)
	}
	broken := rowsOf(query(admin, with(map[string]any{"group_by": "status_group", "break_by": "assignee"})))
	if broken["active/"+member.id] != 1 || broken["active/"+admin.id] != 1 || broken["active/"] != 1 || broken["completed/"+admin.id] != 1 {
		t.Fatalf("status group broken by assignee = %v", broken)
	}
	bySpace := rowsOf(query(admin, with(map[string]any{"group_by": "space"})))
	if bySpace[shared] != 3 || bySpace[secret] != 1 {
		t.Fatalf("by space = %v", bySpace)
	}

	filter := func(p *person, filters map[string]any) float64 {
		t.Helper()
		r := query(p, with(map[string]any{"filters": filters}))
		f.want(r, 200, fmt.Sprint(filters))
		return r.body["total"].(float64)
	}
	for name, c := range map[string]struct {
		who     *person
		filters map[string]any
		want    float64
	}{
		"assigned to me":    {member, map[string]any{"assignees": []string{"me"}}, 1},
		"overdue":           {admin, map[string]any{"overdue": true}, 2}, // Late and mine + Secret; the done one is not active
		"unassigned":        {member, map[string]any{"unassigned": true}, 1},
		"importance high":   {admin, map[string]any{"importance": []string{"high"}}, 1},
		"due in Jan 2020":   {admin, map[string]any{"due_from": "2020-01-01", "due_to": "2020-01-31"}, 1},
		"effort 50..100":    {admin, map[string]any{"effort_min": 50, "effort_max": 100}, 1},
		"completed only":    {member, map[string]any{"status_groups": []string{"completed"}}, 1},
		"assignee by id":    {admin, map[string]any{"assignees": []string{admin.id}}, 2},
		"followed (none)":   {member, map[string]any{"followed": true}, 0},
		"overdue invisible": {member, map[string]any{"overdue": true}, 1},
	} {
		if got := filter(c.who, c.filters); got != c.want {
			t.Errorf("%s: got %v, want %v", name, got, c.want)
		}
	}

	// Time entries: the member sees only entries on Items the member may view.
	minutes := map[string]any{"reporting_on": "time_entries", "metric": "minutes", "space_ids": []string{shared, secret}}
	if r := query(member, minutes); r.body["total"] != 90.0 {
		t.Fatalf("member minutes = %v, want 90", r.body["total"])
	}
	if r := query(admin, minutes); r.body["total"] != 120.0 {
		t.Fatalf("admin minutes = %v, want 120", r.body["total"])
	}
	bill := map[string]any{"reporting_on": "time_entries", "metric": "billable_minutes", "group_by": "user", "space_ids": []string{shared, secret}}
	if got := rowsOf(query(admin, bill)); got[member.id] != 90 || got[admin.id] != 0 {
		t.Fatalf("billable by user = %v", got)
	}
	if r := query(admin, map[string]any{"reporting_on": "time_entries", "metric": "minutes", "space_ids": []string{shared, secret},
		"filters": map[string]any{"entry_from": "2026-05-05", "billing": "non_billable"}}); r.body["total"] != 30.0 {
		t.Fatalf("filtered minutes = %v", r.body["total"])
	}

	// Custom fields: a number field is summed, a single select groups by its option label.
	budget := f.queryID(`INSERT INTO custom_fields(space_id,name,type,options,position) VALUES ($1,'Budget','number','[]',0) RETURNING id::text`, shared)
	impact := f.queryID(`INSERT INTO custom_fields(space_id,name,type,options,position) VALUES ($1,'Impact','single_select','[{"id":"11111111-1111-4111-8111-111111111111","label":"High impact","color":"#3B82F6"},{"id":"22222222-2222-4222-8222-222222222222","label":"Low impact","color":"#3B82F6"}]',1) RETURNING id::text`, shared)
	f.exec(`UPDATE items SET fields=jsonb_build_object($2::text, 10.5, $3::text, '11111111-1111-4111-8111-111111111111') WHERE id=$1`, a, budget, impact)
	f.exec(`UPDATE items SET fields=jsonb_build_object($2::text, 4, $3::text, '22222222-2222-4222-8222-222222222222') WHERE id=$1`, b, budget, impact)
	f.exec(`UPDATE items SET fields=jsonb_build_object($2::text, 1000) WHERE id=$1`, d, budget) // the secret Space
	if r := query(member, with(map[string]any{"metric": "sum_field", "field_id": budget})); r.body["total"] != 14.5 {
		t.Fatalf("member budget = %v, want 14.5 (decimals kept, secret Space hidden)", r.body["total"])
	}
	// Budget is a field of the shared Space, so the value on the secret Space's item does not apply and is not counted (CF Space isolation).
	if r := query(admin, with(map[string]any{"metric": "sum_field", "field_id": budget})); r.body["total"] != 14.5 {
		t.Fatalf("admin budget = %v, want 14.5 (a Space field counts only where it applies)", r.body["total"])
	}
	labels := query(admin, with(map[string]any{"group_by": "field", "field_id": impact}))
	f.want(labels, 200, "group by single select")
	byLabel := map[string]float64{}
	for _, raw := range labels.body["rows"].([]any) {
		row := raw.(map[string]any)
		byLabel[row["group"].(map[string]any)["label"].(string)] = row["value"].(float64)
	}
	// The secret Space's item does not carry the shared Space's field, so only the unlabelled item of shared counts as "".
	if byLabel["High impact"] != 1 || byLabel["Low impact"] != 1 || byLabel[""] != 1 {
		t.Fatalf("by impact = %v", byLabel)
	}
	for name, q := range map[string]map[string]any{
		"sum without a field":        with(map[string]any{"metric": "sum_field"}),
		"field without sum or dim":   with(map[string]any{"field_id": budget}),
		"unknown field":              with(map[string]any{"metric": "sum_field", "field_id": "99999999-9999-4999-8999-999999999999"}),
		"sum over a select":          with(map[string]any{"metric": "sum_field", "field_id": impact}),
		"group by a number":          with(map[string]any{"group_by": "field", "field_id": budget}),
		"sum and group by one field": with(map[string]any{"metric": "sum_field", "group_by": "field", "field_id": budget}),
		"field id not a uuid":        with(map[string]any{"metric": "sum_field", "field_id": "budget"}),
		"field on time entries":      {"reporting_on": "time_entries", "metric": "minutes", "field_id": budget},
	} {
		f.want(query(admin, q), 422, name)
	}

	// The schema is closed: anything outside it is rejected before SQL exists.
	for name, q := range map[string]map[string]any{
		"unknown source":            {"reporting_on": "users", "metric": "count"},
		"unknown metric":            {"reporting_on": "tasks", "metric": "avg"},
		"time metric on tasks":      {"reporting_on": "tasks", "metric": "minutes"},
		"injection in group_by":     with(map[string]any{"group_by": "status_group; DROP TABLE items"}),
		"break_by without group_by": with(map[string]any{"break_by": "assignee"}),
		"same dimension twice":      with(map[string]any{"group_by": "assignee", "break_by": "assignee"}),
		"task dimension on time":    {"reporting_on": "time_entries", "metric": "minutes", "group_by": "assignee"},
		"task filter on time":       {"reporting_on": "time_entries", "metric": "minutes", "filters": map[string]any{"overdue": true}},
		"time filter on tasks":      with(map[string]any{"filters": map[string]any{"billing": "billable"}}),
		"bad date":                  with(map[string]any{"filters": map[string]any{"due_from": "01/01/2020"}}),
		"bad space id":              with(map[string]any{"space_ids": []string{"nope"}}),
		"bad status group":          with(map[string]any{"filters": map[string]any{"status_groups": []string{"x' OR '1'='1"}}}),
		"bad assignee":              with(map[string]any{"filters": map[string]any{"assignees": []string{"someone"}}}),
		"negative effort":           with(map[string]any{"filters": map[string]any{"effort_min": -1}}),
	} {
		f.want(query(admin, q), 422, name)
	}
	f.want(f.call(admin, "POST", "/analytics/query", map[string]any{"reporting_on": "tasks", "metric": "count", "sql": "select 1"}), 400, "unknown field")
	f.want(f.call(nil, "POST", "/analytics/query", count), 401, "anonymous")

	// A source over the row limit fails clearly instead of returning a partial number.
	defer func(old int) { analytics.MaxRows = old }(analytics.MaxRows)
	analytics.MaxRows = 2
	f.want(query(admin, count), 422, "too many rows")
}
