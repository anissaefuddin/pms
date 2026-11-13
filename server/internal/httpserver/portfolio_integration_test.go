package httpserver_test

import "testing"

func portfolioProject(f *fixture, actor *person, space, title string) (string, string) {
	f.t.Helper()
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project',$1,$2) RETURNING id::text", title, actor.id)
	task := f.queryID("INSERT INTO items(type,title,created_by,start_date,due_date) VALUES ('task',$1,$2,'2026-11-02','2026-11-04') RETURNING id::text", title+" task", actor.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,NULL,'U'),($3,$2,$1,'U')", project, space, task)
	return project, task
}

func TestPortfolioTimelineCrossProjectDependencyAndPolicy(t *testing.T) {
	f := newFixture(t)
	admin, reader := f.person("admin"), f.person("regular")
	firstSpace, secondSpace := f.space("Portfolio A"), f.space("Portfolio B")
	first, a := portfolioProject(f, admin, firstSpace, "Alpha")
	second, b := portfolioProject(f, admin, secondSpace, "Beta")
	if got := f.call(admin, "POST", dependencyPath(b), dependencyInput(a, "FS")); got.status != 201 {
		t.Fatalf("cross-project dependency: %+v", got)
	}

	body := map[string]any{"project_ids": []string{first, second}}
	got := f.call(admin, "POST", "/portfolio/timeline", body)
	if got.status != 200 || len(got.body["projects"].([]any)) != 2 || len(got.body["items"].([]any)) != 4 || len(got.body["dependencies"].([]any)) != 1 {
		t.Fatalf("portfolio timeline: %+v", got)
	}
	f.share(firstSpace, "user", reader.id, "read")
	if hidden := f.call(reader, "POST", "/portfolio/timeline", body); hidden.status != 404 {
		t.Fatalf("hidden project must not be disclosed: %+v", hidden)
	}
	visible := f.call(reader, "POST", "/portfolio/timeline", map[string]any{"project_ids": []string{first}})
	if visible.status != 200 || len(visible.body["items"].([]any)) != 2 || len(visible.body["dependencies"].([]any)) != 0 {
		t.Fatalf("visible project: %+v", visible)
	}
	if invalid := f.call(admin, "POST", "/portfolio/timeline", map[string]any{"project_ids": []string{}}); invalid.status != 422 {
		t.Fatalf("empty portfolio: %+v", invalid)
	}
}
