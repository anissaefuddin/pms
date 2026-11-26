package httpserver_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// DEF-021: no registered route answers 500 to a malformed id, whatever the method.
func TestMalformedIDsNeverCauseServerErrors(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	f.space("Sweep")
	files, _ := filepath.Glob("*.go")
	route := regexp.MustCompile(`"(GET|POST|PUT|PATCH|DELETE) /api/v1(/[^"]*\{[^"]*)"`)
	param := regexp.MustCompile(`\{[^}]+\}`)
	seen := map[string]bool{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range route.FindAllStringSubmatch(string(src), -1) {
			for _, bad := range []string{"undefined", "null", "00000000-0000-0000-0000-00000000zzzz"} {
				path := param.ReplaceAllString(m[2], bad)
				if seen[m[1]+path] || strings.HasPrefix(path, "/public/") {
					continue
				}
				seen[m[1]+path] = true
				var body any
				if m[1] != "GET" && m[1] != "DELETE" {
					body = map[string]any{}
				}
				if got := f.call(admin, m[1], path, body, "If-Match", `"1"`); got.status >= 500 {
					t.Errorf("%s %s: %d %v", m[1], path, got.status, got.body)
				}
			}
		}
	}
	if len(seen) < 100 {
		t.Fatalf("the sweep found only %d routes", len(seen))
	}
}
