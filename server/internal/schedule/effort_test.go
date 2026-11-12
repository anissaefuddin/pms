package schedule

import (
	"errors"
	"math"
	"testing"
)

func TestEffortDuration(t *testing.T) {
	for _, tc := range []struct {
		name        string
		effort      int32
		dailyHours  int
		allocations []Allocation
		days        int32
		err         error
	}{
		{"workspace fallback", 721, 6, nil, 3, nil},
		{"exact day", 360, 6, nil, 1, nil},
		{"percentage uses workspace capacity", 360, 6, []Allocation{{"percent", 50}}, 2, nil},
		{"mixed units", 451, 6, []Allocation{{"percent", 50}, {"minutes_per_day", 270}}, 2, nil},
		{"over allocation is accepted", 960, 6, []Allocation{{"percent", 200}}, 2, nil},
		{"zero effort", 0, 8, []Allocation{{"percent", 0}}, 0, nil},
		{"explicit zero capacity", 60, 8, []Allocation{{"percent", 0}}, 0, ErrNoAllocationCapacity},
		{"subminute capacity", 1, 1, []Allocation{{"percent", 1}}, 2, nil},
		{"maximum duration", math.MaxInt32, 1, []Allocation{{"minutes_per_day", 1}}, math.MaxInt32, nil},
		{"duration overflow", math.MaxInt32, 1, []Allocation{{"percent", 1}}, 0, ErrEffortDurationRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			days, err := EffortDuration(tc.effort, tc.dailyHours, tc.allocations)
			if days != tc.days || !errors.Is(err, tc.err) {
				t.Fatalf("got (%d, %v), want (%d, %v)", days, err, tc.days, tc.err)
			}
		})
	}
}
