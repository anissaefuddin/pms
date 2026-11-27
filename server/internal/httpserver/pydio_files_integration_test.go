package httpserver_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
)

// MVP-13: the item panel lists the Pydio folder a text field links to. Access follows the item and the field; the link
// must stay on the configured host and inside the roots; Pydio failures do not leak upstream details.
func TestPydioFilesFollowItemAndFieldAccess(t *testing.T) {
	var calls atomic.Int32
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=svc-token/") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Query().Get("prefix") {
		case "projects/broken/":
			http.Error(w, "<Error>internal detail</Error>", http.StatusInternalServerError)
		case "projects/PTM/":
			fmt.Fprint(w, `<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>projects/PTM/HAZOP.pdf</Key><Size>5</Size><LastModified>2026-11-20T10:00:00Z</LastModified></Contents></ListBucketResult>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer gw.Close()

	off := newFixture(t)
	f := newFixtureWith(t, 1000, func(c *config.Config) {
		c.PydioLinkURL, c.PydioLinkToken, c.PydioLinkRoots = gw.URL, "svc-token", []string{"projects"}
		c.PydioListing = true // listing is an explicit opt-in (Workstream C); this test exercises the enabled path
	})
	admin, member, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("PTM")
	f.share(space, "user", member.id, "read")
	field := f.call(admin, http.MethodPost, "/custom-fields", map[string]any{"name": "Pydio Folder", "type": "text"})
	f.want(field, http.StatusCreated, "Pydio Folder field")
	fid := field.body["id"].(string)
	f.want(f.call(admin, http.MethodPost, "/spaces/"+space+"/custom-fields/"+fid+"/assignments", map[string]any{"scope_kind": "space", "scope_id": space}), http.StatusCreated, "assign")
	project := func(link string) string {
		r := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "project", "title": "P " + link, "locations": []map[string]any{{"space_id": space}}})
		f.want(r, http.StatusCreated, "project")
		id := r.body["id"].(string)
		if link != "" {
			version := f.queryID("SELECT version::text FROM items WHERE id=$1", id)
			f.want(f.call(admin, http.MethodPatch, "/items/"+id, map[string]any{"fields": map[string]any{fid: link}}, "If-Match", fmt.Sprintf("%q", version)), http.StatusOK, "set link")
		}
		return id
	}
	files := func(p *person, item string) reply {
		return f.call(p, http.MethodGet, "/items/"+item+"/pydio-files?field="+fid, nil)
	}
	ptm := project(gw.URL + "/projects/PTM")

	got := files(member, ptm)
	f.want(got, http.StatusOK, "a member who can read the project lists the folder")
	entries := got.body["items"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["name"] != "HAZOP.pdf" || entries[0].(map[string]any)["url"] != gw.URL+"/projects/PTM/HAZOP.pdf" {
		t.Fatalf("listing = %v", got.body)
	}
	before := calls.Load()
	f.want(files(outsider, ptm), http.StatusNotFound, "no access to the item, no listing")
	f.want(files(member, project("")), http.StatusNotFound, "no link")
	f.want(files(member, project(gw.URL+"/personal-files/admin")), http.StatusUnprocessableEntity, "outside the roots")
	f.want(files(member, project("https://elsewhere.example.com/projects/PTM")), http.StatusUnprocessableEntity, "another host")
	if calls.Load() != before {
		t.Fatalf("Pydio was called %d times for refused requests, want 0", calls.Load()-before)
	}
	f.want(files(member, project(gw.URL+"/projects/gone")), http.StatusNotFound, "missing folder")
	broken := files(member, project(gw.URL+"/projects/broken"))
	f.want(broken, http.StatusBadGateway, "Pydio failure")
	if strings.Contains(fmt.Sprint(broken.body), "internal detail") {
		t.Fatalf("upstream detail leaked: %v", broken.body)
	}

	// A restricted field the member has no grant for is hidden, so its folder is not listed either.
	group := f.group("Doc Control", admin)
	f.want(f.call(admin, http.MethodPut, "/custom-fields/"+fid+"/grants", map[string]any{"grants": []map[string]any{{"subject_kind": "group", "subject_id": group, "level": "view"}}}), http.StatusOK, "restrict the field")
	f.want(files(member, ptm), http.StatusNotFound, "hidden field, no listing")
	f.want(files(admin, ptm), http.StatusOK, "the granted admin still lists it")

	// Feature off: the same recipe on an instance without PMS_PYDIO_LINK_URL.
	offAdmin := off.person("admin")
	offSpace := off.space("PTM")
	offField := off.call(offAdmin, http.MethodPost, "/custom-fields", map[string]any{"name": "Pydio Folder", "type": "text"}).body["id"].(string)
	off.want(off.call(offAdmin, http.MethodPost, "/spaces/"+offSpace+"/custom-fields/"+offField+"/assignments", map[string]any{"scope_kind": "space", "scope_id": offSpace}), http.StatusCreated, "assign")
	offItem := off.call(offAdmin, http.MethodPost, "/items", map[string]any{"type": "project", "title": "P", "locations": []map[string]any{{"space_id": offSpace}}}).body["id"].(string)
	off.want(off.call(offAdmin, http.MethodPatch, "/items/"+offItem, map[string]any{"fields": map[string]any{offField: gw.URL + "/projects/PTM"}}, "If-Match", fmt.Sprintf("%q", off.queryID("SELECT version::text FROM items WHERE id=$1", offItem))), http.StatusOK, "set link")
	off.want(off.call(offAdmin, http.MethodGet, "/items/"+offItem+"/pydio-files?field="+offField, nil), http.StatusServiceUnavailable, "feature off")
}
