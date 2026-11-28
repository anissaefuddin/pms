package httpserver_test

import (
	"net/http"
	"testing"
	"time"
)

// B3-17: a public Gantt snapshot is a bearer link, so the token alone is not enough. It keeps working only while the person who
// created it can still view the project, and it stops when that access ends (membership revoked or the person deactivated).
func TestGanttSnapshotLinkEndsWithCreatorAccess(t *testing.T) {
	f := newFixture(t)
	admin, creator := f.person("admin"), f.person("regular")
	client := f.createClient(admin, "GS-A")["id"].(string)
	f.addMember(admin, client, admin)
	f.addMember(admin, client, creator)
	space := f.scopedSpace("GS client", "client", client)
	f.share(space, "user", admin.id, "full")
	f.share(space, "user", creator.id, "full")
	project := f.itemIn(space, "client", client, admin.id, "Client plan project")
	f.exec("UPDATE items SET type='project' WHERE id=$1", project)

	made := f.call(creator, http.MethodPost, "/items/"+project+"/gantt-snapshots", map[string]any{
		"name": "Client plan", "expires_at": time.Now().Add(time.Hour), "show_assignees": false,
	})
	f.want(made, http.StatusCreated, "publish snapshot")
	token := made.body["token"].(string)

	f.want(f.call(nil, http.MethodGet, "/public/gantt/"+token, nil), http.StatusOK, "link works while the creator can view the project")
	f.revokeMember(admin, client, creator)
	f.want(f.call(nil, http.MethodGet, "/public/gantt/"+token, nil), http.StatusNotFound, "link ends when the creator loses Customer access")
}
