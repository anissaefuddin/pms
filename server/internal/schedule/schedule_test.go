package schedule

import (
	"testing"
	"time"
)

func TestWeekdaysSkipsWeekend(t *testing.T) {
	start := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC) // Friday
	got := Weekdays([]Task{{ID: "one", Start: start, DurationDays: 2}})[0]
	if got.Finish.Weekday() != time.Monday {
		t.Fatalf("finish = %s, want Monday", got.Finish)
	}
}

func TestShiftFinishToStart(t *testing.T) {
	date := func(day int) time.Time {
		return time.Date(2026, time.November, day, 0, 0, 0, 0, time.UTC)
	}
	for _, tc := range []struct {
		name                                                     string
		predecessorFinish, successorStart, wantStart, wantFinish int
	}{
		{"overlap on weekday", 11, 10, 12, 13},
		{"predecessor finishes Friday", 13, 13, 16, 17},
		{"successor already starts later", 12, 19, 19, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			predecessor := ScheduledTask{Finish: date(tc.predecessorFinish)}
			successor := Task{ID: "successor", Start: date(tc.successorStart), DurationDays: 2}
			got := ShiftFinishToStart(predecessor, successor)
			want := ScheduledTask{ID: "successor", Start: date(tc.wantStart), Finish: date(tc.wantFinish)}
			if got != want {
				t.Fatalf("schedule = %+v, want %+v", got, want)
			}
		})
	}
}
