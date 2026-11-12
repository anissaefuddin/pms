package schedule

import (
	"errors"
	"math"
)

type Allocation struct {
	Unit  string
	Value int32
}

var (
	ErrNoAllocationCapacity = errors.New("positive effort requires positive daily allocation")
	ErrEffortDurationRange  = errors.New("calculated duration exceeds the supported range")
)

// EffortDuration rounds up to a whole working day. With no explicit allocation,
// the workspace's daily hours supply one day's capacity.
func EffortDuration(effortMinutes int32, dailyHours int, allocations []Allocation) (int32, error) {
	if effortMinutes == 0 {
		return 0, nil
	}
	dailyMinutes := int64(dailyHours) * 60
	capacity := int64(0)
	if len(allocations) == 0 {
		capacity = dailyMinutes * 100
	}
	for _, allocation := range allocations {
		switch allocation.Unit {
		case "percent":
			capacity += dailyMinutes * int64(allocation.Value)
		case "minutes_per_day":
			capacity += int64(allocation.Value) * 100
		}
	}
	if capacity == 0 {
		return 0, ErrNoAllocationCapacity
	}
	effort := int64(effortMinutes) * 100
	days := 1 + (effort-1)/capacity
	if days > math.MaxInt32 {
		return 0, ErrEffortDurationRange
	}
	return int32(days), nil
}
