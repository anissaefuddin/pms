package httpserver_test

import (
	"fmt"
	"testing"
)

// DEF-018: a status change records the revision of the rules it was made under; editing the rules later does not change history.
func TestStatusChangesRecordTheWorkflowRevision(t *testing.T) {
	f := newFixture(t)
	admin, member, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Flow")
	f.share(space, "user", member.id, "editor")
	status := func(name string) string {
		return f.queryID("SELECT id::text FROM statuses WHERE name=$1 AND workflow_id=(SELECT id FROM workflows WHERE is_default)", name)
	}
	made := f.call(member, "POST", "/items", map[string]any{"type": "task", "title": "T", "locations": []map[string]any{{"space_id": space}}})
	id, version := made.body["id"].(string), 1
	move := func(to, request string) {
		t.Helper()
		f.want(f.call(member, "PATCH", "/items/"+id, map[string]any{"status_id": status(to)}, "If-Match", fmt.Sprintf(`"%d"`, version), "X-Request-ID", request), 200, "to "+to)
		version++
	}
	latest := func() map[string]any {
		t.Helper()
		got := f.call(member, "GET", "/items/"+id+"/activity", nil)
		f.want(got, 200, "activity")
		return got.items()[0]
	}
	move("In Progress", "req-flow-1")
	first := latest()
	flow := first["workflow"].(map[string]any)
	if flow["revision"] != float64(1) || first["request_id"] != "req-flow-1" {
		t.Fatalf("first change: %v", first)
	}
	move("On Hold", "req-flow-2")
	if got := latest()["workflow"].(map[string]any)["revision"]; got != float64(1) {
		t.Fatalf("unchanged rules keep the revision: %v", got)
	}
	f.exec("UPDATE statuses SET require_children_done=true WHERE id=$1", status("Completed"))
	move("In Progress", "req-flow-3")
	if got := latest()["workflow"].(map[string]any)["revision"]; got != float64(2) {
		t.Fatalf("changed rules make a new revision: %v", got)
	}
	path := "/workflows/" + flow["id"].(string) + "/revisions/"
	one, two := f.call(admin, "GET", path+"1", nil), f.call(admin, "GET", path+"2", nil)
	f.want(one, 200, "revision 1")
	f.want(two, 200, "revision 2")
	if fmt.Sprint(one.body["definition"]) == fmt.Sprint(two.body["definition"]) {
		t.Fatal("revisions must differ")
	}
	f.want(f.call(member, "GET", path+"1", nil), 403, "rule definitions are for administrators")
	f.want(f.call(outsider, "GET", "/items/"+id+"/activity", nil), 404, "history stays private")
	f.want(f.call(admin, "GET", path+"9", nil), 404, "unknown revision")
}
