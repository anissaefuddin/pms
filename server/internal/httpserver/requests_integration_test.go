package httpserver_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestMyRequests(t *testing.T) {
	f := newFixture(t)
	admin, requester, other, viewer := f.person("admin"), f.person("regular"), f.person("regular"), f.person("viewer")
	work := f.space("Work")
	project := f.call(admin, "POST", "/items", map[string]any{"type": "project", "title": "Platform", "locations": []map[string]any{{"space_id": work}}})
	f.want(project, 201, "project")
	sibling := f.call(admin, "POST", "/items", map[string]any{"type": "task", "title": "Secret", "locations": []map[string]any{{"space_id": work, "parent_item_id": project.body["id"]}}})
	f.want(sibling, 201, "sibling task")
	form := f.call(admin, "POST", "/spaces/"+work+"/forms", formBody(work, func(b map[string]any) {
		b["response"].(map[string]any)["location"] = map[string]any{"space_id": work, "parent_item_id": project.body["id"]}
	}))
	f.want(form, 201, "form into the project")
	formID := form.body["id"].(string)
	f.publishForm(admin, formID)
	f.want(f.call(admin, "PUT", "/forms/"+formID+"/access", map[string]any{"access": "internal"}), 200, "publish")

	sent := f.submit(requester, formID, map[string]any{"subject": "First"})
	f.want(sent, 201, "submit")
	item := sent.body["item_id"].(string)
	path := "/items/" + item

	// Submitting gives no access to the Item it created (RF-08): the requester sees the submission in My requests only.
	f.want(f.call(requester, "GET", path, nil), 404, "requester cannot open the created item")
	f.want(f.call(requester, "PATCH", path, map[string]any{"title": "Mine now"}, "If-Match", `"1"`), 404, "no edits")
	f.want(f.call(requester, "DELETE", path, nil), 404, "no delete")
	for name, p := range map[string]string{
		"the project":          "/items/" + project.body["id"].(string),
		"a sibling task":       "/items/" + sibling.body["id"].(string),
		"the project children": "/items/" + project.body["id"].(string) + "/children",
		"the Space":            "/spaces/" + work,
		"the Space items":      "/spaces/" + work + "/items",
	} {
		if r := f.call(requester, "GET", p, nil); r.status != 404 {
			t.Fatalf("requester must not reach %s: %d %v", name, r.status, r.body)
		}
	}
	f.want(f.call(other, "GET", path, nil), 404, "another user")
	comment := map[string]any{"body": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Any update?"}}}}}}
	f.want(f.call(requester, "POST", path+"/comments", comment), 404, "requester cannot comment on the created item")
	f.want(f.call(requester, "GET", path+"/attachments", nil), 404, "requester cannot see its attachments")

	// A viewer submits too: the same rule applies, so the user type ceiling is not reachable through submissions.
	vs := f.submit(viewer, formID, map[string]any{"subject": "Viewer's"})
	f.want(vs, 201, "viewer submits")
	f.want(f.call(viewer, "GET", "/items/"+vs.body["item_id"].(string), nil), 404, "viewer cannot open the created item")

	// Status changes reach a requester without access as a notification with no Item link; the text holds no Space data.
	status := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' LIMIT 1")
	f.want(f.call(admin, "PATCH", path, map[string]any{"status_id": status}, "If-Match", `"1"`), 200, "team completes the request")
	if n := f.queryID("SELECT count(*)::text FROM notifications WHERE item_id IS NULL AND space_id IS NULL AND recipient_id=$1 AND kind='status_changed' AND snippet LIKE 'Your request%is now: %'", requester.id); n != "1" {
		t.Fatalf("requester gets one status notification without an Item link, got %s", n)
	}
	if n := f.queryID("SELECT count(*)::text FROM notifications WHERE item_id=$1 AND recipient_id=$2", item, requester.id); n != "0" {
		t.Fatalf("requester without item access gets no item-linked notifications, got %s", n)
	}
	if inbox := f.call(requester, "GET", "/notifications", nil); inbox.status != 200 || !strings.Contains(fmt.Sprint(inbox.body), "is now: ") {
		t.Fatalf("requester sees the status in the inbox: %d %v", inbox.status, inbox.body)
	}

	// My requests: own submissions only, newest first, with the live status, paged by cursor.
	f.want(f.submit(requester, formID, map[string]any{"subject": "Second"}), 201, "second")
	f.want(f.submit(requester, formID, map[string]any{"subject": "Third"}), 201, "third")
	page := f.call(requester, "GET", "/me/requests?limit=2", nil)
	if page.status != 200 || len(page.items()) != 2 || page.items()[0]["title"] != "Third" || page.body["next_cursor"] == nil {
		t.Fatalf("first page: %v", page.body)
	}
	rest := f.call(requester, "GET", "/me/requests?limit=2&cursor="+page.body["next_cursor"].(string), nil)
	if len(rest.items()) != 1 || rest.items()[0]["title"] != "First" || rest.body["next_cursor"] != nil {
		t.Fatalf("second page: %v", rest.body)
	}
	first := rest.items()[0]
	if first["form_name"] != "IT request" || first["status"].(map[string]any)["group"] != "completed" || first["item_id"] != item {
		t.Fatalf("request row shows the form and the current status: %v", first)
	}
	if mine := f.call(other, "GET", "/me/requests", nil); mine.status != 200 || len(mine.items()) != 0 {
		t.Fatalf("others see none of them: %v", mine.body)
	}
	f.want(f.call(requester, "GET", "/me/requests?limit=0", nil), 422, "bad limit")
	f.want(f.call(requester, "GET", "/me/requests?cursor=x", nil), 422, "bad cursor")

	// Deleted items leave the list and lose the requester's access.
	f.want(f.call(admin, "DELETE", path, nil), http.StatusNoContent, "team deletes the item")
	if all := f.call(requester, "GET", "/me/requests", nil); len(all.items()) != 2 {
		t.Fatalf("deleted requests disappear: %v", all.body)
	}
	f.want(f.call(requester, "GET", path, nil), 404, "deleted item")
}
