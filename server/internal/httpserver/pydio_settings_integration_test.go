package httpserver_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
)

const savedOrigin = "https://files.example.com"

func pydioSettings(version int, change map[string]any) map[string]any {
	body := map[string]any{"version": version, "enabled": true, "base_url": savedOrigin, "roots": []string{"clients"},
		"customer_roots": map[string]any{"PY-A": []string{"clients/a"}}}
	for k, v := range change {
		body[k] = v
	}
	return body
}

func TestPydioSettingsAreAdministratorOnlyValidatedAndAudited(t *testing.T) {
	f := pydioFixture(t, false)
	admin, member := f.person("admin"), f.person("regular")
	f.createClient(admin, "PY-A")
	for method, body := range map[string]any{http.MethodGet: nil, http.MethodPut: pydioSettings(0, nil), http.MethodDelete: nil} {
		f.want(f.call(member, method, "/pydio/settings", body), http.StatusForbidden, "a regular user: "+method)
	}

	// Without a saved row the environment is shown as it is.
	got := f.call(admin, http.MethodGet, "/pydio/settings", nil)
	f.want(got, http.StatusOK, "read")
	if got.body["source"] != "environment" || got.body["base_url"] != pydioBase || got.body["version"] != float64(0) || got.body["enabled"] != true {
		t.Fatalf("environment view = %v", got.body)
	}
	if strings.Contains(fmt.Sprint(got.body), "link-only") {
		t.Fatalf("the token must never be returned: %v", got.body)
	}
	f.want(f.call(admin, http.MethodDelete, "/pydio/settings", nil), http.StatusNotFound, "nothing saved to remove")

	for name, change := range map[string]map[string]any{
		"http":                      {"base_url": "http://files.example.com"},
		"credentials":               {"base_url": "https://u:p@files.example.com"},
		"path":                      {"base_url": savedOrigin + "/ws"},
		"query":                     {"base_url": savedOrigin + "?a=1"},
		"no roots":                  {"roots": []string{}},
		"dot segment":               {"roots": []string{"clients/.."}},
		"empty segment":             {"roots": []string{"clients//a"}},
		"customer outside roots":    {"customer_roots": map[string]any{"PY-A": []string{"clients-old/a"}}},
		"customer without roots":    {"customer_roots": map[string]any{"PY-A": []string{}}},
		"unknown customer":          {"customer_roots": map[string]any{"NOPE": []string{"clients/a"}}},
		"customer code wrong case":  {"customer_roots": map[string]any{"py-a": []string{"clients/a"}}},
		"customer root dot segment": {"customer_roots": map[string]any{"PY-A": []string{"clients/./a"}}},
	} {
		if r := f.call(admin, http.MethodPut, "/pydio/settings", pydioSettings(0, change)); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("%s: status %d, want 422 (%v)", name, r.status, r.body)
		}
	}

	saved := f.call(admin, http.MethodPut, "/pydio/settings", pydioSettings(0, map[string]any{"base_url": savedOrigin + "/", "roots": []string{" /clients/ ", "clients"}}))
	f.want(saved, http.StatusOK, "create")
	if saved.body["source"] != "database" || saved.body["version"] != float64(1) || saved.body["base_url"] != savedOrigin || fmt.Sprint(saved.body["roots"]) != "[clients]" {
		t.Fatalf("saved view = %v", saved.body)
	}
	f.want(f.call(admin, http.MethodPut, "/pydio/settings", pydioSettings(0, nil)), http.StatusConflict, "a second create")
	f.want(f.call(admin, http.MethodPut, "/pydio/settings", pydioSettings(7, nil)), http.StatusConflict, "a stale version")
	updated := f.call(admin, http.MethodPut, "/pydio/settings", pydioSettings(1, map[string]any{"customer_roots": map[string]any{}}))
	f.want(updated, http.StatusOK, "update")
	if updated.body["version"] != float64(2) {
		t.Fatalf("updated view = %v", updated.body)
	}
	f.want(f.call(admin, http.MethodDelete, "/pydio/settings", nil), http.StatusNoContent, "remove")
	if back := f.call(admin, http.MethodGet, "/pydio/settings", nil); back.body["source"] != "environment" {
		t.Fatalf("after removal = %v", back.body)
	}
	if got := f.queryID("SELECT string_agg(action, ',' ORDER BY id) FROM audit_events WHERE action LIKE 'pydio.%'"); got != "pydio.configured,pydio.configured,pydio.removed" {
		t.Fatalf("audit = %s", got)
	}
}

