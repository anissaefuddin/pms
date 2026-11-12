package schedule

import (
	"testing"
	"time"
)

func day(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }

func TestCalendar(t *testing.T) {
	// 2026-11-09 is a Monday.
	std := DefaultCalendar()
	mid, _ := NewCalendar([]int{1, 2, 3, 4, 5}, []time.Time{day("2026-11-11")}) // holiday Wednesday
	sat, _ := NewCalendar([]int{1, 2, 3, 4, 5, 6}, nil)
	sun, _ := NewCalendar([]int{0, 1}, nil)
	endHoliday, _ := NewCalendar([]int{1, 2, 3, 4, 5}, []time.Time{day("2026-11-13"), day("2026-11-16")}) // Fri and Mon
	for _, c := range []struct {
		name   string
		cal    Calendar
		start  string
		days   int
		finish string
	}{
		{"one day on a workday", std, "2026-11-09", 1, "2026-11-09"},
		{"start on Saturday moves to Monday", std, "2026-11-14", 1, "2026-11-16"},
		{"five days span a weekend", std, "2026-11-12", 5, "2026-11-18"},
		{"holiday in the middle is skipped", mid, "2026-11-09", 4, "2026-11-13"},
		{"holiday on the last day pushes finish", endHoliday, "2026-11-12", 2, "2026-11-17"},
		{"zero duration counts as one", std, "2026-11-09", 0, "2026-11-09"},
		{"Saturday works", sat, "2026-11-14", 2, "2026-11-16"},
		{"Sunday and Monday only", sun, "2026-11-10", 2, "2026-11-16"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.cal.Finish(day(c.start), c.days).Format(time.DateOnly); got != c.finish {
				t.Fatalf("Finish = %s, want %s", got, c.finish)
			}
		})
	}
	if n := mid.Duration(day("2026-11-09"), day("2026-11-13")); n != 4 {
		t.Fatalf("Duration with holiday = %d, want 4", n)
	}
	if n := std.Duration(day("2026-11-13"), day("2026-11-09")); n != 0 {
		t.Fatalf("Duration with finish before start = %d, want 0", n)
	}
	if _, err := NewCalendar(nil, nil); err != ErrNoWorkingDays {
		t.Fatalf("empty schedule error = %v", err)
	}
	if _, err := NewCalendar([]int{7}, nil); err == nil {
		t.Fatal("weekday 7 must be rejected")
	}
}
