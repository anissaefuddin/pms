package httpserver_test

import (
	"net/http"
	"os"
	"testing"
	"time"
)

// TestBrowserM2Flows seeds a Space and runs the three M2 browser flows (S2.6.1) against the real API and PostgreSQL.
func TestBrowserM2Flows(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	owner, other := f.person("admin"), f.person("regular")
	f.exec("UPDATE users SET first_name='Lane',last_name='Owner' WHERE id=$1", owner.id)
	f.exec("UPDATE users SET first_name='Lane',last_name='Other' WHERE id=$1", other.id)
	space := f.space("M2 browser fixture")
	f.share(space, "user", owner.id, "full")
	f.share(space, "user", other.id, "editor")
	view := func(kind, name string) string {
		res := f.call(owner, http.MethodPost, "/views", map[string]any{"scope_space_id": space, "type": kind, "name": name, "visibility": "public"})
		f.want(res, http.StatusCreated, "create "+name)
		return res.body["id"].(string)
	}
	task := func(title string, fields map[string]any) string {
		id := customFieldItem(f, owner, space, title)
		if len(fields) > 0 {
			f.want(f.call(owner, http.MethodPatch, "/items/"+id, fields, "If-Match", `"1"`), http.StatusOK, "seed "+title)
		}
		return id
	}
	// Flow 1: 1 Mar 2027 is a Monday. The holiday (3 Mar) is added through the settings screen.
	pred := task("Pred task", map[string]any{"start_date": "2027-03-01", "due_date": "2027-03-01"})
	succ := task("Succ task", map[string]any{"start_date": "2027-03-02", "due_date": "2027-03-02"})
	f.want(f.call(owner, http.MethodPost, dependencyPath(succ), dependencyInput(pred, "FS")), http.StatusCreated, "dependency")
	// Flow 2 and 3
	fieldsTask := task("Fields task", nil)
	now := time.Now()
	day := func(d int) string {
		return time.Date(now.Year(), now.Month(), d, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
	cal := task("Calendar task", map[string]any{"start_date": day(10), "due_date": day(10)})
	laneA := task("Lane task", map[string]any{"assignee_ids": []string{owner.id}})
	laneB := task("Other lane task", map[string]any{"assignee_ids": []string{other.id}})

	runBrowserFixture(t, f, []string{"m2-live.spec.ts"},
		"PMS_FIXTURE_M2_COOKIE="+owner.cookie.String(),
		"PMS_FIXTURE_M2_SPACE="+space,
		"PMS_FIXTURE_M2_GANTT="+view("gantt", "Plan"),
		"PMS_FIXTURE_M2_TABLE="+view("table", "Grid"),
		"PMS_FIXTURE_M2_BOARD="+view("board", "Kanban"),
		"PMS_FIXTURE_M2_CALENDAR="+view("calendar", "Month"),
		"PMS_FIXTURE_M2_PRED="+pred, "PMS_FIXTURE_M2_SUCC="+succ,
		"PMS_FIXTURE_M2_FIELDS_TASK="+fieldsTask,
		"PMS_FIXTURE_M2_CAL_TASK="+cal, "PMS_FIXTURE_M2_CAL_DAY="+day(10),
		"PMS_FIXTURE_M2_LANE_A="+laneA, "PMS_FIXTURE_M2_LANE_B="+laneB,
		"PMS_FIXTURE_M2_OTHER="+other.id,
	)
}
