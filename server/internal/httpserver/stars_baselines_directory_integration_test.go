package httpserver_test

import (
	"net/http"
	"strings"
	"testing"
)

// PROMPT #026 Workstream A: stars, baselines and the resource directory against current Customer access. Each surface is read raw, so
// names, identifiers and aggregates are checked in the body. Positive controls come first; the revoked member is the last check.
func TestStarsBaselinesAndDirectoryFollowCurrentAccess(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	clientA := f.createClient(admin, "SBD-A")["id"].(string)
	clientB := f.createClient(admin, "SBD-B")["id"].(string)
	member, outsider, bareAdmin := f.person("regular"), f.person("regular"), f.person("admin")
	f.addMember(admin, clientA, admin)
	f.addMember(admin, clientB, admin)
	f.addMember(admin, clientA, member)
	f.addMember(admin, clientB, outsider)
	spaceA := f.scopedSpace("SBD workspace A", "client", clientA)
	spaceB := f.scopedSpace("SBD workspace B", "client", clientB)
	f.share(spaceA, "user", admin.id, "full")
	f.share(spaceA, "user", member.id, "full")
	f.share(spaceB, "user", admin.id, "full")
	f.share(spaceB, "user", outsider.id, "full")
	projA := f.itemIn(spaceA, "client", clientA, member.id, "Alpha secret project")
	projB := f.itemIn(spaceB, "client", clientB, outsider.id, "Bravo secret project")
	f.exec("UPDATE items SET type='project' WHERE id = ANY($1::uuid[])", []string{projA, projB})

	// Stars: a person stars only what they can view; a starred item is listed only while it stays visible.
	f.want(f.call(member, http.MethodPut, "/items/"+projA+"/star", nil), http.StatusNoContent, "member stars own project")
	f.want(f.call(member, http.MethodPut, "/items/"+projB+"/star", nil), http.StatusNotFound, "member cannot star another Customer's project")
	f.want(f.call(outsider, http.MethodPut, "/items/"+projB+"/star", nil), http.StatusNoContent, "outsider stars own project")
	f.want(f.call(bareAdmin, http.MethodPut, "/items/"+projA+"/star", nil), http.StatusNotFound, "admin without access cannot star")
	status, _, body := f.rawGet(member, "/stars")
	if status != http.StatusOK || !strings.Contains(body, "Alpha secret project") || strings.Contains(body, "Bravo secret project") {
		t.Fatalf("member stars must list only the visible project (status %d)", status)
	}
	if status, _, body := f.rawGet(outsider, "/stars"); status != http.StatusOK || strings.Contains(body, "Alpha secret project") || strings.Contains(body, projA) {
		t.Fatalf("outsider stars disclose another Customer's project (status %d)", status)
	}
	if status, _, body := f.rawGet(bareAdmin, "/stars"); status != http.StatusOK || strings.Contains(body, "secret project") {
		t.Fatalf("admin without access sees starred content (status %d)", status)
	}

	// Baselines: list, variance and creation follow the project's current visibility.
	created := f.call(admin, http.MethodPost, "/items/"+projA+"/baselines", map[string]any{"name": "Plan v1"})
	f.want(created, http.StatusCreated, "admin creates a baseline on a visible project")
	baselineID := created.body["id"].(string)
	f.want(f.call(outsider, http.MethodPost, "/items/"+projA+"/baselines", map[string]any{"name": "Leak"}), http.StatusNotFound, "outsider cannot baseline another Customer's project")
	if status, _, body := f.rawGet(member, "/items/"+projA+"/baselines"); status != http.StatusOK || !strings.Contains(body, "Plan v1") {
		t.Fatalf("member lists the baseline (status %d)", status)
	}
	for name, p := range map[string]*person{"outsider": outsider, "admin without access": bareAdmin} {
		status, _, body := f.rawGet(p, "/items/"+projA+"/baselines")
		if status != http.StatusNotFound || strings.Contains(body, "Plan v1") {
			t.Fatalf("%s reads another Customer's baselines (status %d)", name, status)
		}
		if status, _, body := f.rawGet(p, "/baselines/"+baselineID+"/variance"); status != http.StatusNotFound || strings.Contains(body, "Plan v1") {
			t.Fatalf("%s reads another Customer's variance (status %d)", name, status)
		}
	}
	f.want(f.call(member, http.MethodGet, "/baselines/"+baselineID+"/variance", nil), http.StatusOK, "member reads variance")

	// Resource directory: the GET is workspace directory data, not project content. Nothing about either Customer's items may appear.
	for name, p := range map[string]*person{"member": member, "outsider": outsider, "admin without access": bareAdmin} {
		status, _, body := f.rawGet(p, "/resource-directory")
		if status != http.StatusOK {
			t.Fatalf("%s directory status %d", name, status)
		}
		for _, leak := range []string{"secret project", projA, projB, clientA, clientB, "SBD-A", "SBD-B"} {
			if strings.Contains(body, leak) {
				t.Fatalf("%s directory discloses %q", name, leak)
			}
		}
	}

	// Revocation: the member loses the project, so the star, the baseline and its variance stop being returned or accepted.
	f.revokeMember(admin, clientA, member)
	if status, _, body := f.rawGet(member, "/stars"); status != http.StatusOK || strings.Contains(body, "Alpha secret project") {
		t.Fatalf("a revoked member still sees a star (status %d)", status)
	}
	f.want(f.call(member, http.MethodGet, "/items/"+projA+"/baselines", nil), http.StatusNotFound, "revoked member loses baselines")
	f.want(f.call(member, http.MethodGet, "/baselines/"+baselineID+"/variance", nil), http.StatusNotFound, "revoked member loses variance")
	f.want(f.call(member, http.MethodPut, "/items/"+projA+"/star", nil), http.StatusNotFound, "revoked member cannot star")
}
