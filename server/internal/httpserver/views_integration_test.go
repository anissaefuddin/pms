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
	f.share(space, "user", viewer.id, "read")
	list := "/spaces/" + space + "/views"
	root := f.call(admin, http.MethodGet, list, nil)
	if root.status != 200 || len(root.items()) != 2 || root.items()[0]["is_default"] != true {
		t.Fatalf("seeded views: %d %v", root.status, root.body)
	}
	board := root.items()[1]["id"].(string)
	personal := f.call(viewer, http.MethodPost, "/views/"+board+"/duplicate", map[string]any{"name": "Mine", "visibility": "personal"})
	if personal.status != 201 || personal.body["type"] != "board" {
		t.Fatalf("duplicate: %d %v", personal.status, personal.body)
	}
	if got := f.call(admin, http.MethodGet, list, nil); got.status != 200 || len(got.items()) != 2 {
		t.Fatalf("public list: %d %v", got.status, got.body)
	}
	if got := f.call(viewer, http.MethodGet, list, nil); got.status != 200 || len(got.items()) != 3 {
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
	childViews := f.call(admin, http.MethodGet, "/items/"+project.body["id"].(string)+"/views", nil)
	if childViews.status != 200 || len(childViews.items()) != 2 || childViews.items()[1]["type"] != "board" || childViews.items()[1]["is_default"] != true {
		t.Fatalf("project default view: %d %v", childViews.status, childViews.body)
	}
}
