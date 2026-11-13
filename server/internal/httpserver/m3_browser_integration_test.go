package httpserver_test

import (
	"net/http"
	"os"
	"testing"
)

// TestBrowserM3Flows seeds workflow, form and access data and runs the four M3 browser flows (S3.6.1) against the real API and PostgreSQL.
func TestBrowserM3Flows(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	admin, editor, approver, requester := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	for p, name := range map[*person][2]string{admin: {"Ada", "Admin"}, editor: {"Eddie", "Editor"}, approver: {"Avery", "Approver"}, requester: {"Remy", "Requester"}} {
		f.exec("UPDATE users SET first_name=$2,last_name=$3 WHERE id=$1", p.id, name[0], name[1])
	}
	intake, work := f.space("M3 Intake"), f.space("M3 Work")
	f.share(intake, "user", admin.id, "full")
	f.share(work, "user", admin.id, "full")
	f.share(work, "user", editor.id, "editor")
	f.share(work, "user", approver.id, "read")
	// The requester has no access to either Space.
	create := func(body map[string]any) string {
		r := f.call(admin, http.MethodPost, "/items", body)
		f.want(r, http.StatusCreated, "create "+body["title"].(string))
		return r.body["id"].(string)
	}
	task := func(title string) string {
		return create(map[string]any{"type": "task", "title": title, "locations": []map[string]any{{"space_id": work}}})
	}
	locked, auto := task("Locked task"), task("Auto-assign task")
	project := create(map[string]any{"type": "project", "title": "M3 Platform", "locations": []map[string]any{{"space_id": work}}})
	sibling := create(map[string]any{"type": "task", "title": "Sibling secret", "locations": []map[string]any{{"space_id": work, "parent_item_id": project}}})
	private := f.call(admin, http.MethodPost, "/spaces/"+intake+"/forms", formBody(work, func(b map[string]any) { b["name"] = "Private M3 form" }))
	f.want(private, http.StatusCreated, "private form")

	public := f.call(admin, http.MethodPost, "/spaces/"+intake+"/forms", formBody(work, func(b map[string]any) { b["name"] = "Public M3 form" }))
	f.want(public, http.StatusCreated, "public form")
	link := f.call(admin, http.MethodPost, "/forms/"+public.body["id"].(string)+"/public-link", nil)
	f.want(link, http.StatusOK, "public link")

	runBrowserFixture(t, f, []string{"m3-live.spec.ts"},
		"PMS_FIXTURE_M3_ADMIN="+admin.cookie.String(),
		"PMS_FIXTURE_M3_EDITOR="+editor.cookie.String(),
		"PMS_FIXTURE_M3_APPROVER="+approver.cookie.String(),
		"PMS_FIXTURE_M3_REQUESTER="+requester.cookie.String(),
		"PMS_FIXTURE_M3_EDITOR_ID="+editor.id,
		"PMS_FIXTURE_M3_ADMIN_ID="+admin.id,
		"PMS_FIXTURE_M3_WORKFLOW="+f.queryID("SELECT id::text FROM workflows WHERE is_default"),
		"PMS_FIXTURE_M3_INTAKE="+intake, "PMS_FIXTURE_M3_WORK="+work,
		"PMS_FIXTURE_M3_LOCKED="+locked, "PMS_FIXTURE_M3_AUTO="+auto,
		"PMS_FIXTURE_M3_PUBLIC_TOKEN="+link.body["public_token"].(string),
		"PMS_FIXTURE_M3_PROJECT="+project, "PMS_FIXTURE_M3_SIBLING="+sibling,
	)
}
