package httpserver_test

import (
	"testing"
	"time"
)

func TestGanttSnapshotsHidePrivateDataAndExpire(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	space := f.space("Snapshot")
	f.share(space, "user", admin.id, "full")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES('project','Launch',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES($1,$2,'A')", project, space)
	task := f.queryID("INSERT INTO items(type,title,created_by,start_date,due_date) VALUES('task','Secret plan',$1,'2026-11-01','2026-11-03') RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES($1,$2,$3,'A')", task, space, project)
	f.exec("INSERT INTO item_assignees(item_id,user_id) VALUES($1,$2)", task, admin.id)

	made := f.call(admin, "POST", "/items/"+project+"/gantt-snapshots", map[string]any{"name": "Client plan", "expires_at": time.Now().Add(time.Hour), "show_assignees": false})
	f.want(made, 201, "create snapshot")
	token := made.body["token"].(string)
	view := f.call(nil, "GET", "/public/gantt/"+token, nil)
	f.want(view, 200, "public snapshot")
	items := view.body["items"].([]any)
	if got := items[0].(map[string]any); got["title"] != "Secret plan" || got["assignees"] != nil {
		t.Fatalf("masked snapshot = %#v", got)
	}

	f.exec("UPDATE items SET title='Changed later' WHERE id=$1", task)
	view = f.call(nil, "GET", "/public/gantt/"+token, nil)
	if got := view.body["items"].([]any)[0].(map[string]any)["title"]; got != "Secret plan" {
		t.Fatalf("snapshot changed: %v", got)
	}
	// Only people who can edit the project may list the history, make or revoke links.
	viewer, outsider := f.person("regular"), f.person("regular")
	f.share(space, "user", viewer.id, "read")
	body := map[string]any{"name": "X", "expires_at": time.Now().Add(time.Hour)}
	f.want(f.call(viewer, "POST", "/items/"+project+"/gantt-snapshots", body), 403, "read cannot publish")
	f.want(f.call(outsider, "POST", "/items/"+project+"/gantt-snapshots", body), 404, "hidden project")
	f.want(f.call(outsider, "DELETE", "/gantt-snapshots/"+made.body["id"].(string), nil), 404, "hidden snapshot")
	f.want(f.call(viewer, "DELETE", "/gantt-snapshots/"+made.body["id"].(string), nil), 403, "read cannot revoke")
	list := f.call(viewer, "GET", "/items/"+project+"/gantt-snapshots", nil)
	f.want(list, 200, "history")
	if len(list.items()) != 1 || list.items()[0]["token"] != nil {
		t.Fatalf("history must not repeat the link: %v", list.body)
	}
	f.want(f.call(admin, "POST", "/items/"+project+"/gantt-snapshots", map[string]any{"name": "Late", "expires_at": time.Now().Add(-time.Hour)}), 422, "past expiry")
	other := f.call(admin, "POST", "/items/"+project+"/gantt-snapshots", map[string]any{"name": "Revoked", "expires_at": time.Now().Add(time.Hour), "show_assignees": true})
	f.want(other, 201, "second")
	named := f.call(nil, "GET", "/public/gantt/"+other.body["token"].(string), nil)
	if a := named.body["items"].([]any)[0].(map[string]any)["assignees"]; a == nil {
		t.Fatalf("opt-in shows assignee names: %v", named.body)
	}
	f.want(f.call(admin, "DELETE", "/gantt-snapshots/"+other.body["id"].(string), nil), 204, "revoke")
	f.want(f.call(nil, "GET", "/public/gantt/"+other.body["token"].(string), nil), 404, "revoked link")
	f.exec("UPDATE gantt_snapshots SET expires_at=now()-interval '1 second'")
	f.want(f.call(nil, "GET", "/public/gantt/"+token, nil), 404, "expired snapshot")
	f.want(f.call(nil, "GET", "/public/gantt/not-a-token", nil), 404, "invalid token")
}
