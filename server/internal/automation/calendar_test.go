package automation

import (
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/schedule"
)

// AUT-T18 (part): a work-day offset steps over weekends, and a calendar-day offset does not.
func TestWorkDaysSkipWeekendsAndCalendarDaysDoNot(t *testing.T) {
	cal, err := schedule.NewCalendar([]int{1, 2, 3, 4, 5}, nil) // Monday to Friday
	if err != nil {
		t.Fatal(err)
	}
	monday := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC) // a Monday, checked below
	if monday.Weekday() != time.Monday {
		t.Fatalf("test date is not a Monday: %s", monday.Weekday())
	}
	friday := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	tuesday := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		from time.Time
		key  dueKey
		want time.Time
	}{
		{"one working day before a Monday is Friday", monday, dueKey{Days: -1, Work: true}, friday},
		{"one working day after a Friday is Monday", friday, dueKey{Days: 1, Work: true}, monday},
		{"two working days after a Monday is Wednesday", monday, dueKey{Days: 2, Work: true}, time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)},
		{"zero working days is the same day", tuesday, dueKey{Days: 0, Work: true}, tuesday},
		{"two calendar days after a Friday is Sunday", friday, dueKey{Days: 2}, time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		if got := dueDate(c.from, c.key, cal); !got.Equal(c.want) {
			t.Fatalf("%s: got %s, want %s", c.name, got.Format(time.DateOnly), c.want.Format(time.DateOnly))
		}
	}
}

// Deadlines are evaluated on the local calendar day of the workspace: 20:00 UTC on 1 December is already 2 December in Jakarta.
func TestLocalTodayFollowsTheWorkspaceZone(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 12, 1, 20, 0, 0, 0, time.UTC)
	if got := localToday(now, jakarta).Format(time.DateOnly); got != "2026-12-02" {
		t.Fatalf("local day in Jakarta: %s", got)
	}
	if got := localToday(now, time.UTC).Format(time.DateOnly); got != "2026-12-01" {
		t.Fatalf("UTC day: %s", got)
	}
}
