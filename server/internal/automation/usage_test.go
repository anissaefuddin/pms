package automation

import "testing"

// The usage states: warning from 80 percent, exhausted from 100, and the throttling reason follows the order rule, workspace, Space.
func TestUsageStatesAndThrottlingOrder(t *testing.T) {
	cases := []struct {
		used, limit int
		state       string
	}{{0, 5, "ok"}, {3, 5, "ok"}, {4, 5, "warning"}, {4, 5, "warning"}, {5, 5, "exhausted"}, {9, 5, "exhausted"}}
	for _, c := range cases {
		if got := newCounter(c.used, c.limit).State; got != c.state {
			t.Fatalf("%d of %d: got %s, want %s", c.used, c.limit, got, c.state)
		}
	}
	both := Usage{Rule: newCounter(5, 5), Workspace: newCounter(9, 5), Space: newCounter(9, 5)}
	if got := both.ThrottledBy(); got != "rule_quota" {
		t.Fatalf("the rule cap is named first: %s", got)
	}
	ws := Usage{Rule: newCounter(1, 5), Workspace: newCounter(5, 5), Space: newCounter(5, 5)}
	if got := ws.ThrottledBy(); got != "workspace_quota" {
		t.Fatalf("then the workspace cap: %s", got)
	}
	if got := (Usage{Rule: newCounter(4, 5), Space: newCounter(4, 5)}).ThrottledBy(); got != "" {
		t.Fatalf("a warning does not throttle: %s", got)
	}
}
