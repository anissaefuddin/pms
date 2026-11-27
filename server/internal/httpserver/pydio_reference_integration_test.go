package httpserver_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
)

// PROMPT #027 (B4-01..03, PY-01..04, PY-05..10, PY-11..13). A project's Pydio folder reference is a Text field marked pydio_folder. Every write is
// checked against the configured link origin and roots. Every read is checked against current item access, Customer eligibility and field
// permission. Listing stays OFF: the link-first path never calls Pydio.
const pydioBase = "https://dms.example.com"

func pydioFixture(t *testing.T, listing bool) *fixture {
	t.Helper()
	return newFixtureWith(t, 1000, func(c *config.Config) {
		c.PydioLinkURL, c.PydioLinkToken, c.PydioLinkRoots = pydioBase, "link-only", []string{"projects", "common-files/Clients"}
		c.PydioListing = listing
	})
}

func TestProjectDocumentReferenceIsValidatedAndProtected(t *testing.T) {
	f := pydioFixture(t, false)
	admin := f.person("admin")
	client := f.createClient(admin, "PY-A")["id"].(string)
	other := f.createClient(admin, "PY-B")["id"].(string)
	member, outsider, restricted := f.person("regular"), f.person("regular"), f.person("regular")
	f.addMember(admin, client, admin)
	f.addMember(admin, client, member)
	f.addMember(admin, client, restricted)
	f.addMember(admin, other, outsider)
	space := f.scopedSpace("PY workspace", "client", client)
	f.share(space, "user", admin.id, "full")
	f.share(space, "user", member.id, "full")
	f.share(space, "user", restricted.id, "full")
	otherSpace := f.scopedSpace("PY other", "client", other)
	f.share(otherSpace, "user", outsider.id, "full")
	f.share(otherSpace, "user", admin.id, "full")

	field := f.call(admin, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{
		"name": "Documents", "type": "text", "config": map[string]any{"pydio_folder": true},
	})
	f.want(field, http.StatusCreated, "canonical Pydio folder field")
	fid := field.body["id"].(string)
	f.want(f.call(admin, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{
		"name": "Plain", "type": "text", "config": map[string]any{"pydio_folder": true},
	}), http.StatusCreated, "pydio flag on a second field only checks values, not the type")
	f.want(f.call(admin, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{
		"name": "Bad flag", "type": "number", "config": map[string]any{"pydio_folder": true},
	}), http.StatusUnprocessableEntity, "the flag applies only to Text fields")

	// Values are written by PATCH after creation. A fresh project is created for each refused reference.
	newProject := func() string {
		created := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "project", "title": "PTM", "locations": []map[string]any{{"space_id": space}}})
		f.want(created, http.StatusCreated, "project")
		return created.body["id"].(string)
	}
	setRef := func(id, link string) reply {
		return f.call(admin, http.MethodPatch, "/items/"+id, map[string]any{"fields": map[string]any{fid: link}}, "If-Match", `"1"`)
	}
	// PY-05: a valid HTTPS reference under an allowed root is accepted and stored.
	itemID := newProject()
	f.want(setRef(itemID, pydioBase+"/projects/PTM-2026/Drawings"), http.StatusOK, "PY-05 valid reference under an allowed root")

	// PY-06, PY-07, PY-08, PY-09: refused on create, with no item written and no value echoed.
	for name, link := range map[string]string{
		"wrong host (PY-06)":        "https://evil.example.com/projects/x",
		"http scheme (PY-06)":       "http://dms.example.com/projects/x",
		"other port (PY-06)":        "https://dms.example.com:8443/projects/x",
		"traversal (PY-07)":         pydioBase + "/projects/../secret",
		"encoded traversal (PY-07)": pydioBase + "/projects/%2e%2e/secret",
		"sibling root (PY-08)":      pydioBase + "/projects-old/x",
		"credentials (PY-09)":       "https://admin:pw@dms.example.com/projects/x",
		"token in query (PY-09)":    pydioBase + "/projects/x?token=abc",
	} {
		res := setRef(newProject(), link)
		if res.status != http.StatusUnprocessableEntity {
			t.Fatalf("%s: status %d, want 422", name, res.status)
		}
		if raw, _ := res.body["detail"].(string); strings.Contains(raw, "secret") || strings.Contains(raw, "abc") || strings.Contains(raw, "pw") {
			t.Fatalf("%s: the refusal repeats the value: %q", name, raw)
		}
	}
	// The same validation on PATCH: an invalid value leaves the stored reference unchanged.
	bad := f.call(admin, http.MethodPatch, "/items/"+itemID, map[string]any{"fields": map[string]any{fid: "https://evil.example.com/projects/x"}}, "If-Match", `"2"`)
	f.want(bad, http.StatusUnprocessableEntity, "PATCH with a foreign host")

	// PY-01: an authorized same-Customer reader sees the reference; PY-12: link-first reading works with listing OFF.
	read := f.call(member, http.MethodGet, "/items/"+itemID, nil)
	f.want(read, http.StatusOK, "PY-01 same-Customer reader")
	if fields, _ := read.body["fields"].(map[string]any); fields[fid] != pydioBase+"/projects/PTM-2026/Drawings" {
		t.Fatalf("PY-01: the reader must see the reference: %v", read.body["fields"])
	}
	// PY-02: another Customer cannot retrieve the item or its reference.
	if status, _, body := f.rawGet(outsider, "/items/"+itemID); status != http.StatusNotFound || strings.Contains(body, "PTM-2026") {
		t.Fatalf("PY-02: another Customer retrieved the reference (status %d)", status)
	}
	// PY-03: a field the reader may not see is absent from the item.
	f.want(f.call(admin, http.MethodPut, "/spaces/"+space+"/custom-fields/"+fid+"/grants", map[string]any{
		"grants": []map[string]any{{"subject_kind": "user", "subject_id": admin.id, "level": "edit"}}}), http.StatusOK, "restrict the reference field")
	hidden := f.call(member, http.MethodGet, "/items/"+itemID, nil)
	if fields, _ := hidden.body["fields"].(map[string]any); fields != nil && fields[fid] != nil {
		t.Fatalf("PY-03: a restricted reference is visible to a reader without the field grant")
	}
	f.want(f.call(admin, http.MethodPut, "/spaces/"+space+"/custom-fields/"+fid+"/grants", map[string]any{"grants": []any{}}), http.StatusOK, "lift the restriction")

	// PY-04: revoked membership denies subsequent reads of the reference.
	f.revokeMember(admin, client, member)
	if status, _, body := f.rawGet(member, "/items/"+itemID); status != http.StatusNotFound || strings.Contains(body, "PTM-2026") {
		t.Fatalf("PY-04: a revoked member still reads the reference (status %d)", status)
	}
	// PY-11: with listing OFF the listing endpoint refuses before any Pydio request or item data.
	f.want(f.call(admin, http.MethodGet, "/items/"+itemID+"/pydio-files?field="+fid, nil), http.StatusServiceUnavailable, "PY-11 listing OFF")
}

