package httpserver_test

import (
	"fmt"
	"strings"
	"testing"
)

// DEF-010/DEF-017: one set of time entries, read by every consumer of money, for every kind of account.
func TestFinancialFiguresAgreeAcrossConsumers(t *testing.T) {
	f := newFixture(t)
	full, editor, limited, reader := f.person("regular"), f.person("regular"), f.person("collaborator"), f.person("regular")
	finance, adminEditor, adminNone := f.unlinkedAdmin(), f.unlinkedAdmin(), f.unlinkedAdmin()
	space := f.spaceWithoutAdmins("Finance")
	for p, role := range map[*person]string{full: "full", editor: "editor", limited: "limited", reader: "read", finance: "full", adminEditor: "editor"} {
		f.share(space, "user", p.id, role)
	}
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','P',$1) RETURNING id::text", finance.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", project, space)
	task := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','T',$1) RETURNING id::text", finance.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'A')", task, space, project)
	budget := f.call(full, "POST", "/spaces/"+space+"/custom-fields", map[string]any{"name": "Budget", "type": "currency", "config": map[string]any{"currency": "IDR", "project_budget": true}})
	f.want(budget, 201, "budget field")
	field := budget.body["id"].(string)
	day := 0
	entry := func(minutes int, status string, rate any, billing string, locked bool) {
		day++
		f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes,billing_type,status,locked,rate_snapshot) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)",
			task, editor.id, fmt.Sprintf("2026-11-%02d", day), minutes, billing, status, locked, rate)
	}
	// The rate trigger fills a missing rate only from hourly_rates, which stays empty here.
	entry(60, "approved", 6000, "billable", false)     // 6000
	entry(90, "approved", 4000, "billable", true)      // locked: 6000
	entry(30, "approved", 0, "billable", false)        // a real zero rate
	entry(60, "draft", 6000, "billable", false)        // not counted
	entry(60, "submitted", 6000, "billable", false)    // not counted
	entry(60, "rejected", 6000, "billable", false)     // not counted
	entry(60, "approved", 9000, "non_billable", false) // not billed
	entry(60, "draft", 6000, "billable", true)         // locked is not approval (DEC-06 Rev.1)
	entry(60, "submitted", 6000, "billable", true)     // locked is not approval
	entry(60, "rejected", 6000, "billable", true)      // locked is not approval
	const known = float64(12000)

	summary := func(p *person) map[string]any {
		t.Helper()
		got := f.call(p, "GET", "/items/"+project+"/time-summary", nil)
		f.want(got, 200, "summary")
		return got.body
	}
	query := map[string]any{"reporting_on": "time_entries", "metric": "actual_cost_minor", "group_by": "user"}
	analytics := func(p *person) reply { return f.call(p, "POST", "/analytics/query", query) }
	report := f.call(finance, "POST", "/reports", map[string]any{"name": "Cost", "definition": map[string]any{"layout": "table", "query": query}})
	f.want(report, 201, "report")
	export := func() string {
		status, _, body := f.rawGet(finance, "/reports/"+report.body["id"].(string)+"/export")
		if status != 200 {
			t.Fatalf("export %d", status)
		}
		return body
	}
	setBudget := func(value any) {
		f.exec("UPDATE items SET fields=jsonb_build_object($2::text,$3::numeric) WHERE id=$1", project, field, value)
	}
	budgetRow := func() map[string]any {
		rows := summary(finance)["budgets"].([]any)
		if len(rows) != 1 {
			t.Fatalf("budgets: %v", rows)
		}
		return rows[0].(map[string]any)
	}

	// Complete cost: the panel, analytics and the export give one number.
	got := summary(finance)
	if got["cost_status"] != "full" || got["actual_cost_minor"] != known || got["approved_minutes"] != float64(240) || got["pending_minutes"] != float64(240) || got["actual_minutes"] != float64(600) {
		t.Fatalf("panel: %v", got)
	}
	a := analytics(finance)
	if a.status != 200 || a.body["total"] != known || a.body["cost_status"] != "full" {
		t.Fatalf("analytics: %v", a.body)
	}
	if body := export(); !strings.Contains(body, "12000") || strings.Contains(body, "PARTIAL") {
		t.Fatalf("export: %s", body)
	}

	// Budget: none, zero, under, exact, over.
	for _, c := range []struct {
		name                  string
		budget                any
		variance, utilization any
	}{{"none", nil, nil, nil}, {"zero", 0, float64(-12000), nil}, {"under", 200, float64(8000), float64(60)}, {"exact", 120, float64(0), float64(100)}, {"over", 100, float64(-2000), float64(120)}} {
		setBudget(c.budget)
		row := budgetRow()
		if row["variance_minor"] != c.variance || row["utilization_percent"] != c.utilization || row["currency"] != "IDR" {
			t.Errorf("%s budget: %v", c.name, row)
		}
	}

	// Persona: cost reaches the administrator with Full only; nobody else is told a number, not even zero.
	for name, p := range map[string]*person{"Full without the finance role": full, "Editor": editor, "Limited": limited, "Read": reader} {
		s := summary(p)
		if s["cost_status"] != "unauthorized" || s["actual_cost_minor"] != nil || s["known_cost_minor"] != nil {
			t.Errorf("%s: %v", name, s)
		}
		if row := s["budgets"].([]any)[0].(map[string]any); row["variance_minor"] != nil || row["utilization_percent"] != nil {
			t.Errorf("%s variance: %v", name, row)
		}
		f.want(analytics(p), 403, name+" analytics")
	}
	f.want(f.call(adminNone, "GET", "/items/"+project+"/time-summary", nil), 404, "admin without a grant: panel")
	if a := analytics(adminNone); a.status != 200 || a.body["total"] != float64(0) || len(a.body["rows"].([]any)) != 0 {
		t.Errorf("admin without a grant sees no money: %v", a.body)
	}
	// Nor does their own draft turn the others' hidden cost into a row of zero (DEF-046).
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes,billing_type,status) VALUES ($1,$2,'2026-11-30',30,'billable','draft')", task, adminEditor.id)
	if a := f.call(adminEditor, "POST", "/analytics/query", map[string]any{"reporting_on": "time_entries", "metric": "actual_cost_minor"}); a.status != 200 || len(a.body["rows"].([]any)) != 0 || a.body["cost_status"] != "partial" {
		t.Errorf("a hidden cost plus an own draft is not a cost of zero: %v", a.body)
	}
	f.exec("DELETE FROM time_entries WHERE user_id=$1", adminEditor.id)
	s := summary(adminEditor)
	if s["cost_status"] != "partial" || s["actual_cost_minor"] != nil || s["known_cost_minor"] != float64(0) {
		t.Errorf("admin with Editor, panel: %v", s)
	}
	if a := analytics(adminEditor); a.body["cost_status"] != "partial" || a.body["total"] != float64(0) {
		t.Errorf("admin with Editor, analytics: %v", a.body)
	}

	// An approved billable entry without a rate: unknown, not free, everywhere.
	entry(45, "approved", nil, "billable", false)
	setBudget(200)
	got = summary(finance)
	if got["cost_status"] != "partial" || got["actual_cost_minor"] != nil || got["known_cost_minor"] != known {
		t.Fatalf("panel with a missing rate: %v", got)
	}
	if row := budgetRow(); row["variance_minor"] != nil || row["utilization_percent"] != nil {
		t.Fatalf("no variance on an incomplete cost: %v", row)
	}
	if a := analytics(finance); a.body["cost_status"] != "partial" || a.body["total"] != known {
		t.Fatalf("analytics with a missing rate: %v", a.body)
	}
	// A person whose approved time has no rate at all is not listed with a cost of zero.
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes,billing_type,status) VALUES ($1,$2,'2026-11-28',30,'billable','approved')", task, full.id)
	if a := analytics(finance); len(a.body["rows"].([]any)) != 1 {
		t.Fatalf("an unknown cost is shown as a row: %v", a.body["rows"])
	}
	// Nor when they also have time that costs nothing yet.
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes,billing_type,status) VALUES ($1,$2,'2026-11-29',30,'billable','draft')", task, full.id)
	if a := analytics(finance); len(a.body["rows"].([]any)) != 1 {
		t.Fatalf("an unknown cost plus a draft became a zero row: %v", a.body["rows"])
	}
	if body := export(); !strings.Contains(body, "PARTIAL") {
		t.Fatalf("export must say it is partial: %s", body)
	}
	// A later rate does not rewrite what was logged.
	f.want(f.call(finance, "POST", "/rates", map[string]any{"user_id": editor.id, "rate_minor": 100000, "effective_from": "2026-01-01"}), 201, "rate")
	if a := analytics(finance); a.body["total"] != known {
		t.Fatalf("history moved: %v", a.body)
	}
}
