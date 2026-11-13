package httpserver_test

import (
	"strconv"
	"testing"
)

// WF-03B: entering a status starts an approval for the configured approvers; the result moves the item to the chosen status.
func TestEnteringAStatusStartsAnApprovalWhoseResultMovesTheItem(t *testing.T) {
	f := newFixture(t)
	admin, editor, approver, noAccess := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	space := f.space("Auto approval")
	f.share(space, "user", editor.id, "full")
	f.share(space, "user", approver.id, "read")
	stat := func(group string) string {
		return f.queryID("SELECT id::text FROM statuses WHERE grp=$1::status_group ORDER BY position LIMIT 1", group)
	}
	review, approved, rejected := stat("deferred"), stat("completed"), stat("cancelled")

	for name, bad := range map[string]map[string]any{
		"unknown approver": {"approval_approver_ids": []string{noAccess.id[:8] + "-0000-4000-8000-000000000000"}},
		"self target":      {"approved_status_id": review},
		"other workflow":   {"approved_status_id": "00000000-0000-4000-8000-000000000001"},
	} {
		if got := f.call(admin, "PATCH", "/statuses/"+review, bad); got.status != 422 {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	f.want(f.call(editor, "PATCH", "/statuses/"+review, map[string]any{"approval_approver_ids": []string{approver.id}}), 403, "admins only")
	cfg := f.call(admin, "PATCH", "/statuses/"+review, map[string]any{"approval_approver_ids": []string{approver.id, noAccess.id}, "approved_status_id": approved, "rejected_status_id": rejected})
	f.want(cfg, 200, "configure")

	item := f.call(editor, "POST", "/items", map[string]any{"type": "task", "title": "Needs sign-off", "locations": []map[string]any{{"space_id": space}}})
	f.want(item, 201, "item")
	id, version := item.body["id"].(string), 1
	move := func(p *person, to string) reply {
		got := f.call(p, "PATCH", "/items/"+id, map[string]any{"status_id": to}, "If-Match", `"`+strconv.Itoa(version)+`"`)
		if got.status == 200 {
			version = int(got.body["version"].(float64))
		}
		return got
	}
	f.want(move(editor, review), 200, "enter the review status")
	list := f.call(editor, "GET", "/items/"+id+"/approvals", nil).items()
	if len(list) != 1 || list[0]["status"] != "pending" || list[0]["total"] != float64(1) {
		t.Fatalf("approval not started or wrongly addressed (the approver without access must be left out): %+v", list)
	}
	aid := list[0]["id"].(string)
	if n := f.queryID("SELECT count(*)::text FROM notifications WHERE kind='approval_requested' AND recipient_id=$1", approver.id); n != "1" {
		t.Fatalf("approver not notified: %s", n)
	}
	f.want(f.call(approver, "PUT", "/approvals/"+aid+"/decision", map[string]any{"decision": "approved", "comment": "ok"}), 200, "approve")
	got := f.call(editor, "GET", "/items/"+id, nil)
	if got.body["status"].(map[string]any)["id"] != approved {
		t.Fatalf("approved item did not move: %+v", got.body["status"])
	}
	version = int(got.body["version"].(float64))

	// A rejection moves it to the rejected status; an item that was moved by hand meanwhile stays where it is.
	f.want(move(admin, f.queryID("SELECT id::text FROM statuses WHERE grp='active' ORDER BY position LIMIT 1")), 200, "back to active")
	f.want(move(editor, review), 200, "enter review again")
	list = f.call(editor, "GET", "/items/"+id+"/approvals", nil).items()
	var second string
	for _, a := range list {
		if a["status"] == "pending" {
			second = a["id"].(string)
		}
	}
	f.want(f.call(approver, "PUT", "/approvals/"+second+"/decision", map[string]any{"decision": "rejected"}), 200, "reject")
	if got := f.call(editor, "GET", "/items/"+id, nil); got.body["status"].(map[string]any)["id"] != rejected {
		t.Fatalf("rejected item did not move: %+v", got.body["status"])
	}
}
