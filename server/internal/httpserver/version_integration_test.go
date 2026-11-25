package httpserver_test

import (
	"strings"
	"testing"
)

// DEF-019: the running build is identifiable, and image identity is only what the deploy scripts supplied.
func TestVersionReportsBuildAndOnlySuppliedImageIdentity(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")

	f.want(f.call(nil, "GET", "/version", nil), 401, "anonymous")

	r := f.call(member, "GET", "/version", nil)
	f.want(r, 200, "member")
	for _, key := range []string{"version", "commit", "built_at"} {
		if s, _ := r.body[key].(string); s == "" {
			t.Fatalf("member: %s missing in %v", key, r.body)
		}
	}
	if _, ok := r.body["image"]; ok {
		t.Fatalf("member sees image identity: %v", r.body)
	}

	image := func() map[string]any {
		r := f.call(admin, "GET", "/version", nil)
		f.want(r, 200, "admin")
		m, ok := r.body["image"].(map[string]any)
		if !ok {
			t.Fatalf("admin: no image object in %v", r.body)
		}
		return m
	}
	expect := func(what, ref, id, dig string) {
		t.Helper()
		if m := image(); m["ref"] != ref || m["id"] != id || m["registry_digest"] != dig {
			t.Fatalf("%s: got %v", what, m)
		}
	}
	t.Setenv("PMS_IMAGE_REF", "")
	t.Setenv("PMS_IMAGE_ID", "")
	t.Setenv("PMS_IMAGE_DIGEST", "")
	expect("nothing supplied", "unavailable", "unavailable", "unavailable")
	t.Setenv("PMS_IMAGE_REF", "ghcr.io/acme/pms:v1")
	t.Setenv("PMS_IMAGE_ID", "ghcr.io/acme/pms:v1")
	t.Setenv("PMS_IMAGE_DIGEST", "v1")
	expect("a tag is not a digest", "ghcr.io/acme/pms:v1", "unavailable", "unavailable")
	t.Setenv("PMS_IMAGE_ID", "sha256:"+strings.Repeat("b", 64))
	t.Setenv("PMS_IMAGE_DIGEST", digest)
	expect("supplied", "ghcr.io/acme/pms:v1", "sha256:"+strings.Repeat("b", 64), digest)
}
