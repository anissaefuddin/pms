package httpserver_test

import (
	"net/http"
	"testing"
)

// B3-17: public dashboards, forms and Gantt snapshots are off by default. With PMS_PUBLIC_LINKS unset, no public route exists, so
// a token that was issued earlier gets 404 as well. The fixture here keeps the production default.
func TestPublicLinksAreOffByDefault(t *testing.T) {
	f := newFixtureWith(t, 1000, nil)
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/public/dashboards/anytoken"},
		{http.MethodGet, "/public/forms/anytoken"},
		{http.MethodPost, "/public/forms/anytoken/submissions"},
		{http.MethodGet, "/public/gantt/anytoken"},
	} {
		res := f.call(nil, c.method, c.path, nil)
		if res.status != http.StatusNotFound {
			t.Fatalf("%s %s with public links off: status %d, want 404", c.method, c.path, res.status)
		}
	}
}
