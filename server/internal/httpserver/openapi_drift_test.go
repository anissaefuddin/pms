package httpserver

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// DEF-026: every public route is in the contract and every contract operation has a route. A route kept out on purpose is listed
// here with its reason.
func TestOpenAPIMatchesRegisteredRoutes(t *testing.T) {
	internal := map[string]string{}
	doc, err := openapi3.NewLoader().LoadFromFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	param := regexp.MustCompile(`\{[^}]*\}`)
	norm := func(method, path string) string { return method + " " + param.ReplaceAllString(path, "{}") }
	spec := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			spec[norm(method, path)] = true
		}
	}
	// Routes are read from the source: patterns written whole, and those built from a base or a method list.
	code := map[string]bool{}
	files, _ := filepath.Glob("*.go")
	literal := regexp.MustCompile(`"(GET|POST|PUT|PATCH|DELETE) /api/v1(/[^"]*)"`)
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range literal.FindAllStringSubmatch(string(src), -1) {
			code[norm(m[1], strings.TrimSuffix(m[2], "/"))] = true
		}
	}
	// Built from a prefix or a method list, so not written as one literal in the source. Each must still be in the contract.
	for _, built := range []string{"GET /attachments/{}/versions/{}/preview", "GET /attachments/{}/versions/{}/preview/{}", "GET /attachments/{}/versions/{}/annotations",
		"POST /attachments/{}/versions/{}/annotations", "PATCH /attachments/{}/versions/{}/annotations/{}", "DELETE /attachments/{}/versions/{}/annotations/{}",
		"GET /time-entries/{}", "PATCH /time-entries/{}", "DELETE /time-entries/{}",
		"GET /resource-directory", "POST /resource-directory", "PUT /resource-directory", "DELETE /resource-directory",
		"GET /scim/v2/ServiceProviderConfig", "GET /scim/v2/ResourceTypes", "GET /scim/v2/Schemas", "GET /scim/v2/Users", "POST /scim/v2/Users", "GET /scim/v2/Users/{}",
		"PUT /scim/v2/Users/{}", "PATCH /scim/v2/Users/{}", "DELETE /scim/v2/Users/{}", "GET /scim/v2/Groups", "POST /scim/v2/Groups", "GET /scim/v2/Groups/{}",
		"PUT /scim/v2/Groups/{}", "PATCH /scim/v2/Groups/{}", "DELETE /scim/v2/Groups/{}"} {
		code[built] = true
	}
	var missing, unrouted []string
	for route := range code {
		if !spec[route] && internal[route] == "" {
			missing = append(missing, route)
		}
	}
	for route := range spec {
		if !code[route] {
			unrouted = append(unrouted, route)
		}
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Errorf("the contract is not valid OpenAPI: %v", err)
	}
	sort.Strings(missing)
	sort.Strings(unrouted)
	t.Logf("routes in code %d, operations in spec %d", len(code), len(spec))
	if len(missing) > 0 || len(unrouted) > 0 {
		t.Errorf("not in the contract (%d):\n%s\nin the contract but not found as a literal route (%d):\n%s",
			len(missing), strings.Join(missing, "\n"), len(unrouted), strings.Join(unrouted, "\n"))
	}
}
