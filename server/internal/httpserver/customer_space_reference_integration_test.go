package httpserver_test

import (
	"net/http"
	"testing"
)

// B5-01: a Customer Space carries its Customer, derived from the Space's own ownership. An Internal Space has none.
// The reference appears in the Space detail, the Space list and the overview, and only for people who can read the Space.
func TestCustomerSpaceCarriesItsCustomerOnEveryReadSurface(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	member := f.person("regular")
	client := f.createClient(admin, "CUST-REF")["id"].(string)
	f.addMember(admin, client, member)
	created := f.call(member, "POST", "/spaces", map[string]any{"name": "Customer ref space", "scope_state": "client", "client_id": client})
	f.want(created, http.StatusCreated, "a member creates a Customer Space")
	sid := created.body["id"].(string)
	internal := f.call(member, "POST", "/spaces", map[string]any{"name": "Internal ref space", "scope_state": "internal"})
	f.want(internal, http.StatusCreated, "an internal Space")
	iid := internal.body["id"].(string)

	detail := f.call(member, "GET", "/spaces/"+sid, nil)
	f.want(detail, http.StatusOK, "read the Customer Space")
	cust, ok := detail.body["customer"].(map[string]any)
	if !ok || cust["id"] != client || cust["code"] != "CUST-REF" || cust["archived"] != false {
		t.Fatalf("the Customer Space must carry its Customer: %v", detail.body["customer"])
	}
	if _, has := f.call(member, "GET", "/spaces/"+iid, nil).body["customer"]; has {
		t.Fatal("an internal Space must not carry a Customer")
	}

	list := f.call(member, "GET", "/spaces", nil)
	f.want(list, http.StatusOK, "list Spaces")
	var inList bool
	for _, row := range list.items() {
		if row["id"] == sid {
			inList = true
			if c, ok := row["customer"].(map[string]any); !ok || c["code"] != "CUST-REF" {
				t.Fatalf("the Space list lost the Customer: %v", row["customer"])
			}
		}
	}
	if !inList {
		t.Fatal("the member's Customer Space is missing from the list")
	}

	overview := f.call(member, "GET", "/overview?limit=100", nil)
	f.want(overview, http.StatusOK, "the overview")
	var inOverview bool
	for _, raw := range overview.body["spaces"].([]any) {
		row := raw.(map[string]any)
		if row["id"] == sid {
			inOverview = true
			if c, ok := row["customer"].(map[string]any); !ok || c["code"] != "CUST-REF" {
				t.Fatalf("the overview lost the Customer: %v", row["customer"])
			}
		}
	}
	if !inOverview {
		t.Fatal("the Customer Space is missing from the overview")
	}

	// An archived Customer is still named on its Spaces, and marked so the UI can say so.
	f.exec("UPDATE clients SET status='archived', archived_at=now() WHERE id=$1", client)
	archived := f.call(member, "GET", "/spaces/"+sid, nil)
	f.want(archived, http.StatusOK, "read after the Customer is archived")
	if c, ok := archived.body["customer"].(map[string]any); !ok || c["archived"] != true {
		t.Fatalf("an archived Customer must be marked: %v", archived.body["customer"])
	}

	// Without a grant on the Space, its Customer is not disclosed.
	outsider := f.person("regular")
	f.want(f.call(outsider, "GET", "/spaces/"+sid, nil), http.StatusNotFound, "no grant, no Space, no Customer")
}
