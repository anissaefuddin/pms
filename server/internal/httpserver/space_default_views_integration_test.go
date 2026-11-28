package httpserver_test

import "testing"

// A new Space starts with Table, Board, Gantt, Calendar and Files (default), so any of them can be made the default.
func TestSpaceStartsOnFiles(t *testing.T) {
	f := newFixture(t)
	space := f.space("Default views")
	got := f.queryID("SELECT string_agg(type::text || ':' || is_default::text, ',' ORDER BY position) FROM views WHERE scope_space_id=$1", space)
	if want := "table:false,board:false,gantt:false,calendar:false,files:true"; got != want {
		t.Fatalf("space views = %q, want %q", got, want)
	}
}
