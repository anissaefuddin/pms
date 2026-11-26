package reportdigest

import (
	"fmt"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/analytics"
)

// DEF-029: the email keeps the largest rows and says how many it leaves out.
func TestRenderKeepsTheLargestRowsAndSaysWhatIsLeftOut(t *testing.T) {
	var result analytics.Result
	for i := 1; i <= 11; i++ {
		result.Rows = append(result.Rows, analytics.Row{Group: analytics.Label{Key: fmt.Sprintf("%02d", i), Label: fmt.Sprintf("Space %d", i)}, Value: float64(i)})
		result.Total += float64(i)
	}
	data := render("R", analytics.Query{Metric: "count"}, result)
	if len(data.Rows) != maxRows || data.More != 3 || data.Total != "66" || data.Rows[0].Label != "Space 11" {
		t.Fatalf("%+v", data)
	}
	result.Rows = result.Rows[:2]
	if data := render("R", analytics.Query{Metric: "count"}, result); data.More != 0 || len(data.Rows) != 2 {
		t.Fatalf("%+v", data)
	}
}
