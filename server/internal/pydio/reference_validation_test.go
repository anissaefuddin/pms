package pydio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// PY-05..PY-09: structural validation of a canonical folder reference. No network request is made by ValidateLink.
func TestReferenceValidationIsStructural(t *testing.T) {
	base, _ := url.Parse("https://dms.example.com")
	roots := []string{"projects", "common-files/Clients"}
	accepted := []string{
		"https://dms.example.com/projects/PTM-2026/Drawings",
		"https://DMS.example.com/common-files/Clients/PHE",
	}
	rejected := map[string]string{
		"wrong host (PY-06)":        "https://evil.example.com/projects/x",
		"http scheme (PY-06)":       "http://dms.example.com/projects/x",
		"other port (PY-06)":        "https://dms.example.com:8443/projects/x",
		"traversal (PY-07)":         "https://dms.example.com/projects/../secret",
		"encoded traversal (PY-07)": "https://dms.example.com/projects/%2e%2e/secret",
		"encoded slash (PY-07)":     "https://dms.example.com/projects%2F..%2Fsecret",
		"backslash (PY-07)":         "https://dms.example.com/projects\\..\\secret",
		"empty segment (PY-07)":     "https://dms.example.com/projects//x",
		"sibling root (PY-08)":      "https://dms.example.com/projects-old/x",
		"sibling prefix (PY-08)":    "https://dms.example.com/projectsX/x",
		"credentials (PY-09)":       "https://user:secret@dms.example.com/projects/x",
		"token in query (PY-09)":    "https://dms.example.com/projects/x?token=abc",
		"fragment":                  "https://dms.example.com/projects/x#frag",
		"raw space":                 "https://dms.example.com/projects/a b",
		"outside any root":          "https://dms.example.com/private/x",
	}
	for _, link := range accepted {
		if _, err := ValidateLink(link, base, roots); err != nil {
			t.Fatalf("PY-05: %q must be accepted: %v", link, err)
		}
	}
	for name, link := range rejected {
		if _, err := ValidateLink(link, base, roots); err == nil {
			t.Fatalf("%s: %q must be rejected", name, link)
		}
	}
}

// Redirects are followed only to the configured https origin (B4-07). A redirect to another origin is refused, not followed.
func TestListingRefusesRedirectToAnotherOrigin(t *testing.T) {
	var hits int
	evil := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer evil.Close()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/steal", http.StatusFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "tok", []string{"projects"})
	c.HTTP.Transport = srv.Client().Transport
	if _, err := c.List(context.Background(), "projects/x"); err == nil {
		t.Fatal("a redirect to another origin must fail the listing")
	}
	if hits != 0 {
		t.Fatalf("the client followed a redirect to another origin (%d requests)", hits)
	}
}
