package httpserver_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/projectexcel"
)

// PROMPT #028 PY-10: an Excel import cannot persist an invalid Pydio folder reference, and a refused import leaves no partial write.
// The column maps to the canonical pydio_folder field by its stable field UUID ("field:<uuid>").
func TestExcelImportCannotPersistAnInvalidPydioReference(t *testing.T) {
	f := pydioFixture(t, false)
	admin := f.person("admin")
	client := f.createClient(admin, "PY10-A")["id"].(string)
	f.addMember(admin, client, admin)
	space := f.scopedSpace("PY10 workspace", "client", client)
	f.share(space, "user", admin.id, "full")
	field := f.call(admin, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{
		"name": "Documents", "type": "text", "config": map[string]any{"pydio_folder": true},
	})
	f.want(field, http.StatusCreated, "canonical field")
	fid := field.body["id"].(string)
	project := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "project", "title": "Destination", "locations": []map[string]any{{"space_id": space}}})
	f.want(project, http.StatusCreated, "destination project")
	destination := project.body["id"].(string)

	workbook := func(title, link string) []byte {
		t.Helper()
		data, err := projectexcel.Write([][]string{{"Title", "Docs"}, {title, link}})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	mapping, _ := json.Marshal(map[string]string{"Docs": "field:" + fid})
	run := func(file []byte, title string) (reply, reply) {
		t.Helper()
		fields := map[string]string{"mapping": string(mapping), "destination": destination}
		preview := f.uploadExcel(admin, space, file, fields)
		digest, _ := preview.body["digest"].(string)
		fields["commit"], fields["digest"] = "true", digest
		return preview, f.uploadExcel(admin, space, file, fields)
	}
	count := func(title string) string { return f.queryID("SELECT count(*)::text FROM items WHERE title=$1", title) }

	// Positive control: a valid reference is accepted and stored under the canonical field.
	preview, commit := run(workbook("Valid import", pydioBase+"/projects/PTM-2026/Drawings"), "Valid import")
	f.want(preview, http.StatusOK, "valid preview")
	if len(preview.body["errors"].([]any)) != 0 {
		t.Fatalf("a valid reference must not be an import error: %v", preview.body["errors"])
	}
	f.want(commit, http.StatusCreated, "valid commit")
	if stored := f.queryID("SELECT fields->>'" + fid + "' FROM items WHERE title='Valid import'"); stored != pydioBase+"/projects/PTM-2026/Drawings" {
		t.Fatalf("the valid reference must be stored: %q", stored)
	}

	// Invalid references: the preview names the problem and the commit persists nothing (no partial write).
	for name, link := range map[string]string{
		"foreign host":           "https://evil.example.com/projects/x",
		"traversal":              pydioBase + "/projects/../secret",
		"credentials in the URL": "https://admin:pw@dms.example.com/projects/x",
	} {
		title := "Refused " + name
		preview, commit := run(workbook(title, link), title)
		if len(preview.body["errors"].([]any)) == 0 && commit.status < 400 {
			t.Fatalf("%s: the invalid reference was accepted by the import", name)
		}
		if commit.status < 400 || count(title) != "0" {
			t.Fatalf("%s: a refused import persisted %s items (status %d)", name, count(title), commit.status)
		}
	}
}
