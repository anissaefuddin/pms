package items

import (
	"testing"
	"time"
)

func TestCalculateProjectProgress(t *testing.T) {
	now := time.Date(2026, 11, 13, 23, 0, 0, 0, time.UTC)
	date := func(offset int) *time.Time {
		day := now.Truncate(24*time.Hour).AddDate(0, 0, offset)
		return &day
	}
	completed := taskProgress{completed: true, percent: 15, days: 1, due: date(-1)}
	partial := taskProgress{percent: 40, days: 3, due: date(3)}
	for _, tc := range []struct {
		name, method, status string
		due                  *time.Time
		tasks                []taskProgress
		percent              int
		risk                 string
	}{
		{"counts completed status", "task_count", "", nil, []taskProgress{completed, partial}, 50, "yellow"},
		{"weights by duration", "weighted_duration", "", nil, []taskProgress{completed, partial}, 55, "yellow"},
		{"averages manual values", "average_percent", "", nil, []taskProgress{completed, partial}, 70, "yellow"},
		{"empty project", "task_count", "", nil, nil, 0, "green"},
		{"empty weighted project", "weighted_duration", "", nil, nil, 0, "green"},
		{"empty average with own date", "average_percent", "", date(-1), nil, 0, "red"},
		{"milestone and missing duration weigh one", "weighted_duration", "", nil, []taskProgress{completed, {}, {percent: 50}}, 50, "green"},
		{"weighted maximum duration", "weighted_duration", "", nil, []taskProgress{{completed: true, days: 2147483647}, {days: 2147483647}}, 50, "green"},
		{"round to nearest", "task_count", "", nil, []taskProgress{completed, {}, {}}, 33, "green"},
		{"round exact half up", "average_percent", "", nil, []taskProgress{{percent: 50}, {percent: 51}}, 51, "green"},
		{"manual 100 is still overdue without completed status", "average_percent", "", nil, []taskProgress{{percent: 100, due: date(-1)}}, 100, "red"},
		{"due today is yellow", "task_count", "", date(0), nil, 0, "yellow"},
		{"due in four days is green", "task_count", "", nil, []taskProgress{{due: date(4)}}, 0, "green"},
		{"completed project risk green but percent still calculated", "task_count", "completed", date(-1), []taskProgress{{due: date(-1)}}, 0, "green"},
		{"cancelled project risk green", "average_percent", "cancelled", date(-1), []taskProgress{partial}, 40, "green"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotPercent, gotRisk := calculateProjectProgress(projectProgressInput{method: tc.method, status: tc.status, due: tc.due}, tc.tasks, now)
			if gotPercent != tc.percent || gotRisk != tc.risk {
				t.Fatalf("progress = %d/%s, want %d/%s", gotPercent, gotRisk, tc.percent, tc.risk)
			}
		})
	}
}
