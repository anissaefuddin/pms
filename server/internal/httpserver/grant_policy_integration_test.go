package httpserver_test

import (
	"net/http"
	"testing"
)

// GRANT-01: the creator of a Customer Space keeps the explicit Full grant, which needs active Customer membership. Revoking that
// membership denies protected content access at once. Nothing grants access to other Spaces (D-17.1 explicit access only).
func TestCreatorGrantNeedsMembershipAndEndsWithIt(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	creator := f.person("regular")
	client := f.createClient(admin, "GRANT-A")["id"].(string)
	f.addMember(admin, client, creator)
	space := f.call(creator, "POST", "/spaces", map[string]any{"name": "Creator space", "scope_state": "client", "client_id": client})
	f.want(space, http.StatusCreated, "a member creates a Customer Space")
	sid := space.body["id"].(string)
	if n := f.queryID("SELECT count(*)::text FROM space_admins WHERE space_id=$1 AND user_id=$2", sid, creator.id); n != "1" {
		t.Fatalf("the creator must hold the explicit grant: %s rows", n)
	}
	task := f.call(creator, "POST", "/items", map[string]any{"type": "task", "title": "Creator task", "locations": []map[string]string{{"space_id": sid}}})
	f.want(task, http.StatusCreated, "the creator works in the Space")
	f.want(f.call(creator, "GET", "/items/"+task.body["id"].(string), nil), http.StatusOK, "the creator reads its Task")
	f.revokeMember(admin, client, creator)
	f.want(f.call(creator, "GET", "/spaces/"+sid, nil), http.StatusNotFound, "revoked membership denies the Space")
	f.want(f.call(creator, "GET", "/items/"+task.body["id"].(string), nil), http.StatusNotFound, "revoked membership denies the Task")
	f.addMember(admin, client, creator)
	f.want(f.call(creator, "GET", "/items/"+task.body["id"].(string), nil), http.StatusOK, "regranted membership restores the explicit grant")
}

// A Workspace Admin needs Customer membership to create a Customer Space, and gets no access to Spaces other people created.
func TestWorkspaceAdminWithoutMembershipCannotCreateOrReadCustomerWork(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	client := f.createClient(admin, "GRANT-B")["id"].(string)
	outsider := f.unlinkedAdmin()
	refused := f.call(outsider, "POST", "/spaces", map[string]any{"name": "Admin attempt", "scope_state": "client", "client_id": client})
	if refused.status == http.StatusCreated {
		t.Fatal("a Workspace Admin without membership created a Customer Space")
	}
	if n := f.queryID("SELECT count(*)::text FROM spaces WHERE name='Admin attempt'"); n != "0" {
		t.Fatalf("the refused Space was written: %s rows", n)
	}
	owner := f.person("regular")
	internal := f.call(owner, "POST", "/spaces", map[string]any{"name": "Owner internal space", "scope_state": "internal"})
	f.want(internal, http.StatusCreated, "an internal Space")
	sid := internal.body["id"].(string)
	item := f.call(owner, "POST", "/items", map[string]any{"type": "task", "title": "Owner task", "locations": []map[string]string{{"space_id": sid}}})
	f.want(item, http.StatusCreated, "a task in the Space")
	f.want(f.call(outsider, "GET", "/spaces/"+sid, nil), http.StatusNotFound, "an admin's role gives no access to another Space")
	f.want(f.call(outsider, "GET", "/items/"+item.body["id"].(string), nil), http.StatusNotFound, "nor to its Tasks")
}

// Read Only and Limited are resource grants on a Regular user. Read Only views and cannot edit; Limited views and cannot edit the title.
func TestReadOnlyAndLimitedGrantsStayWithinTheirRole(t *testing.T) {
	f := newFixture(t)
	owner := f.person("regular")
	reader := f.person("regular")
	limited := f.person("regular")
	created := f.call(owner, "POST", "/spaces", map[string]any{"name": "Role grants", "scope_state": "internal"})
	f.want(created, http.StatusCreated, "the owner creates an internal Space")
	sid := created.body["id"].(string)
	f.share(sid, "user", reader.id, "read")
	f.share(sid, "user", limited.id, "limited")
	item := f.call(owner, "POST", "/items", map[string]any{"type": "task", "title": "Role work", "locations": []map[string]string{{"space_id": sid}}})
	f.want(item, http.StatusCreated, "a task to share")
	id := item.body["id"].(string)
	version := f.queryID("SELECT version::text FROM items WHERE id=$1", id)
	f.want(f.call(reader, "GET", "/items/"+id, nil), http.StatusOK, "Read Only views the Task")
	f.want(f.call(reader, "PATCH", "/items/"+id, map[string]any{"title": "by reader"}, "If-Match", `"`+version+`"`), http.StatusForbidden, "Read Only cannot edit")
	f.want(f.call(limited, "GET", "/items/"+id, nil), http.StatusOK, "Limited views the Task")
	f.want(f.call(limited, "PATCH", "/items/"+id, map[string]any{"title": "by limited"}, "If-Match", `"`+version+`"`), http.StatusForbidden, "Limited cannot rename the Task")
	if title := f.queryID("SELECT title FROM items WHERE id=$1", id); title != "Role work" {
		t.Fatalf("a refused edit changed the title: %q", title)
	}
}

// POLICY-01: only Admin and Regular accounts may become Customer members. Viewer and External membership is deferred.
func TestViewerAndExternalCannotBecomeCustomerMembers(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	client := f.createClient(admin, "GRANT-C")["id"].(string)
	for _, userType := range []string{"viewer", "external"} {
		p := f.person(userType)
		r := f.call(admin, "POST", "/clients/"+client+"/members", map[string]any{"user_id": p.id})
		if r.status == http.StatusOK || r.status == http.StatusCreated {
			t.Fatalf("a %s account became a Customer member (status %d)", userType, r.status)
		}
	}
	regular := f.person("regular")
	f.addMember(admin, client, regular)
}
