package httpserver_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// MVP-02 (docs/roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md §3.1): the R1 customer reference is a Workspace single_select field
// "Customer" assigned to each customer Space, filled on projects, and reported with analytics group_by=field. This pins the
// recipe the operators follow, so a change that breaks it fails here before it reaches the instance.
func TestCustomerReferenceRecipe(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	ptm, phe, internal := f.space("PTM"), f.space("PHE"), f.space("Internal")
	f.share(ptm, "user", member.id, "read") // the member sees one customer only

	optPTM, optPHE := uuid.NewString(), uuid.NewString()
	field := f.call(admin, http.MethodPost, "/custom-fields", map[string]any{"name": "Customer", "type": "single_select", "options": []map[string]any{
		{"id": optPTM, "label": "PTM — Pertamina", "color": "#2563EB"}, {"id": optPHE, "label": "PHE — Pertamina Hulu Energi", "color": "#16A34A"}}})
	f.want(field, http.StatusCreated, "Workspace Customer field")
	fid := field.body["id"].(string)
	for _, s := range []string{ptm, phe} {
		f.want(f.call(admin, http.MethodPost, "/spaces/"+s+"/custom-fields/"+fid+"/assignments", map[string]any{"scope_kind": "space", "scope_id": s}), http.StatusCreated, "assign Customer to the customer Space")
	}

	project := func(space, title string) string {
		r := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "project", "title": title, "locations": []map[string]any{{"space_id": space}}})
		f.want(r, http.StatusCreated, "create project "+title)
		return r.body["id"].(string)
	}
	setCustomer := func(item, option string) reply {
		version := f.queryID("SELECT version::text FROM items WHERE id=$1", item)
		return f.call(admin, http.MethodPatch, "/items/"+item, map[string]any{"fields": map[string]any{fid: option}}, "If-Match", fmt.Sprintf("%q", version))
	}
	f.want(setCustomer(project(ptm, "PTM study"), optPTM), http.StatusOK, "customer on the PTM project")
	f.want(setCustomer(project(phe, "PHE study"), optPHE), http.StatusOK, "customer on the PHE project")
	// A Space that is not assigned (internal work) cannot carry a customer value.
	f.want(setCustomer(project(internal, "Internal tooling"), optPTM), http.StatusUnprocessableEntity, "no Customer outside assigned Spaces")

	byCustomer := map[string]any{"reporting_on": "projects", "metric": "count", "group_by": "field", "field_id": fid,
		"space_ids": []string{ptm, phe, internal}, "filters": map[string]any{}}
	if got := rowsOf(f.call(admin, http.MethodPost, "/analytics/query", byCustomer)); got[optPTM] != 1 || got[optPHE] != 1 {
		t.Fatalf("admin projects by customer = %v, want one PTM and one PHE", got)
	}
	// D-17.1: grouping must not reveal another customer's projects, not even as a count.
	if got := rowsOf(f.call(member, http.MethodPost, "/analytics/query", byCustomer)); got[optPTM] != 1 || len(got) != 1 {
		t.Fatalf("member projects by customer = %v, want only PTM", got)
	}
}
