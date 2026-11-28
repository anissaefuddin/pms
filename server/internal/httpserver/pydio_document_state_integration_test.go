package httpserver_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
)

// PROMPT #033 (Workstream A): the document section shows one of four states. Only a validated link against the Customer's approved
// roots is actionable. A folder value is never returned unless the state is "linked".
func documentFixture(t *testing.T) *fixture {
	t.Helper()
	return newFixtureWith(t, 1000, func(c *config.Config) {
		c.PydioLinkURL, c.PydioLinkToken, c.PydioLinkRoots = pydioBase, "link-only", []string{"projects", "other"}
		c.PydioCustomerRoots = map[string][]string{"PY-A": {"projects/ptm"}}
	})
}

func TestProjectDocumentStatesAreServerDecided(t *testing.T) {
	f := documentFixture(t)
	admin := f.person("admin")
	client := f.createClient(admin, "PY-A")["id"].(string)
	noMapping := f.createClient(admin, "PY-B")["id"].(string)
	member, outsider := f.person("regular"), f.person("regular")
	f.addMember(admin, client, member)
	f.addMember(admin, noMapping, member)
	space := f.call(member, "POST", "/spaces", map[string]any{"name": "Document space", "scope_state": "client", "client_id": client})
	f.want(space, http.StatusCreated, "Customer A space")
	sid := space.body["id"].(string)
	otherSpace := f.call(member, "POST", "/spaces", map[string]any{"name": "Document space B", "scope_state": "client", "client_id": noMapping})
	f.want(otherSpace, http.StatusCreated, "Customer B space")
	internal := f.call(member, "POST", "/spaces", map[string]any{"name": "Document internal", "scope_state": "internal"})
	f.want(internal, http.StatusCreated, "internal space")
	field := f.call(member, http.MethodPost, "/spaces/"+sid+"/custom-fields", map[string]any{"name": "Documents", "type": "text", "config": map[string]any{"pydio_folder": true}})
	f.want(field, http.StatusCreated, "canonical Pydio folder field on the Customer A space")
	fidA := field.body["id"].(string)
	fieldB := f.call(member, http.MethodPost, "/spaces/"+otherSpace.body["id"].(string)+"/custom-fields", map[string]any{"name": "Docs B", "type": "text", "config": map[string]any{"pydio_folder": true}})
	f.want(fieldB, http.StatusCreated, "canonical field on the Customer B space")
	fidB := fieldB.body["id"].(string)
	// Internal work has no Customer; its document field is workspace-wide.
	fieldWS := f.call(admin, http.MethodPost, "/custom-fields", map[string]any{"name": "Docs internal", "type": "text", "config": map[string]any{"pydio_folder": true}})
	f.want(fieldWS, http.StatusCreated, "workspace-wide Pydio field")
	fidWS := fieldWS.body["id"].(string)
	f.want(f.call(member, http.MethodPost, "/spaces/"+internal.body["id"].(string)+"/custom-fields/"+fidWS+"/assignments", map[string]any{"scope_kind": "space", "scope_id": internal.body["id"].(string)}), http.StatusCreated, "assign the workspace field to the internal space")

	project := func(space string) string {
		r := f.call(member, "POST", "/items", map[string]any{"type": "project", "title": "Docs", "locations": []map[string]any{{"space_id": space}}})
		f.want(r, http.StatusCreated, "project")
		return r.body["id"].(string)
	}
	setValue := func(item, fid, value string) {
		version := f.queryID("SELECT version::text FROM items WHERE id=$1", item)
		f.want(f.call(member, "PATCH", "/items/"+item, map[string]any{"fields": map[string]any{fid: value}}, "If-Match", fmt.Sprintf("%q", version)), http.StatusOK, "store the folder value")
	}
	state := func(p *person, item string) (string, string, string) {
		t.Helper()
		r := f.call(p, "GET", "/items/"+item+"/pydio-reference", nil)
		f.want(r, http.StatusOK, "document state")
		st, _ := r.body["state"].(string)
		open, _ := r.body["open_url"].(string)
		raw := fmt.Sprint(r.body)
		return st, open, raw
	}

	// 1. Not configured: the field exists but holds no value.
	empty := project(sid)
	if st, _, _ := state(member, empty); st != "not_configured" {
		t.Fatalf("an empty field must be not_configured, got %q", st)
	}

	// 2. Linked: a validated link under the Customer's approved root is actionable.
	good := "https://dms.example.com/projects/ptm/site-survey"
	linked := project(sid)
	setValue(linked, fidA, good)
	st, open, _ := state(member, linked)
	if st != "linked" || open != good {
		t.Fatalf("a link under the approved root must be linked with its address: %s %q", st, open)
	}

	// 3. Invalid: outside the approved root, a query string, or a foreign host. The value is never returned.
	// The write path checks the global origin and roots (B4-02): a query string or a foreign host is refused there. A link under
	// another global root is accepted on write, and only the Customer's own roots decide the state, so it reads as invalid.
	cases := []struct {
		value     string
		writeCode int
	}{
		{"https://dms.example.com/other/site-survey", http.StatusOK},
		{"https://dms.example.com/projects/ptm/x?token=secret", http.StatusUnprocessableEntity},
		{"https://evil.example.net/projects/ptm/x", http.StatusUnprocessableEntity},
	}
	for _, c := range cases {
		bad := c.value
		p := project(sid)
		version := f.queryID("SELECT version::text FROM items WHERE id=$1", p)
		f.want(f.call(member, "PATCH", "/items/"+p, map[string]any{"fields": map[string]any{fidA: bad}}, "If-Match", fmt.Sprintf("%q", version)), c.writeCode, "the write check for "+bad)
		// A value stored before a check is legacy data: put it in directly, so the state logic is tested on it.
		f.exec("UPDATE items SET fields = jsonb_set(fields, ARRAY[$2], to_jsonb($3::text)) WHERE id=$1", p, fidA, bad)
		st, open, raw := state(member, p)
		if st != "invalid" || open != "" || strings.Contains(raw, "secret") || strings.Contains(raw, "site-survey") {
			t.Fatalf("an unsafe link must be invalid and never returned: %s %q %s", st, open, raw)
		}
	}

	// 4. Roots unavailable: a Customer without an approved mapping gets no link, even for a valid-looking value.
	noRoots := project(otherSpace.body["id"].(string))
	setValue(noRoots, fidB, good)
	st, open, raw := state(member, noRoots)
	if st != "roots_unavailable" || open != "" || strings.Contains(raw, "site-survey") {
		t.Fatalf("no approved Customer roots must mean roots_unavailable without the folder: %s %q", st, open)
	}

	// 5. Internal work has no Customer, so it cannot be linked on the global roots alone.
	internalItem := project(internal.body["id"].(string))
	setValue(internalItem, fidWS, good)
	if st, open, _ := state(member, internalItem); st != "roots_unavailable" || open != "" {
		t.Fatalf("internal work must not link on the global roots: %s %q", st, open)
	}

	// 6. Without item access the reference is not found, and the value is not revealed.
	f.want(f.call(outsider, "GET", "/items/"+linked+"/pydio-reference", nil), http.StatusNotFound, "no access, no reference")
}
