package schedule

import (
	"errors"
	"time"
)

// Calendar answers "is this a working day" from the workspace work schedule (SET-07).
// Dates are UTC midnights; the time of day is ignored.
type Calendar struct {
	working  [7]bool // indexed by time.Weekday
	holidays map[string]time.Time
}

var ErrNoWorkingDays = errors.New("at least one working day is required")

// DefaultCalendar is Monday to Friday with no holidays.
func DefaultCalendar() Calendar {
	c, _ := NewCalendar([]int{1, 2, 3, 4, 5}, nil)
	return c
}

// NewCalendar takes weekdays as 0 (Sunday) to 6 (Saturday).
func NewCalendar(days []int, holidays []time.Time) (Calendar, error) {
	c := Calendar{holidays: map[string]time.Time{}}
	for _, d := range days {
		if d < 0 || d > 6 {
			return Calendar{}, errors.New("working day must be 0 to 6")
		}
		c.working[d] = true
	}
	if c.working == [7]bool{} {
		return Calendar{}, ErrNoWorkingDays
	}
	for _, h := range holidays {
		c.holidays[h.Format(time.DateOnly)] = civilDay(h)
	}
	return c, nil
}

func (c Calendar) IsWorkday(d time.Time) bool {
	_, holiday := c.holidays[d.Format(time.DateOnly)]
	return c.working[d.Weekday()] && !holiday
}

// NextWorkday returns d itself when it is a working day. Holidays are finite and at least one weekday works, so it ends.
func (c Calendar) NextWorkday(d time.Time) time.Time {
	for !c.IsWorkday(d) {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

// Finish is the inclusive last day of a task that starts on the first working day on or after start
// and lasts durationDays working days (minimum 1).
func (c Calendar) Finish(start time.Time, durationDays int) time.Time {
	finish := c.NextWorkday(start)
	for n := 1; n < durationDays; n++ {
		finish = c.NextWorkday(finish.AddDate(0, 0, 1))
	}
	return finish
}

// Duration counts the working days from start to finish, both included. It is 0 when finish is before start.
func (c Calendar) Duration(start, finish time.Time) int {
	start, finish = civilDay(start), civilDay(finish)
	if finish.Before(start) {
		return 0
	}
	days := int((finish.Unix()-start.Unix())/86400) + 1
	weeks := days / 7
	n := 0
	for _, works := range c.working {
		if works {
			n += weeks
		}
	}
	for i := 0; i < days%7; i++ {
		if c.working[(int(start.Weekday())+i)%7] {
			n++
		}
	}
	for _, holiday := range c.holidays {
		if !holiday.Before(start) && !holiday.After(finish) && c.working[holiday.Weekday()] {
			n--
		}
	}
	return n
}

func civilDay(date time.Time) time.Time {
	year, month, day := date.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
