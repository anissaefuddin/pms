package items

import "testing"

// QA-D-lane3-001: appending siblings must not grow the rank one character per child, or a folder stops accepting children at ~512 (capacity is now ~18,000 per parent).
func TestAppendRankStaysShortAndOrdered(t *testing.T) {
	last := ""
	for i := 0; i < 18000; i++ {
		next := rankAfter(last)
		if next <= last || len(next) > 512 || next[len(next)-1] == '0' {
			t.Fatalf("child %d: %q after %q", i, next, last)
		}
		last = next
	}
}