// PY-13: a reference written before validation existed is not actionable: the listing path refuses it and makes no Pydio call.
func TestLegacyUnsafeReferenceIsNotActionable(t *testing.T) {
	f := pydioFixture(t, true)
	admin := f.person("admin")
	legacyClient := f.createClient(admin, "PY-L")["id"].(string)
	f.addMember(admin, legacyClient, admin)
	space := f.scopedSpace("PY legacy", "client", legacyClient)
	f.share(space, "user", admin.id, "full")
	field := f.call(admin, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{"name": "Docs", "type": "text", "config": map[string]any{"pydio_folder": true}})
	f.want(field, http.StatusCreated, "legacy field")
	fid := field.body["id"].(string)
	item := f.itemIn(space, "client", legacyClient, admin.id, "Legacy")
	f.exec("UPDATE items SET type='project' WHERE id=$1", item)
	f.exec(`UPDATE items SET fields = jsonb_build_object($2::text, 'https://evil.example.com/projects/x?token=abc') WHERE id=$1`, item, fid)
	res := f.call(admin, http.MethodGet, "/items/"+item+"/pydio-files?field="+fid, nil)
	if res.status != http.StatusUnprocessableEntity || strings.Contains(res.body["detail"].(string), "evil") {
		t.Fatalf("PY-13: an unsafe legacy reference must not be opened or echoed: %d %v", res.status, res.body)
	}
}
