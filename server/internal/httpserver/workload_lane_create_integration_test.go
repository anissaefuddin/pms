package httpserver_test

import (
	"net/http"
	"testing"
)

// Batch 4 (W1, W3): assigning someone else on a direct create needs Space Admin; the Task's allocations are saved with it.
func TestDirectCreateAssignmentRulesAndAllocations(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	member, other, planner := f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Lane")
	f.share(space, "user", member.id, "editor")
	f.share(space, "user", planner.id, "editor")
	f.share(space, "user", other.id, "read")
	f.exec("INSERT INTO space_admins(space_id,user_id) VALUES ($1,$2)", space, planner.id)

	body := func(title string, assignee string) map[string]any {
		b := map[string]any{"type": "task", "title": title, "locations": []map[string]string{{"space_id": space}}}
		if assignee != "" {
			b["assignee_ids"] = []string{assignee}
		}
		return b
	}

	f.want(f.call(member, http.MethodPost, "/items", body("Other lane", other.id)), http.StatusForbidden, "member cannot assign someone else")
	f.want(f.call(member, http.MethodPost, "/items", body("Own lane", member.id)), http.StatusCreated, "member can assign self")
	f.want(f.call(planner, http.MethodPost, "/items", body("Planned lane", other.id)), http.StatusCreated, "space admin can assign someone else")
	f.want(f.call(admin, http.MethodPost, "/items", body("Admin lane", other.id)), http.StatusCreated, "workspace admin can assign someone else")

	// W1: allocations are created with the Task, atomically; no resource booking is created.
	alloc := map[string]any{
		"type": "task", "title": "Allocated lane", "locations": []map[string]string{{"space_id": space}}, "assignee_ids": []string{member.id},
		"assignee_allocations": []map[string]any{{"assignee_id": member.id, "allocation": map[string]any{"unit": "minutes_per_day", "value": 60}}},
	}
	made := f.call(planner, http.MethodPost, "/items", alloc)
	f.want(made, http.StatusCreated, "allocation create by space admin")
	id := made.body["id"].(string)
	var unit string
	var value int
	if err := f.pool.QueryRow(f.ctx, "SELECT allocation_unit, allocation_value FROM item_assignees WHERE item_id=$1 AND user_id=$2", id, member.id).Scan(&unit, &value); err != nil {
		t.Fatal(err)
	}
	if unit != "minutes_per_day" || value != 60 {
		t.Fatalf("saved allocation = %q %d, want minutes_per_day 60", unit, value)
	}
	if n := f.queryID("SELECT count(*)::text FROM resource_bookings WHERE project_id=$1", id); n != "0" {
		t.Fatalf("resource bookings created = %s, want 0", n)
	}
}
