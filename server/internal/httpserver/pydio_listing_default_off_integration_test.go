package httpserver_test

import (
	"net/http"
	"testing"
)

// Workstream C: Pydio folder listing is off until PMS_PYDIO_LISTING=true. The check runs before the item or field is read, so a
// disabled instance answers the same way for every item and field. The shared fixture enables it; this test uses the default.
func TestPydioListingIsOffByDefault(t *testing.T) {
	f := newFixtureWith(t, 1000, nil)
	admin := f.person("admin")
	space := f.space("Pydio off")
	f.share(space, "user", admin.id, "full")
	item := f.queryID("INSERT INTO items(type,title,created_by,scope_state) VALUES ('task','Linked',$1,'internal') RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", item, space)
	for _, field := range []string{"00000000-0000-4000-8000-000000000001", "not-a-uuid"} {
		res := f.call(admin, http.MethodGet, "/items/"+item+"/pydio-files?field="+field, nil)
		if res.status != http.StatusServiceUnavailable || res.body["code"] != "pydio_listing_disabled" {
			t.Fatalf("pydio listing must be disabled by default (field %q): status %d body %v", field, res.status, res.body)
		}
	}
}
