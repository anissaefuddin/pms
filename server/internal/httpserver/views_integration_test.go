package httpserver_test

import (
	"net/http"
	"testing"
)

func TestViewLifecycle(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	viewer := f.person("viewer")
	space := f.space("View scope")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	f.share(space, "user", viewer.id, "read")
	list := "/spaces/" + space + "/views"
	root := f.call(admin, http.MethodGet, list, nil)
	if root.status != 200 || len(root.items()) != 5 || root.items()[4]["is_default"] != true || root.items()[4]["type"] != "files" ||
		root.items()[0]["type"] != "table" || root.items()[1]["type"] != "board" {
		t.Fatalf("seeded views: %d %v", root.status, root.body)
	}
	board := root.items()[1]["id"].(string)
	personal := f.call(viewer, http.MethodPost, "/views/"+board+"/duplicate", map[string]any{"name": "Mine", "visibility": "personal"})
	if personal.status != 201 || personal.body["type"] != "board" {
		t.Fatalf("duplicate: %d %v", personal.status, personal.body)
	}
	if got := f.call(admin, http.MethodGet, list, nil); got.status != 200 || len(got.items()) != 5 {
		t.Fatalf("public list: %d %v", got.status, got.body)
	}
	if got := f.call(viewer, http.MethodGet, list, nil); got.status != 200 || len(got.items()) != 6 {
		t.Fatalf("owner list: %d %v", got.status, got.body)
	}
	updated := f.call(admin, http.MethodPatch, "/views/"+board, map[string]any{"is_default": true, "name": "Planning", "position": 0})
	if updated.status != 200 || updated.body["is_default"] != true || updated.body["position"] != float64(0) {
		t.Fatalf("update: %d %v", updated.status, updated.body)
	}
	if got := f.call(admin, http.MethodDelete, "/views/"+board, nil); got.status != 409 {
		t.Fatalf("default delete: %d %v", got.status, got.body)
	}
	if got := f.call(viewer, http.MethodDelete, "/views/"+personal.body["id"].(string), nil); got.status != 204 {
		t.Fatalf("personal delete: %d %v", got.status, got.body)
	}
	project := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "project", "title": "Roadmap", "default_view_type": "board",
		"locations": []map[string]any{{"space_id": space}}})
	if project.status != 201 {
		t.Fatalf("project: %d %v", project.status, project.body)
	}
	projectID := project.body["id"].(string)
	projectViews := "/items/" + projectID + "/views"
	childViews := f.call(admin, http.MethodGet, projectViews, nil)
	if childViews.status != 200 || len(childViews.items()) != 3 || childViews.items()[0]["type"] != "table" || childViews.items()[2]["type"] != "files" ||
		childViews.items()[1]["type"] != "board" || childViews.items()[1]["is_default"] != true {
		t.Fatalf("project default view: %d %v", childViews.status, childViews.body)
	}
	for _, viewType := range []string{"gantt", "calendar"} {
		t.Run(viewType, func(t *testing.T) {
			input := map[string]any{"scope_item_id": projectID, "type": viewType, "name": viewType, "visibility": "public"}
			if denied := f.call(viewer, http.MethodPost, "/views", input); denied.status != 403 {
				t.Fatalf("viewer create public: %d %v", denied.status, denied.body)
			}
			created := f.call(admin, http.MethodPost, "/views", input)
			if created.status != 201 || created.body["type"] != viewType || created.body["is_default"] != false {
				t.Fatalf("create: %d %v", created.status, created.body)
			}
			viewPath := "/views/" + created.body["id"].(string)
			copy := f.call(viewer, http.MethodPost, viewPath+"/duplicate", map[string]any{"name": "Personal copy", "visibility": "personal"})
			if copy.status != 201 || copy.body["type"] != viewType {
				t.Fatalf("duplicate: %d %v", copy.status, copy.body)
			}
			if got := f.call(admin, http.MethodGet, projectViews, nil); got.status != 200 || len(got.items()) != 4 {
				t.Fatalf("public list: %d %v", got.status, got.body)
			}
			if got := f.call(viewer, http.MethodGet, projectViews, nil); got.status != 200 || len(got.items()) != 5 {
				t.Fatalf("owner list: %d %v", got.status, got.body)
			}
			updated := f.call(admin, http.MethodPatch, viewPath, map[string]any{"name": "Planning " + viewType, "is_default": true})
			if updated.status != 200 || updated.body["name"] != "Planning "+viewType || updated.body["is_default"] != true {
				t.Fatalf("rename and set default: %d %v", updated.status, updated.body)
			}
			if got := f.call(admin, http.MethodDelete, viewPath, nil); got.status != 409 {
				t.Fatalf("default delete: %d %v", got.status, got.body)
			}
			boardPath := "/views/" + childViews.items()[1]["id"].(string)
			if got := f.call(admin, http.MethodPatch, boardPath, map[string]any{"is_default": true}); got.status != 200 {
				t.Fatalf("restore default: %d %v", got.status, got.body)
			}
			if got := f.call(admin, http.MethodDelete, viewPath, nil); got.status != 204 {
				t.Fatalf("delete: %d %v", got.status, got.body)
			}
			if got := f.call(viewer, http.MethodDelete, "/views/"+copy.body["id"].(string), nil); got.status != 204 {
				t.Fatalf("personal delete: %d %v", got.status, got.body)
			}
		})
	}
}

func TestDefaultViewTypesSeedChosenView(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Default views")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	for _, viewType := range []string{"gantt", "calendar", "files", "chart", "table", "board"} {
		created := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "project", "title": viewType, "default_view_type": viewType,
			"locations": []map[string]any{{"space_id": space}}})
		if created.status != 201 {
			t.Fatalf("%s project: %d %v", viewType, created.status, created.body)
		}
		views := f.call(admin, http.MethodGet, "/items/"+created.body["id"].(string)+"/views", nil)
		var defaults []string
		for _, view := range views.items() {
			if view["is_default"] == true {
				defaults = append(defaults, view["type"].(string))
			}
		}
		if len(defaults) != 1 || defaults[0] != viewType {
			t.Fatalf("%s: default views = %v (%d %v)", viewType, defaults, views.status, views.body)
		}
	}
	for _, kind := range []string{"project", "folder"} {
		created := f.call(admin, http.MethodPost, "/items", map[string]any{"type": kind, "title": "unchosen " + kind, "locations": []map[string]any{{"space_id": space}}})
		f.want(created, http.StatusCreated, kind)
		if got := f.queryID("SELECT string_agg(type::text, ',') FROM views WHERE scope_item_id=$1 AND is_default", created.body["id"]); got != "files" {
			t.Fatalf("a %s without a chosen view opens on %q", kind, got)
		}
	}
	if rejected := f.call(admin, http.MethodPost, "/items", map[string]any{"type": "project", "title": "x", "default_view_type": "list",
		"locations": []map[string]any{{"space_id": space}}}); rejected.status != 422 {
		t.Fatalf("unknown view accepted: %d %v", rejected.status, rejected.body)
	}
}