// A saved configuration replaces the environment as a whole, applies without a restart, and removing it brings the environment back.
func TestSavedPydioSettingsApplyWithoutARestart(t *testing.T) {
	f := documentFixture(t)
	admin := f.person("admin")
	client := f.createClient(admin, "PY-A")["id"].(string)
	f.addMember(admin, client, admin)
	space := f.scopedSpace("PY settings", "client", client)
	f.share(space, "user", admin.id, "full")
	field := f.call(admin, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{"name": "Documents", "type": "text", "config": map[string]any{"pydio_folder": true}})
	f.want(field, http.StatusCreated, "field")
	fid := field.body["id"].(string)
	project := func() string {
		r := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "project", "title": "Docs", "locations": []map[string]any{{"space_id": space}}})
		f.want(r, http.StatusCreated, "project")
		return r.body["id"].(string)
	}
	set := func(item, link string) reply {
		version := f.queryID("SELECT version::text FROM items WHERE id=$1", item)
		return f.call(admin, http.MethodPatch, "/items/"+item, map[string]any{"fields": map[string]any{fid: link}}, "If-Match", fmt.Sprintf("%q", version))
	}
	state := func(item string) any {
		r := f.call(admin, http.MethodGet, "/items/"+item+"/pydio-reference", nil)
		f.want(r, http.StatusOK, "state")
		return r.body["state"]
	}
	old, saved := pydioBase+"/projects/ptm/x", savedOrigin+"/clients/a/x"
	item := project()
	f.want(set(item, old), http.StatusOK, "environment origin")
	f.want(set(project(), saved), http.StatusUnprocessableEntity, "not configured yet")

	f.want(f.call(admin, http.MethodPut, "/pydio/settings", pydioSettings(0, nil)), http.StatusOK, "save")
	f.want(set(project(), old), http.StatusUnprocessableEntity, "the environment origin no longer applies")
	if got := state(item); got != "invalid" {
		t.Fatalf("a link stored under the old configuration must be invalid, got %v", got)
	}
	fresh := project()
	f.want(set(fresh, saved), http.StatusOK, "saved origin")
	if got := state(fresh); got != "linked" {
		t.Fatalf("saved configuration must link, got %v", got)
	}

	f.want(f.call(admin, http.MethodPut, "/pydio/settings", pydioSettings(1, map[string]any{"enabled": false})), http.StatusOK, "turn off")
	f.want(set(project(), saved), http.StatusUnprocessableEntity, "off means no reference is accepted")
	f.want(set(project(), old), http.StatusUnprocessableEntity, "off does not fall back to the environment")
	if got := state(fresh); got != "not_configured" {
		t.Fatalf("off must be not_configured, got %v", got)
	}

	f.want(f.call(admin, http.MethodDelete, "/pydio/settings", nil), http.StatusNoContent, "remove")
	if got := state(item); got != "linked" {
		t.Fatalf("the environment must apply again, got %v", got)
	}
}

// The listing token belongs to the environment's address. A saved configuration that points elsewhere must never receive it.
func TestPydioListingTokenStaysWithItsOwnAddress(t *testing.T) {
	f := newFixtureWith(t, 1000, func(c *config.Config) {
		c.PydioLinkURL, c.PydioLinkToken, c.PydioLinkRoots, c.PydioListing = pydioBase, "svc-token", []string{"clients"}, true
	})
	admin := f.person("admin")
	f.createClient(admin, "PY-A")
	available := func(r reply) any { return r.body["listing_token_available"] }
	if r := f.call(admin, http.MethodGet, "/pydio/settings", nil); available(r) != true {
		t.Fatalf("environment: %v", r.body)
	}
	elsewhere := f.call(admin, http.MethodPut, "/pydio/settings", pydioSettings(0, nil))
	f.want(elsewhere, http.StatusOK, "another address")
	if available(elsewhere) != false {
		t.Fatalf("another address must not get the token: %v", elsewhere.body)
	}
	same := f.call(admin, http.MethodPut, "/pydio/settings", pydioSettings(1, map[string]any{"base_url": pydioBase}))
	f.want(same, http.StatusOK, "the environment address")
	if available(same) != true {
		t.Fatalf("the same address keeps the token: %v", same.body)
	}
}

// Upload limits default to 100 MB and every type, survive a client that does not send them, and are audited as stored.
func TestPydioUploadLimits(t *testing.T) {
	f := pydioFixture(t, false)
	admin := f.person("admin")
	f.createClient(admin, "PY-A")
	limits := func(r reply) string {
		return fmt.Sprint(r.body["max_upload_mb"], " ", r.body["allowed_extensions"])
	}
	put := func(version int, change map[string]any, want string) {
		t.Helper()
		r := f.call(admin, http.MethodPut, "/pydio/settings", pydioSettings(version, change))
		f.want(r, http.StatusOK, "save")
		if got := limits(r); got != want {
			t.Fatalf("version %d: limits = %s, want %s", version, got, want)
		}
	}
	if got := limits(f.call(admin, http.MethodGet, "/pydio/settings", nil)); got != "100 []" {
		t.Fatalf("environment limits = %s", got)
	}
	many := make([]string, 101)
	for i := range many {
		many[i] = fmt.Sprintf("e%d", i)
	}
	for name, change := range map[string]map[string]any{
		"zero":      {"max_upload_mb": 0},
		"too large": {"max_upload_mb": 1025},
		"dotted":    {"allowed_extensions": []string{"tar.gz"}},
		"wildcard":  {"allowed_extensions": []string{"*"}},
		"blank":     {"allowed_extensions": []string{" "}},
		"too many":  {"allowed_extensions": many},
	} {
		if r := f.call(admin, http.MethodPut, "/pydio/settings", pydioSettings(0, change)); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("%s: status %d, want 422 (%v)", name, r.status, r.body)
		}
	}
	put(0, nil, "100 []")
	put(1, map[string]any{"max_upload_mb": 1024, "allowed_extensions": []string{".PDF", "pdf", " Docx "}}, "1024 [pdf docx]")
	put(2, nil, "1024 [pdf docx]")
	put(3, map[string]any{"max_upload_mb": nil, "allowed_extensions": nil}, "1024 [pdf docx]")
	put(4, map[string]any{"max_upload_mb": 1}, "1 [pdf docx]")
	put(5, map[string]any{"allowed_extensions": []string{}}, "1 []")
	if got := f.queryID(`SELECT details->>'max_upload_mb' || ' ' || (details->>'allowed_extensions') FROM audit_events
WHERE action='pydio.configured' ORDER BY id DESC LIMIT 1`); got != "1 []" {
		t.Fatalf("audit = %s", got)
	}
	f.want(f.call(admin, http.MethodDelete, "/pydio/settings", nil), http.StatusNoContent, "remove")
	put(0, map[string]any{"max_upload_mb": 250, "allowed_extensions": []string{"zip"}}, "250 [zip]")
}
