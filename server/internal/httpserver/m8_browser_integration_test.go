package httpserver_test

import (
	"net/http"
	"os"
	"testing"
)

// TestBrowserM8Flows runs the M8 browser flows (custom item types, status guards) against the real API and PostgreSQL.
func TestBrowserM8Flows(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	admin, editor := f.person("admin"), f.person("regular")
	for p, name := range map[*person][2]string{admin: {"Ada", "Admin"}, editor: {"Eddie", "Editor"}} {
		f.exec("UPDATE users SET first_name=$2,last_name=$3 WHERE id=$1", p.id, name[0], name[1])
	}
	work := f.space("M8 Work")
	f.share(work, "user", admin.id, "full")
	f.share(work, "user", editor.id, "editor")
	task := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": "Guarded task", "locations": []map[string]any{{"space_id": work}}})
	f.want(task, http.StatusCreated, "task")
	runBrowserFixture(t, f, []string{"m8-live.spec.ts"},
		"PMS_FIXTURE_M8_ADMIN="+admin.cookie.String(),
		"PMS_FIXTURE_M8_EDITOR="+editor.cookie.String(),
		"PMS_FIXTURE_M8_EDITOR_ID="+editor.id,
		"PMS_FIXTURE_M8_WORKFLOW="+f.queryID("SELECT id::text FROM workflows WHERE is_default"),
		"PMS_FIXTURE_M8_WORK="+work, "PMS_FIXTURE_M8_TASK="+task.body["id"].(string),
	)
}

// SPC-14 live: the Space "+" menu toggled in a real browser.
func TestBrowserCreateMenu(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Menu Space")
	f.share(space, "user", admin.id, "full")
	runBrowserFixture(t, f, []string{"create-menu-live.spec.ts"}, "PMS_FIXTURE_MENU_ADMIN="+admin.cookie.String(), "PMS_FIXTURE_MENU_SPACE="+space)
}

// E2E-PAR-05 live: an admin builds a branching form routed to Blueprints in the UI, an outside visitor fills the public
// link, and the item with its Blueprint children appears for the branch taken.
func TestBrowserFormBranchingBlueprint(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	admin := f.person("admin")
	f.exec("UPDATE users SET first_name='Ada',last_name='Admin' WHERE id=$1", admin.id)
	work := f.space("Branch Work")
	f.share(work, "user", admin.id, "full")
	for _, key := range []string{"bug", "idea"} {
		f.want(f.call(admin, http.MethodPost, "/item-types", map[string]any{"key": key, "name": map[string]string{"bug": "Bug", "idea": "Idea"}[key], "base": "task"}), http.StatusCreated, key)
	}
	for name, children := range map[string][]string{"Bug triage": {"Reproduce", "Fix"}, "Idea review": {"Evaluate"}} {
		root := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": name, "locations": []map[string]any{{"space_id": work}}})
		f.want(root, http.StatusCreated, name)
		for _, c := range children {
			f.want(f.call(admin, http.MethodPost, "/items", map[string]any{"type": "task", "title": c,
				"locations": []map[string]any{{"space_id": work, "parent_item_id": root.body["id"]}}}), http.StatusCreated, c)
		}
		f.want(f.call(admin, http.MethodPost, "/spaces/"+work+"/blueprints", map[string]any{"name": name, "source_item_id": root.body["id"]}), http.StatusCreated, "blueprint "+name)
	}
	runBrowserFixture(t, f, []string{"form-blueprint-live.spec.ts"}, "PMS_FIXTURE_FB_ADMIN="+admin.cookie.String(), "PMS_FIXTURE_FB_SPACE="+work)
}
