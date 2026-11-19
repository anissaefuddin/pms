package httpserver_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// RF-08: an internal form names its owning Space only to people who can view that Space. The catalog, the fill page,
// the public link and the submission reply never carry Space metadata to a requester without Space access, and
// submitting does not give the requester access to the destination Item.
func TestFormSpaceMetadataFollowsSpaceAccess(t *testing.T) {
	f := newFixture(t)
	admin, editor, owner, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular"), f.person("regular")
	intake := f.space("Intake")
	f.share(intake, "user", editor.id, "editor")
	f.share(intake, "user", owner.id, "full")
	f.share(intake, "user", reader.id, "read")
	made := f.call(editor, "POST", "/spaces/"+intake+"/forms", formBody(intake, nil))
	f.want(made, 201, "build form")
	id := made.body["id"].(string)
	f.publishForm(editor, id)
	f.want(f.call(owner, "PUT", "/forms/"+id+"/access", map[string]any{"access": "internal"}), 200, "publish internal")

	// Positive: people who can view Intake see its name in the catalog.
	for name, p := range map[string]*person{"editor": editor, "reader": reader} {
		got := f.call(p, "GET", "/forms/available", nil)
		f.want(got, 200, "catalog for "+name)
		if len(got.items()) != 1 || got.items()[0]["space_name"] != "Intake" {
			t.Fatalf("%s should see the Space name: %v", name, got.body)
		}
	}
	// D-17 A1 (metadata leakage): a Workspace admin without access to Intake is offered the form but not its Space.
	adminCatalog := f.call(admin, "GET", "/forms/available", nil)
	f.want(adminCatalog, 200, "catalog for admin")
	assertNoSpaceMetadata(t, "admin catalog without Space access", adminCatalog.body, intake)

	// Negative: the requester has no Space access. The form is offered, but no Space name or id appears anywhere.
	catalog := f.call(outsider, "GET", "/forms/available", nil)
	f.want(catalog, 200, "catalog for outsider")
	if len(catalog.items()) != 1 || catalog.items()[0]["name"] != "IT request" {
		t.Fatalf("outsider is offered the form: %v", catalog.body)
	}
	assertNoSpaceMetadata(t, "outsider catalog", catalog.body, intake)
	definition := f.call(outsider, "GET", "/forms/"+id+"/definition", nil)
	f.want(definition, 200, "outsider fills the form")
	assertNoSpaceMetadata(t, "outsider definition", definition.body, intake)

	// Public link: anonymous visitors get the form without any internal Space metadata.
	// D-17 A1: creating a public link needs access to the form's Space; the admin has none here, so the owner makes it.
	link := f.call(owner, "POST", "/forms/"+id+"/public-link", nil)
	f.want(link, 200, "create public link")
	token, _ := link.body["public_token"].(string)
	public := f.call(nil, "GET", "/public/forms/"+token, nil)
	f.want(public, 200, "public form")
	assertNoSpaceMetadata(t, "public form", public.body, intake)

	// Submitting creates the Item in Intake but grants the requester nothing: the reply has no Space data, and the
	// requester cannot read the Item afterwards. Editors still can.
	sent := f.submit(outsider, id, map[string]any{"subject": "VPN down"})
	f.want(sent, 201, "outsider submits")
	assertNoSpaceMetadata(t, "submission reply", sent.body, intake)
	itemID, _ := sent.body["item_id"].(string)
	f.want(f.call(outsider, "GET", "/items/"+itemID, nil), 404, "requester cannot read the created item")
	f.want(f.call(editor, "GET", "/items/"+itemID, nil), 200, "a Space editor reads the created item")
}

func assertNoSpaceMetadata(t *testing.T, what string, body map[string]any, spaceID string) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"space_name", "space_id", "Intake", spaceID} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("%s leaks %q: %s", what, leak, raw)
		}
	}
}
