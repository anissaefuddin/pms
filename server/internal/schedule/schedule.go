package schedule

import "time"

type Task struct {
	ID           string    `json:"id"`
	Start        time.Time `json:"start"`
	DurationDays int       `json:"durationDays"`
}

type ScheduledTask struct {
	ID     string    `json:"id"`
	Start  time.Time `json:"start"`
	Finish time.Time `json:"finish"`
}

// Weekdays is the M0 scheduling spike. M2 replaces the fixed calendar with SET-07.
func Weekdays(tasks []Task) []ScheduledTask {
	result := make([]ScheduledTask, 0, len(tasks))
	for _, task := range tasks {
		start := nextWorkday(task.Start)
		finish := start
		for days := 1; days < max(task.DurationDays, 1); {
			finish = nextWorkday(finish.AddDate(0, 0, 1))
			days++
		}
		result = append(result, ScheduledTask{ID: task.ID, Start: start, Finish: finish})
	}
	return result
}

// ShiftFinishToStart schedules a successor after the predecessor's inclusive finish date.
func ShiftFinishToStart(predecessor ScheduledTask, successor Task) ScheduledTask {
	earliestStart := nextWorkday(predecessor.Finish.AddDate(0, 0, 1))
	if successor.Start.Before(earliestStart) {
		successor.Start = earliestStart
	}
	return Weekdays([]Task{successor})[0]
}

func nextWorkday(date time.Time) time.Time {
	for date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		date = date.AddDate(0, 0, 1)
	}
	return date
}
