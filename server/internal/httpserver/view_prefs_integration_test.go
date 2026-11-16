package httpserver_test

import (
	"testing"

	"github.com/google/uuid"
)

// WA-D07: hidden views and tab order are personal: stored per user and per workspace, and never shared.
func TestViewPreferencesPersistPerUserAndWorkspace(t *testing.T) {
	f := newFixture(t)
	alice := f.person("regular")
	bob := f.person("regular")
	space := uuid.NewString()
	hiddenView := uuid.NewString()
	other := uuid.NewString()

	f.want(f.call(alice, "PUT", "/me/view-preferences/"+space, map[string]any{"hidden": []string{hiddenView, hiddenView}, "order": []string{other, hiddenView}}), 204, "save preferences")
	saved := f.call(alice, "GET", "/me", nil).body["view_prefs"].(map[string]any)
	entry := saved[space].(map[string]any)
	if hidden := entry["hidden"].([]any); len(hidden) != 1 || hidden[0] != hiddenView {
		t.Fatalf("hidden should be stored once: %v", hidden)
	}
	if order := entry["order"].([]any); len(order) != 2 || order[0] != other {
		t.Fatalf("order should keep the given sequence: %v", order)
	}
	if others := f.call(bob, "GET", "/me", nil).body["view_prefs"].(map[string]any); len(others) != 0 {
		t.Fatalf("another user must not see alice's preferences: %v", others)
	}
}

func TestViewPreferencesRejectBadInput(t *testing.T) {
	f := newFixture(t)
	member := f.person("regular")
	f.want(f.call(member, "PUT", "/me/view-preferences/not-a-uuid", map[string]any{"hidden": []string{}, "order": []string{}}), 422, "bad workspace id")
	f.want(f.call(member, "PUT", "/me/view-preferences/"+uuid.NewString(), map[string]any{"hidden": []string{"nope"}, "order": []string{}}), 422, "bad hidden id")
}
