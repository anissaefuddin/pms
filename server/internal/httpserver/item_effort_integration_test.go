package httpserver_test

import (
	"net/http"
	"testing"
)

func TestItemEffortAndAllocation(t *testing.T) {
	f := newFixture(t)
	admin, reader := f.person("admin"), f.person("regular")
	first, second := f.person("regular"), f.person("regular")
	space := f.space("Effort")
	f.share(space, "user", reader.id, "read")
	f.want(f.call(admin, http.MethodPut, "/settings/work-schedule", map[string]any{
		"working_days": []int{1, 2, 3, 4, 5}, "daily_hours": 6,
		"exceptions": []map[string]string{{"date": "2026-01-12", "name": "Office closed"}},
	}, "If-Match", `"1"`), http.StatusOK, "custom workspace capacity and holiday")
	a := scheduledItem(f, admin, space, "Estimate", "2026-01-09", "2026-01-09", 1)
	b := scheduledItem(f, admin, space, "Successor", "2026-01-13", "2026-01-13", 1)
	addSchedulingDependency(f, admin, a, b, "FS", 0)
	path := "/items/" + a
	patch := func(version string, body map[string]any) reply {
		return f.call(admin, http.MethodPatch, path, body, "If-Match", version)
	}
	allocation := func(id, unit string, value int) map[string]any {
		return map[string]any{"assignee_id": id, "allocation": map[string]any{"unit": unit, "value": value}}
	}
	estimate := map[string]any{
		"effort_minutes": 720,
		"assignee_ids":   []string{first.id, second.id},
		"assignee_allocations": []map[string]any{
			allocation(first.id, "percent", 50),
			{"assignee_id": second.id, "allocation": map[string]any{"Unit": "minutes_per_day", "Value": 270}},
		},
	}
	f.want(f.call(reader, http.MethodPatch, path, estimate, "If-Match", `"1"`), http.StatusForbidden, "reader cannot edit estimate")
	saved := patch(`"1"`, estimate)
	f.want(saved, http.StatusOK, "save mixed allocations totaling 125 percent")
	if saved.body["effort_minutes"] != float64(720) || saved.body["duration_days"] != float64(1) || saved.body["due_date"] != "2026-01-09" {
		t.Fatalf("saving effort must leave schedule unchanged: %v", saved.body)
	}
	wantScheduledChanges(t, saved)
	assertAllocation := func(item reply, assignee string, expectedUnit string, expectedValue int) {
		t.Helper()
		for _, raw := range item.body["assignees"].([]any) {
			member := raw.(map[string]any)
			if member["id"] != assignee {
				continue
			}
			if expectedUnit == "" && member["allocation"] == nil {
				return
			}
			alloc, _ := member["allocation"].(map[string]any)
			if alloc["unit"] != expectedUnit || alloc["value"] != float64(expectedValue) {
				t.Fatalf("allocation for %s = %v", assignee, alloc)
			}
			return
		}
		t.Fatalf("assignee %s missing", assignee)
	}
	assertAllocation(saved, first.id, "percent", 50)
	assertAllocation(saved, second.id, "minutes_per_day", 270)
	f.want(patch(`"1"`, map[string]any{"effort_minutes": 960}), http.StatusConflict, "stale estimate")
	calculated := patch(`"2"`, map[string]any{"effort_minutes": 960, "recalculate_duration": true})
	f.want(calculated, http.StatusOK, "explicit recalc uses the new effort and existing allocation")
	if calculated.body["effort_minutes"] != float64(960) || calculated.body["duration_days"] != float64(3) ||
		calculated.body["due_date"] != "2026-01-14" || calculated.header.Get("ETag") != `"3"` {
		t.Fatalf("duration should be three workdays excluding weekend and holiday: %+v", calculated)
	}
	wantScheduledChanges(t, calculated, scheduledChange{b, "2026-01-15", "2026-01-15", 2})

	f.want(patch(`"3"`, map[string]any{
		"assignee_ids": []string{first.id}, "assignee_allocations": []any{allocation(second.id, "percent", 25)},
	}), http.StatusUnprocessableEntity, "allocation for removed assignee rolls back membership change")
	retained := patch(`"3"`, map[string]any{"assignee_ids": []string{first.id}})
	f.want(retained, http.StatusOK, "drop one assignee")
	assertAllocation(retained, first.id, "percent", 50)
	readded := patch(`"4"`, map[string]any{"assignee_ids": []string{first.id, second.id}})
	f.want(readded, http.StatusOK, "re-add removed assignee without old allocation")
	assertAllocation(readded, second.id, "", 0)
	if got := f.queryID("SELECT count(*)::text FROM item_followers WHERE item_id=$1 AND user_id IN ($2,$3)", a, first.id, second.id); got != "2" {
		t.Fatalf("assignees should still be added as followers, got %s", got)
	}
	for name, fields := range map[string]map[string]any{
		"negative effort":               {"effort_minutes": -1},
		"unknown unit":                  {"assignee_allocations": []any{allocation(first.id, "hours", 2)}},
		"negative allocation":           {"assignee_allocations": []any{allocation(first.id, "percent", -5)}},
		"missing value":                 {"assignee_allocations": []any{map[string]any{"assignee_id": first.id, "allocation": map[string]string{"unit": "percent"}}}},
		"duplicate assignee":            {"assignee_allocations": []any{allocation(first.id, "percent", 10), allocation(first.id, "percent", 20)}},
		"explicit duration plus recalc": {"duration_days": 1, "recalculate_duration": true},
		"zero capacity rolls back effort and allocation": {"effort_minutes": 120, "assignee_allocations": []any{allocation(first.id, "percent", 0)}, "recalculate_duration": true},
	} {
		f.want(patch(`"5"`, fields), http.StatusUnprocessableEntity, name)
	}
	current := f.call(reader, http.MethodGet, path, nil)
	f.want(current, http.StatusOK, "reader sees committed estimate")
	assertAllocation(current, first.id, "percent", 50)
	if current.body["effort_minutes"] != float64(960) || current.header.Get("ETag") != `"5"` {
		t.Fatalf("invalid edit changed the committed version or effort: %+v", current)
	}

	cleared := patch(`"5"`, map[string]any{
		"effort_minutes": nil, "assignee_allocations": []map[string]any{{"assignee_id": first.id, "allocation": nil}},
	})
	f.want(cleared, http.StatusOK, "clear the estimate and allocation")
	assertAllocation(cleared, first.id, "", 0)
	if cleared.body["effort_minutes"] != nil {
		t.Fatalf("effort should be cleared: %v", cleared.body)
	}
	if count := f.queryID("SELECT count(*)::text FROM activity WHERE item_id=$1 AND changes ? 'assignee_allocations'", a); count != "3" {
		t.Fatalf("expected three allocation changes in activity, got %s", count)
	}
}

func TestEffortDurationWithoutDates(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Undated effort")
	item := f.call(admin, http.MethodPost, "/items", map[string]any{
		"type": "task", "title": "Undated task", "effort_minutes": 481,
		"locations": []map[string]string{{"space_id": space}},
	})
	f.want(item, http.StatusCreated, "create with effort")
	path := "/items/" + item.body["id"].(string)
	derived := f.call(admin, http.MethodPatch, path, map[string]bool{"recalculate_duration": true}, "If-Match", `"1"`)
	f.want(derived, http.StatusOK, "calculate with workspace fallback")
	if derived.body["duration_days"] != float64(2) || derived.body["start_date"] != nil || derived.body["due_date"] != nil {
		t.Fatalf("expected two days and no new dates: %v", derived.body)
	}
	f.want(f.call(admin, http.MethodPatch, path, map[string]bool{"is_milestone": true}, "If-Match", `"2"`), http.StatusOK, "convert to milestone")
	f.want(f.call(admin, http.MethodPatch, path, map[string]bool{"recalculate_duration": true}, "If-Match", `"3"`), http.StatusUnprocessableEntity, "milestone duration is fixed")
}
