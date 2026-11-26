package items

import (
	"errors"
	"testing"
)

// DEF-036 (review M1): automation records a refused assignment as denied, not as a failure it retries.
func TestAssignRefusalIsForbidden(t *testing.T) {
	if !errors.Is(ErrAssignForbidden, ErrForbidden) || errors.Is(ErrForbidden, ErrAssignForbidden) {
		t.Fatal("an assignment refusal is a kind of forbidden, not the other way round")
	}
}
