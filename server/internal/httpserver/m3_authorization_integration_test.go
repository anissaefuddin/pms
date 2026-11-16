package httpserver_test

import (
	"strings"
	"testing"
)

// TestM3AuthorizationMatrix extends the policy matrix to approvals, forms, public links, workflows, blueprints and the Stream (S3.6.1).
// Each row names the status the policy intends; a mismatch is a bug in the server, not in the table.
func TestM3AuthorizationMatrix(t *testing.T) {
	f := newFixture(t)
	admin, editor, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	guest, viewer := f.person("external"), f.person("viewer") // guest edits only where shared; viewer never writes
	space, hidden := f.space("M3 matrix A"), f.space("M3 matrix B")
	for _, p := range []*person{admin} {
		f.share(space, "user", p.id, "full")
		f.share(hidden, "user", p.id, "full")
	}
	f.share(space, "user", editor.id, "editor")
	f.share(space, "user", reader.id, "read")
	f.share(space, "user", guest.id, "editor")
	f.share(space, "user", viewer.id, "read")

	item := func(space, title string) string {
		r := f.call(admin, "POST", "/items", map[string]any{"type": "task", "title": title, "locations": []map[string]any{{"space_id": space}}})
		f.want(r, 201, "create "+title)
		return r.body["id"].(string)
	}
	open, secret := item(space, "Visible work"), item(hidden, "Hidden work")
	approval := f.call(admin, "POST", "/items/"+open+"/approvals", map[string]any{"approver_ids": []string{reader.id}})
	f.want(approval, 201, "approval on the visible item")
	approvalID := approval.body["id"].(string)
	hiddenApproval := f.call(admin, "POST", "/items/"+secret+"/approvals", map[string]any{"approver_ids": []string{admin.id}})
	f.want(hiddenApproval, 201, "approval on the hidden item")
	hiddenApprovalID := hiddenApproval.body["id"].(string)

	form := f.call(admin, "POST", "/spaces/"+space+"/forms", formBody(space, nil))
	f.want(form, 201, "private form")
	formID := form.body["id"].(string)
	f.publishForm(admin, formID)
	internal := f.call(admin, "POST", "/spaces/"+space+"/forms", formBody(space, func(b map[string]any) { b["name"] = "Internal form" }))
	f.want(internal, 201, "internal form")
	internalID := internal.body["id"].(string)
	f.publishForm(admin, internalID)
	f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", internalID)
	hiddenForm := f.call(admin, "POST", "/spaces/"+hidden+"/forms", formBody(hidden, func(b map[string]any) { b["name"] = "Hidden form" }))
	f.want(hiddenForm, 201, "form in the hidden Space")
	hiddenFormID := hiddenForm.body["id"].(string)
	f.publishForm(admin, hiddenFormID)
	blueprint := f.call(admin, "POST", "/spaces/"+space+"/blueprints", map[string]any{"name": "Matrix", "source_item_id": open})
	f.want(blueprint, 201, "blueprint")
	blueprintID := blueprint.body["id"].(string)
	answers := map[string]any{"answers": map[string]any{"subject": "Help"}}

	type probe struct {
		who          *person
		method, path string
		body         any
		headers      []string
		want         int
		what         string
	}
	probes := []probe{
		// Anonymous callers reach only the public form endpoints.
		{nil, "GET", "/stream", nil, nil, 401, "anonymous cannot read the Stream"},
		{nil, "GET", "/items/" + open + "/approvals", nil, nil, 401, "anonymous cannot list approvals"},
		{nil, "GET", "/forms", nil, nil, 401, "anonymous cannot list forms"},
		{nil, "GET", "/forms/" + internalID + "/definition", nil, nil, 401, "anonymous cannot read an internal form by id"},
		{nil, "POST", "/forms/" + internalID + "/submissions", answers, nil, 401, "anonymous cannot submit an internal form"},
		{nil, "GET", "/public/forms/" + strings.Repeat("x", 43), nil, nil, 404, "an unknown public token is 404"},
		{nil, "POST", "/public/forms/" + strings.Repeat("x", 43) + "/submissions", answers, nil, 404, "submitting to an unknown token is 404"},
		{nil, "PUT", "/approvals/" + approvalID + "/decision", map[string]any{"decision": "approved"}, nil, 401, "anonymous cannot decide"},
		{nil, "GET", "/me/requests", nil, nil, 401, "anonymous has no requests"},

		// Approvals: listing needs View; requesting and editing need Edit; only named approvers decide.
		{reader, "GET", "/items/" + open + "/approvals", nil, nil, 200, "reader lists approvals"},
		{viewer, "GET", "/items/" + open + "/approvals", nil, nil, 200, "viewer lists approvals"},
		{outsider, "GET", "/items/" + open + "/approvals", nil, nil, 404, "outsider cannot list approvals"},
		{outsider, "GET", "/items/" + secret + "/approvals", nil, nil, 404, "no access to the hidden item"},
		{reader, "POST", "/items/" + open + "/approvals", map[string]any{"approver_ids": []string{reader.id}}, nil, 403, "reader cannot request an approval"},
		{viewer, "POST", "/items/" + open + "/approvals", map[string]any{"approver_ids": []string{reader.id}}, nil, 403, "viewer type cannot request an approval"},
		{outsider, "POST", "/items/" + open + "/approvals", map[string]any{"approver_ids": []string{reader.id}}, nil, 404, "outsider cannot request an approval"},
		{editor, "POST", "/items/" + secret + "/approvals", map[string]any{"approver_ids": []string{admin.id}}, nil, 404, "editor cannot request on a hidden item"},
		{editor, "PUT", "/approvals/" + approvalID + "/decision", map[string]any{"decision": "approved"}, nil, 403, "a viewer-level editor who is not an approver cannot decide"},
		{outsider, "PUT", "/approvals/" + approvalID + "/decision", map[string]any{"decision": "approved"}, nil, 404, "outsider cannot decide"},
		{editor, "PUT", "/approvals/" + hiddenApprovalID + "/decision", map[string]any{"decision": "approved"}, nil, 404, "an approval id on a hidden item looks like it does not exist"},
		{outsider, "PATCH", "/approvals/" + approvalID, map[string]any{"description": "x"}, []string{"If-Match", `"1"`}, 404, "outsider cannot edit an approval"},
		{reader, "PATCH", "/approvals/" + approvalID, map[string]any{"description": "x"}, []string{"If-Match", `"1"`}, 403, "reader cannot edit an approval"},
		{outsider, "DELETE", "/approvals/" + approvalID, nil, nil, 404, "outsider cannot delete an approval"},
		{editor, "DELETE", "/approvals/" + hiddenApprovalID, nil, nil, 404, "cannot delete an approval on a hidden item"},

		// Forms: management needs Edit on the Space; sharing needs Share; private forms are hidden from everyone else.
		{reader, "GET", "/spaces/" + space + "/forms", nil, nil, 403, "reader cannot list a Space's forms"},
		{outsider, "GET", "/spaces/" + space + "/forms", nil, nil, 404, "outsider cannot list forms"},
		{outsider, "GET", "/forms/" + formID, nil, nil, 404, "outsider cannot read a form"},
		{reader, "GET", "/forms/" + formID, nil, nil, 403, "reader cannot read a form's settings"},
		{outsider, "PUT", "/forms/" + formID, formBody(space, nil), []string{"If-Match", `"1"`}, 404, "outsider cannot edit a form"},
		{outsider, "DELETE", "/forms/" + formID, nil, nil, 404, "outsider cannot delete a form"},
		{editor, "DELETE", "/forms/" + hiddenFormID, nil, nil, 404, "a form id from another Space looks like it does not exist"},
		{editor, "PUT", "/forms/" + hiddenFormID + "/access", map[string]any{"access": "internal"}, nil, 404, "cannot publish a hidden form"},
		{editor, "PUT", "/forms/" + formID + "/access", map[string]any{"access": "internal"}, nil, 403, "editor cannot publish to everyone"},
		{guest, "PUT", "/forms/" + formID + "/access", map[string]any{"access": "internal"}, nil, 403, "external type cannot publish"},
		{outsider, "PUT", "/forms/" + formID + "/access", map[string]any{"access": "internal"}, nil, 404, "outsider cannot publish"},
		{editor, "POST", "/forms/" + formID + "/public-link", nil, nil, 403, "editor cannot create a public link"},
		{guest, "POST", "/forms/" + formID + "/public-link", nil, nil, 403, "external type cannot create a public link"},
		{reader, "POST", "/forms/" + formID + "/public-link", nil, nil, 403, "reader cannot create a public link"},
		{outsider, "POST", "/forms/" + formID + "/public-link", nil, nil, 404, "outsider cannot create a public link"},
		{editor, "DELETE", "/forms/" + formID + "/public-link", nil, nil, 403, "editor cannot revoke a public link"},
		{admin, "POST", "/forms/" + hiddenFormID + "/public-link", nil, nil, 200, "admin with Share can create one"},
		{editor, "POST", "/forms/" + hiddenFormID + "/public-link", nil, nil, 404, "editor cannot reach a form in a Space they cannot see"},

		// Filling: private forms are 404 to non-managers; internal forms are open to every signed-in type.
		{outsider, "GET", "/forms/" + formID + "/definition", nil, nil, 404, "private form is hidden from the definition endpoint"},
		{outsider, "POST", "/forms/" + formID + "/submissions", answers, nil, 404, "private form cannot be submitted"},
		{reader, "GET", "/forms/" + formID + "/definition", nil, nil, 404, "private form is hidden from a read-only member"},
		{outsider, "GET", "/forms/" + internalID + "/definition", nil, nil, 200, "anyone signed in reads an internal definition"},
		{guest, "GET", "/forms/" + internalID + "/definition", nil, nil, 200, "external type reads an internal definition"},
		{viewer, "POST", "/forms/" + internalID + "/submissions", answers, nil, 201, "viewer type may file a request"},
		{outsider, "GET", "/forms/available", nil, nil, 200, "available forms list for anyone signed in"},

		// Workflows are admin territory; everyone signed in may read.
		{editor, "POST", "/workflows", map[string]any{"name": "X", "statuses": []map[string]any{{"name": "A", "group": "active", "color": "blue"}}}, nil, 403, "editor cannot create a workflow"},
		{guest, "POST", "/workflows", map[string]any{"name": "X"}, nil, 403, "external type cannot create a workflow"},
		{editor, "GET", "/workflows", nil, nil, 200, "everyone reads workflows"},

		// Blueprints follow Space rights.
		{reader, "GET", "/spaces/" + space + "/blueprints", nil, nil, 200, "reader lists blueprints"},
		{outsider, "GET", "/spaces/" + space + "/blueprints", nil, nil, 404, "outsider cannot list blueprints"},
		{reader, "POST", "/blueprints/" + blueprintID + "/items", map[string]any{"title": "Copy", "locations": []map[string]any{{"space_id": space}}}, nil, 403, "reader cannot create from a blueprint"},
		{outsider, "POST", "/blueprints/" + blueprintID + "/items", map[string]any{"title": "Copy", "locations": []map[string]any{{"space_id": space}}}, nil, 404, "outsider cannot use a blueprint"},
		{outsider, "DELETE", "/blueprints/" + blueprintID, nil, nil, 404, "outsider cannot delete a blueprint"},
	}
	for _, p := range probes {
		got := f.call(p.who, p.method, p.path, p.body, p.headers...)
		if got.status != p.want {
			t.Errorf("%s: %s %s = %d, want %d (%v)", p.what, p.method, p.path, got.status, p.want, got.body)
		}
	}

	// The Stream shows an outsider nothing from either Space, even though both are full of activity.
	f.exec("INSERT INTO activity(item_id,actor_id,action,changes) VALUES ($1,$2,'update','{\"title\":{}}'),($3,$2,'update','{\"title\":{}}')", open, admin.id, secret)
	for who, want := range map[*person]int{outsider: 0, reader: 1} {
		got := f.call(who, "GET", "/stream", nil)
		if got.status != 200 || len(got.items()) < want || strings.Contains(string(mustJSON(got.body)), secret) {
			t.Errorf("stream for %s: %d %v", who.id, got.status, got.body)
		}
		if who == outsider && len(got.items()) != 0 {
			t.Errorf("outsider sees %d stream entries", len(got.items()))
		}
	}

	// Refused calls changed nothing.
	if got := f.queryID("SELECT access FROM request_forms WHERE id=$1", formID); got != "private" {
		t.Errorf("form access changed by a refused call: %s", got)
	}
	if got := f.queryID("SELECT coalesce(public_token,'none') FROM request_forms WHERE id=$1", formID); got != "none" {
		t.Errorf("public link created by a refused call")
	}
	if got := f.queryID("SELECT count(*)::text FROM approvals WHERE item_id=$1", open); got != "1" {
		t.Errorf("approvals on the item: %s", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM workflows"); got != "1" {
		t.Errorf("workflows: %s", got)
	}
}
