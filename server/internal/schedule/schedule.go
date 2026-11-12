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

// Weekdays schedules on Monday to Friday. Use Calendar.Finish for the workspace calendar.
func Weekdays(tasks []Task) []ScheduledTask {
	cal := DefaultCalendar()
	result := make([]ScheduledTask, 0, len(tasks))
	for _, task := range tasks {
		start := cal.NextWorkday(task.Start)
		result = append(result, ScheduledTask{ID: task.ID, Start: start, Finish: cal.Finish(start, task.DurationDays)})
	}
	return result
}

// ShiftFinishToStart schedules a successor after the predecessor's inclusive finish date.
func ShiftFinishToStart(predecessor ScheduledTask, successor Task) ScheduledTask {
	earliestStart := DefaultCalendar().NextWorkday(predecessor.Finish.AddDate(0, 0, 1))
	if successor.Start.Before(earliestStart) {
		successor.Start = earliestStart
	}
	return Weekdays([]Task{successor})[0]
}
