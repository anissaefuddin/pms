package automation

import (
	"testing"

	"github.com/google/uuid"
)

// The context resolver (no database): origin first, then the single owning Space, otherwise a reasoned skip. Never a guess, never a
// fallback to another matched Space, and no rank, name, or id order.
func TestResolveExecutionSpace(t *testing.T) {
	a, b, c := uuid.MustParse("00000000-0000-4000-8000-00000000000a"), uuid.MustParse("00000000-0000-4000-8000-00000000000b"), uuid.MustParse("00000000-0000-4000-8000-00000000000c")
	targets := []uuid.UUID{a, b}
	cases := []struct {
		name    string
		origin  string
		spaces  []uuid.UUID
		exec    uuid.UUID
		reason  string
		matched int
	}{
		{"origin that the item is in and that is a target", b.String(), []uuid.UUID{a, b}, b, "", 2},
		{"origin outside the targets is never replaced", c.String(), []uuid.UUID{a, b, c}, uuid.Nil, ReasonOriginOutsideTargets, 2},
		{"origin the item is not in is not used", a.String(), []uuid.UUID{b}, uuid.Nil, ReasonOriginOutsideTargets, 1},
		{"unparsable origin is refused, not guessed", "not-a-uuid", []uuid.UUID{a}, uuid.Nil, ReasonOriginOutsideTargets, 1},
		{"no origin and one matching target is the owning Space", "", []uuid.UUID{b}, b, "", 1},
		{"no origin and two matching targets is ambiguous", "", []uuid.UUID{a, b}, uuid.Nil, ReasonAmbiguousContext, 2},
		{"no matching target at all", "", []uuid.UUID{c}, uuid.Nil, ReasonNotInTarget, 0},
	}
	for _, tc := range cases {
		exec, matched, reason := resolveExecutionSpace(tc.origin, targets, tc.spaces)
		if exec != tc.exec || reason != tc.reason || len(matched) != tc.matched {
			t.Fatalf("%s: got (%s, %d matched, %q), want (%s, %d, %q)", tc.name, exec, len(matched), reason, tc.exec, tc.matched, tc.reason)
		}
	}
	// The matched list is the same whatever order the targets come in, so the trail never depends on it.
	_, m1, _ := resolveExecutionSpace("", []uuid.UUID{a, b}, []uuid.UUID{a, b})
	_, m2, _ := resolveExecutionSpace("", []uuid.UUID{b, a}, []uuid.UUID{b, a})
	if m1[0] != m2[0] || m1[1] != m2[1] {
		t.Fatal("the matched list does not depend on the order of the targets")
	}
}
