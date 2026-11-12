package schedule

import (
	"errors"
	"testing"
	"time"
)

func jan(day int) *time.Time {
	if day == 0 {
		return nil
	}
	date := time.Date(2026, time.January, day, 0, 0, 0, 0, time.UTC)
	return &date
}

func TestPropagateCalendar(t *testing.T) {
	cal, err := NewCalendar([]int{1, 2, 3, 4, 5}, []time.Time{*jan(12)})
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	for _, tc := range []struct {
		name, kind                     string
		lag                            int
		pStart, pFinish, start, finish int
		duration                       *int
		wantStart, wantFinish          int
	}{
		{"FS after weekend and holiday", "FS", 0, 8, 9, 8, 9, nil, 13, 14},
		{"FS positive", "FS", 2, 8, 9, 8, 9, nil, 15, 16},
		{"FS minus one", "FS", -1, 8, 9, 8, 9, nil, 9, 13},
		{"FS minus two", "FS", -2, 8, 9, 7, 8, nil, 8, 9},
		{"FS Saturday finish", "FS", 0, 9, 10, 8, 9, nil, 13, 14},
		{"FS no pull", "FS", 0, 8, 9, 15, 16, nil, 15, 16},
		{"FS exact", "FS", 0, 8, 9, 13, 14, nil, 13, 14},
		{"SS zero", "SS", 0, 9, 13, 7, 8, nil, 9, 13},
		{"SS positive", "SS", 1, 9, 13, 7, 8, nil, 13, 14},
		{"SS negative", "SS", -2, 13, 14, 7, 8, nil, 8, 9},
		{"SS holiday", "SS", 0, 12, 14, 7, 8, nil, 13, 14},
		{"SS no pull", "SS", 0, 9, 13, 15, 16, nil, 15, 16},
		{"FF zero", "FF", 0, 9, 13, 7, 8, nil, 9, 13},
		{"FF positive", "FF", 1, 9, 13, 7, 8, nil, 13, 14},
		{"FF negative", "FF", -2, 9, 13, 6, 7, nil, 7, 8},
		{"FF holiday", "FF", 0, 9, 12, 7, 8, nil, 9, 13},
		{"FF no pull", "FF", 0, 9, 13, 15, 16, nil, 15, 16},
		{"SF zero", "SF", 0, 13, 14, 7, 8, nil, 9, 13},
		{"SF positive", "SF", 1, 13, 14, 7, 8, nil, 13, 14},
		{"SF negative", "SF", -1, 13, 14, 7, 8, nil, 8, 9},
		{"SF no pull", "SF", 0, 13, 14, 15, 16, nil, 15, 16},
		{"milestone", "FF", 0, 9, 13, 9, 9, &zero, 13, 13},
		{"milestone starts on Saturday", "FF", 0, 9, 13, 10, 10, &zero, 13, 13},
		{"unscheduled successor", "FS", 0, 8, 9, 0, 0, nil, 0, 0},
		{"unknown predecessor finish", "FS", 0, 9, 0, 7, 8, nil, 7, 8},
		{"start only successor", "SS", 0, 9, 0, 7, 0, nil, 9, 0},
		{"finish only successor", "FF", 0, 0, 13, 0, 9, nil, 0, 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dates := map[string]Dates{
				"p": {Start: jan(tc.pStart), Finish: jan(tc.pFinish)},
				"s": {Start: jan(tc.start), Finish: jan(tc.finish), Duration: tc.duration},
			}
			changes, err := cal.Propagate(dates, []Link{{From: "p", To: "s", Type: tc.kind, Lag: tc.lag}})
			if err != nil {
				t.Fatal(err)
			}
			wantChanges := tc.start != tc.wantStart || tc.finish != tc.wantFinish
			if (len(changes) == 1) != wantChanges {
				t.Fatalf("unexpected changes: %+v", changes)
			}
			got := dates["s"]
			if !sameDate(got.Start, jan(tc.wantStart)) || !sameDate(got.Finish, jan(tc.wantFinish)) {
				t.Fatalf("start/finish = %v/%v, want %v/%v", got.Start, got.Finish, jan(tc.wantStart), jan(tc.wantFinish))
			}
		})
	}
}

func sameDate(got, want *time.Time) bool {
	if got == nil || want == nil {
		return got == want
	}
	return got.Equal(*want)
}

func TestPropagateDiamondAndCycle(t *testing.T) {
	dates := map[string]Dates{
		"a": {Start: jan(8), Finish: jan(9)},
		"b": {Start: jan(8), Finish: jan(8)},
		"c": {Start: jan(8), Finish: jan(8)},
		"d": {Start: jan(8), Finish: jan(8)},
		"e": {Start: jan(19), Finish: jan(19)},
	}
	links := []Link{
		{From: "b", To: "d", Type: "FS"},
		{From: "c", To: "d", Type: "FS"},
		{From: "a", To: "c", Type: "FS", Lag: 1},
		{From: "a", To: "b", Type: "FS"},
		{From: "e", To: "d", Type: "SS"},
	}
	cal := DefaultCalendar()
	got, err := cal.Propagate(dates, links)
	if err != nil || len(got) != 3 || !sameDate(dates["d"].Start, jan(19)) {
		t.Fatalf("diamond with external predecessor: %+v, d=%+v, err=%v", got, dates["d"], err)
	}
	links = append(links, Link{From: "d", To: "a", Type: "FS"})
	if _, err := cal.Propagate(dates, links); !errors.Is(err, ErrCycle) {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestOffsetDateRange(t *testing.T) {
	cal := DefaultCalendar()
	for _, offset := range []int{-2147483648, 2147483647} {
		if _, err := cal.Offset(*jan(9), offset); !errors.Is(err, ErrDateRange) {
			t.Fatalf("offset %d returned %v", offset, err)
		}
	}
	cal, err := NewCalendar([]int{1, 2, 3, 4, 5}, []time.Time{lastDate})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := cal.Offset(lastDate, -1); err != nil || !got.Equal(lastDate.AddDate(0, 0, -1)) {
		t.Fatalf("backward offset from the final holiday = %v, %v", got, err)
	}
}
