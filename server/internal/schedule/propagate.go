package schedule

import (
	"errors"
	"slices"
	"time"
)

var (
	ErrCycle     = errors.New("scheduling dependencies contain a cycle")
	ErrDateRange = errors.New("schedule exceeds the supported date range")
)

type Dates struct {
	Start, Finish *time.Time
	Duration      *int
}

type Link struct {
	From, To, Type string
	Lag            int
}

// Propagate updates dates in topological order and returns the IDs whose dates moved.
func (c Calendar) Propagate(dates map[string]Dates, links []Link) ([]string, error) {
	incoming := make(map[string][]Link)
	outgoing := make(map[string][]string)
	degree := make(map[string]int)
	for _, link := range links {
		incoming[link.To] = append(incoming[link.To], link)
		outgoing[link.From] = append(outgoing[link.From], link.To)
		degree[link.To]++
	}
	queue := make([]string, 0, len(dates))
	for id := range dates {
		if degree[id] == 0 {
			queue = append(queue, id)
		}
	}
	slices.Sort(queue)
	for i := 0; i < len(queue); i++ {
		for _, to := range outgoing[queue[i]] {
			degree[to]--
			if degree[to] == 0 {
				queue = append(queue, to)
			}
		}
	}
	if len(queue) != len(dates) {
		return nil, ErrCycle
	}
	var changed []string
	for _, id := range queue {
		current := dates[id]
		next, err := c.move(current, incoming[id], dates)
		if err != nil {
			return nil, err
		}
		if current.Start != next.Start || current.Finish != next.Finish {
			dates[id] = next
			changed = append(changed, id)
		}
	}
	return changed, nil
}

func (c Calendar) move(task Dates, incoming []Link, dates map[string]Dates) (Dates, error) {
	var start, finish *time.Time
	for _, link := range incoming {
		predecessor := dates[link.From]
		anchor := predecessor.Start
		if link.Type == "FS" || link.Type == "FF" {
			anchor = predecessor.Finish
		}
		bound, current := &start, task.Start
		if link.Type == "FF" || link.Type == "SF" {
			bound, current = &finish, task.Finish
		}
		if anchor == nil || current == nil {
			continue
		}
		lag := link.Lag
		if link.Type == "FS" && c.IsWorkday(*anchor) {
			lag++ // An inclusive finish occupies that working day.
		}
		earliest, err := c.Offset(*anchor, lag)
		if errors.Is(err, ErrDateRange) && lag < 0 {
			continue // A lower bound before the first representable date is already met.
		}
		if err != nil {
			return Dates{}, err
		}
		if current.Before(earliest) && (*bound == nil || (*bound).Before(earliest)) {
			*bound = &earliest
		}
	}
	if start == nil && finish == nil {
		return task, nil
	}
	if task.Start == nil {
		task.Finish = finish
		return task, nil
	}
	if task.Finish == nil {
		task.Start = start
		return task, nil
	}
	// Stored date pairs can predate the work calendar. Preserve their effective
	// working length, with a same-day window for milestones.
	span := 0
	if task.Duration == nil || *task.Duration != 0 {
		span = max(0, c.Duration(*task.Start, *task.Finish)-1)
	}
	newStart := c.NextWorkday(*task.Start)
	if start != nil {
		newStart = *start
	}
	// A start constraint may already satisfy the finish constraint. Check the
	// resulting finish as well to avoid pulling a legacy nonworking finish in.
	newFinish, err := c.Offset(newStart, span)
	if err != nil {
		return Dates{}, err
	}
	minFinish := *task.Finish
	if finish != nil {
		minFinish = *finish
	}
	if newFinish.Before(minFinish) {
		newStart, err = c.Offset(minFinish, -span)
		if err != nil {
			return Dates{}, err
		}
		newFinish, err = c.Offset(newStart, span)
		if err != nil {
			return Dates{}, err
		}
	}
	if newStart.After(*task.Start) {
		task.Start = &newStart
	}
	if newFinish.After(*task.Finish) {
		task.Finish = &newFinish
	}
	return task, nil
}

var firstDate = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
var lastDate = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)

// Offset first rounds forward to a workday, then moves by signed working days.
func (c Calendar) Offset(date time.Time, days int) (time.Time, error) {
	date = civilDay(date)
	if date.Year() < 1 || date.Year() > 9999 {
		return time.Time{}, ErrDateRange
	}
	date = c.NextWorkday(date)
	direction := 1
	bound := lastDate
	if days < 0 {
		direction, days, bound = -1, -days, firstDate
	}
	remaining := int((bound.Unix()-date.Unix())/86400) * direction
	if days > remaining {
		return time.Time{}, ErrDateRange
	}
	weekly := 0
	for _, works := range c.working {
		if works {
			weekly++
		}
	}
	if weeks := days / weekly; weeks > 0 {
		if weeks*7 > remaining {
			return time.Time{}, ErrDateRange
		}
		target := date.AddDate(0, 0, direction*weeks*7)
		days -= weeks * weekly
		for _, holiday := range c.holidays {
			inRange := holiday.After(date) && !holiday.After(target)
			if direction < 0 {
				inRange = holiday.Before(date) && !holiday.Before(target)
			}
			if inRange && c.working[holiday.Weekday()] {
				days++
			}
		}
		date = target
	}
	for days > 0 {
		date = date.AddDate(0, 0, direction)
		if direction > 0 && date.After(bound) || direction < 0 && date.Before(bound) {
			return time.Time{}, ErrDateRange
		}
		if c.IsWorkday(date) {
			days--
		}
	}
	if date.Year() < 1 || date.Year() > 9999 {
		return time.Time{}, ErrDateRange
	}
	return date, nil
}
