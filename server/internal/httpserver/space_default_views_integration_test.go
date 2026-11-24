package httpserver_test

import "testing"

// A new Space starts with Table (default), Board, Gantt and Calendar, so any of the four can be made the default.
func TestSpaceStartsWithFourViews(t *testing.T) {
	f := newFixture(t)
	space := f.space("Four default views")
	got := f.queryID("SELECT string_agg(type::text || ':' || is_default::text, ',' ORDER BY position) FROM views WHERE scope_space_id=$1", space)
	if want := "table:true,board:false,gantt:false,calendar:false"; got != want {
		t.Fatalf("space views = %q, want %q", got, want)
	}
}
