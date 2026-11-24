package duereminders

import "testing"

func TestEligibleMilestonesAndOverdueCadence(t *testing.T) {
	p := preference{DaysBefore: []int{3, 1, 0}, Overdue: true, OverdueEveryDays: 3}
	tests := []struct {
		offset int
		want   bool
		kind   string
	}{{3, true, "task_due_soon"}, {2, false, ""}, {1, true, "task_due_soon"}, {0, true, "task_due_today"}, {-1, true, "task_overdue"}, {-2, false, ""}, {-4, true, "task_overdue"}, {-7, true, "task_overdue"}}
	for _, tt := range tests {
		r := reminder{Offset: tt.offset}
		got := eligible(p, &r)
		if got != tt.want || r.Kind != tt.kind {
			t.Fatalf("offset %d: eligible=%v kind=%q", tt.offset, got, r.Kind)
		}
	}
}

func TestDigestLimitsEachGroup(t *testing.T) {
	var reminders []reminder
	for i := 0; i < 8; i++ {
		reminders = append(reminders, reminder{Title: "Late", Due: "2026-11-20", Offset: -1, Kind: "task_overdue"})
	}
	for i := 0; i < 7; i++ {
		reminders = append(reminders, reminder{Title: "Today", Due: "2026-11-23", Offset: 0, Kind: "task_due_today"})
	}
	args := digest(recipient{Email: "u@example.test", Locale: "id"}, reminders, "https://lens.test", "2026-11-23")
	if len(args.Data.Rows) != 10 || args.Data.Total != "15" {
		t.Fatalf("rows=%d total=%s", len(args.Data.Rows), args.Data.Total)
	}
}
