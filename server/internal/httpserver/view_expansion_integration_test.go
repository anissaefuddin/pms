package httpserver_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// WA-D05: expansion is stored on the server per user and per view, and each user reads back only their own.
func TestViewExpansionPersistsPerUserAndView(t *testing.T) {
	f := newFixture(t)
	alice := f.person("regular")
	bob := f.person("regular")
	view := uuid.NewString()
	row := uuid.NewString()

	f.want(f.call(alice, "PUT", "/me/view-expansion/"+view, map[string]any{"rows": []string{row, row}, "gantt_collapsed": true}), 204, "save expansion")
	saved := f.call(alice, "GET", "/me", nil).body["view_expansion"].(map[string]any)
	entry := saved[view].(map[string]any)
	if rows := entry["rows"].([]any); len(rows) != 1 || rows[0] != row {
		t.Fatalf("rows should be stored once in order: %v", rows)
	}
	if entry["gantt_collapsed"] != true {
		t.Fatalf("gantt flag not stored: %v", entry)
	}
	if other := f.call(bob, "GET", "/me", nil).body["view_expansion"].(map[string]any); len(other) != 0 {
		t.Fatalf("another user must not see alice's expansion: %v", other)
	}
}

func TestViewExpansionRejectsBadInput(t *testing.T) {
	f := newFixture(t)
	member := f.person("regular")
	f.want(f.call(member, "PUT", "/me/view-expansion/not-a-uuid", map[string]any{"rows": []string{}}), 422, "bad view id")
	f.want(f.call(member, "PUT", "/me/view-expansion/"+uuid.NewString(), map[string]any{"rows": []string{"nope"}}), 422, "bad row id")
	tooMany := make([]string, 501)
	for i := range tooMany {
		tooMany[i] = uuid.NewString()
	}
	f.want(f.call(member, "PUT", "/me/view-expansion/"+uuid.NewString(), map[string]any{"rows": tooMany}), 422, fmt.Sprintf("%d rows", len(tooMany)))
}
