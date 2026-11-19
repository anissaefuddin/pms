package httpserver_test

import (
	"fmt"
	"strings"
	"testing"
)

// A1.3: operational aggregate reporting. The shared engine computes the numbers; the operational path returns groups only,
// hides small groups, refuses drill-down and cost, and audits every privileged call.
func TestOperationalReportIsAggregateOnlyAndAudited(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	owner := f.person("regular")
	regular := f.person("regular")
	secret := f.space("Private finance")
	tiny := f.space("Tiny private")
	f.share(secret, "user", owner.id, "full")
	f.share(tiny, "user", owner.id, "full")
	for i := 0; i < 6; i++ {
		f.want(f.call(owner, "POST", "/items", map[string]any{"type": "task", "title": fmt.Sprintf("Secret %d", i), "locations": []map[string]any{{"space_id": secret}}}), 201, "seed secret")
	}
	for i := 0; i < 2; i++ {
		f.want(f.call(owner, "POST", "/items", map[string]any{"type": "task", "title": fmt.Sprintf("Tiny %d", i), "locations": []map[string]any{{"space_id": tiny}}}), 201, "seed tiny")
	}
	query := map[string]any{"reporting_on": "tasks", "metric": "count", "group_by": "space"}

	// Regular reports follow item access: the admin has no share on the private Spaces, so they see nothing there.
	regularReport := f.call(admin, "POST", "/analytics/query", query)
	f.want(regularReport, 200, "regular report")
	for _, row := range regularReport.body["rows"].([]any) {
		if strings.Contains(fmt.Sprint(row), "Private finance") {
			t.Fatalf("a regular report must not show a private Space to a Workspace admin: %v", row)
		}
	}

	// Operational: aggregate counts, the small Space suppressed, no item titles or ids anywhere in the body.
	op := f.call(admin, "POST", "/analytics/operational", query)
	f.want(op, 200, "operational aggregate")
	if op.body["operational"] != true || op.body["suppressed"] != float64(1) {
		t.Fatalf("operational result flags: %v", op.body)
	}
	rows := op.body["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["value"] != float64(6) {
		t.Fatalf("one group of six shown: %v", rows)
	}
	raw := fmt.Sprint(op.body)
	for _, leaked := range []string{"Secret 0", "Tiny 0"} {
		if strings.Contains(raw, leaked) {
			t.Fatalf("an item title leaked through an aggregate: %s", raw)
		}
	}
	if e := lastAudit(t, f, admin, "report.operational_read"); e["actor_id"] != admin.id {
		t.Fatalf("the privileged read is audited: %v", e)
	}

	// Negative: no drill-down, no cost, no derived metric, and no operational access without the capability.
	f.want(f.call(admin, "POST", "/analytics/operational", map[string]any{"reporting_on": "tasks", "metric": "count", "group_by": "space", "break_by": "status_group"}), 403, "no break-down")
	f.want(f.call(admin, "POST", "/analytics/operational", map[string]any{"reporting_on": "time_entries", "metric": "actual_cost_minor"}), 403, "no cost through operational")
	f.want(f.call(admin, "POST", "/analytics/operational", map[string]any{"reporting_on": "tasks", "metric": "completion_percent", "group_by": "space"}), 403, "no derived metric")
	f.want(f.call(regular, "POST", "/analytics/operational", query), 403, "a regular user has no operational access")
	if e := lastAudit(t, f, admin, "report.operational_denied"); e["actor_id"] != regular.id { // the last refusal is the regular user's
		t.Fatalf("refused privileged calls are audited: %v", e)
	}
}
