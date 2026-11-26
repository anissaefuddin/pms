package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type contractOp struct{ method, path string }

// contractOps lists every operation in api/openapi.yaml; public marks the ones declared `security: []`.
func contractOps(t *testing.T) (protected, public []contractOp) {
	t.Helper()
	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for path, methods := range doc.Paths {
		for method, node := range methods {
			if !strings.Contains("get put post patch delete", method) || node.Kind != yaml.MappingNode {
				continue
			}
			op := contractOp{strings.ToUpper(method), path}
			open := false
			for i := 0; i+1 < len(node.Content); i += 2 {
				if node.Content[i].Value == "security" && node.Content[i+1].Kind == yaml.SequenceNode && len(node.Content[i+1].Content) == 0 {
					open = true
				}
			}
			if open {
				public = append(public, op)
			} else {
				protected = append(protected, op)
			}
		}
	}
	return protected, public
}

var pathParam = regexp.MustCompile(`\{[^}]+\}`)

// S6.3.3 (OWASP A01/A07): every operation the contract does not declare public refuses a caller without a session,
// and an ordinary signed-in user who knows no ids gets a refusal or "not found", never data and never a 5xx.
func TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds(t *testing.T) {
	f := newFixture(t)
	user := f.person("regular")
	protected, public := contractOps(t)
	if len(protected) < 100 || len(public) < 5 {
		t.Fatalf("contract parsed wrongly: %d protected, %d public", len(protected), len(public))
	}
	for _, op := range protected {
		path := pathParam.ReplaceAllStringFunc(op.path, func(string) string { return uuid.NewString() })
		if got := f.call(nil, op.method, path, map[string]any{}); got.status != 401 {
			t.Errorf("anonymous %s %s answered %d, want 401", op.method, op.path, got.status)
		}
		got := f.call(user, op.method, path, map[string]any{})
		switch {
		case got.status >= 500:
			t.Errorf("%s %s with unknown ids answered %d", op.method, op.path, got.status)
		case op.path == "/auth/logout": // ends the caller's own session whatever it holds
		case strings.Contains(op.path, "{"):
			// An operation on a resource id must never succeed when no such resource exists.
			if got.status == 200 || got.status == 201 || got.status == 204 {
				if !strings.Contains(op.path, "/star") {
					t.Errorf("%s %s on unknown ids answered %d", op.method, op.path, got.status)
				}
			}
		}
	}
}

// A request the browser abandoned must not be reported as a server error.
func TestAbandonedRequestIsNotAServerError(t *testing.T) {
	f := newFixture(t)
	user := f.person("regular")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest("GET", "/api/v1/me", nil).WithContext(ctx)
	req.AddCookie(user.cookie)
	rec := httptest.NewRecorder()
	f.srv.Config.Handler.ServeHTTP(rec, req)
	if rec.Code >= 500 {
		t.Fatalf("abandoned request answered %d: %s", rec.Code, rec.Body.String())
	}
	_ = http.StatusOK
}

func TestItemActivityAndFollowers(t *testing.T) {
	f := newFixture(t)
	admin, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Hist")
	f.share(space, "user", admin.id, "full") // A1.2: the admin creates this Space's content as an explicit member
	f.share(space, "user", reader.id, "read")
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','History',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", item, space)
	for i := range 3 {
		f.exec("INSERT INTO activity(item_id,actor_id,action,changes) VALUES ($1,$2,'item.updated',$3::jsonb)", item, admin.id, `{"title":{"from":"a","to":"b`+string(rune('0'+i))+`"}}`)
	}
	got := f.call(reader, "GET", "/items/"+item+"/activity?limit=2", nil)
	if got.status != 200 || len(got.items()) != 2 || got.body["next_cursor"] == nil {
		t.Fatalf("first page: %+v", got)
	}
	if first := got.items()[0]; first["actor"].(map[string]any)["id"] != admin.id || first["changes"] == nil {
		t.Fatalf("entry shape: %+v", first)
	}
	rest := f.call(reader, "GET", "/items/"+item+"/activity?cursor="+got.body["next_cursor"].(string), nil)
	if rest.status != 200 || len(rest.items()) != 1 || rest.body["next_cursor"] != nil {
		t.Fatalf("second page: %+v", rest)
	}
	for _, bad := range []string{"limit=0", "limit=101", "cursor=x"} {
		if r := f.call(reader, "GET", "/items/"+item+"/activity?"+bad, nil); r.status != 422 {
			t.Fatalf("%s: %+v", bad, r)
		}
	}
	if r := f.call(outsider, "GET", "/items/"+item+"/activity", nil); r.status != 404 {
		t.Fatalf("outsider history: %+v", r)
	}
	body := map[string]any{"user_ids": []string{reader.id, admin.id}}
	if r := f.call(reader, "PUT", "/items/"+item+"/followers", body); r.status != 403 {
		t.Fatalf("read-only follower change: %+v", r)
	}
	if r := f.call(outsider, "PUT", "/items/"+item+"/followers", body); r.status != 404 {
		t.Fatalf("outsider follower change: %+v", r)
	}
	if r := f.call(admin, "PUT", "/items/"+item+"/followers", map[string]any{"user_ids": []string{uuid.NewString()}}); r.status != 422 {
		t.Fatalf("unknown follower: %+v", r)
	}
	if r := f.call(admin, "PUT", "/items/"+item+"/followers", body); r.status != 204 {
		t.Fatalf("set followers: %+v", r)
	}
	if r := f.call(admin, "PUT", "/items/"+item+"/followers", map[string]any{"user_ids": []string{reader.id}}); r.status != 204 {
		t.Fatalf("replace followers: %+v", r)
	}
	if n := f.queryID("SELECT count(*)::text FROM item_followers WHERE item_id=$1 AND user_id=$2", item, reader.id); n != "1" {
		t.Fatalf("follower missing: %s", n)
	}
	if n := f.queryID("SELECT count(*)::text FROM item_followers WHERE item_id=$1", item); n != "1" {
		t.Fatalf("replace left %s followers", n)
	}
}

// idorFixture builds a private Space with one of each resource kind the contract addresses by id.
func idorFixture(t *testing.T, f *fixture, owner *person) map[string]string {
	t.Helper()
	made := f.call(owner, "POST", "/space-templates/project-management/spaces", map[string]any{"request_id": uuid.NewString()})
	f.want(made, 201, "template")
	space := made.body["id"].(string)
	board := f.call(owner, "POST", "/dashboards", map[string]any{"space_id": space, "name": "Private board"})
	f.want(board, 201, "dashboard")
	dashboard := board.body["id"].(string)
	widget := f.call(owner, "POST", "/dashboards/"+dashboard+"/widgets", map[string]any{"name": "W", "type": "pie", "position": 0, "config": map[string]any{}}, "If-Match", `"1"`)
	f.want(widget, 201, "widget")
	report := f.call(owner, "POST", "/reports", map[string]any{"name": "Private report", "template": "overdue"})
	f.want(report, 201, "report")
	ids := map[string]string{
		"spaceId":     space,
		"itemId":      f.queryID("SELECT i.id::text FROM items i JOIN item_locations l ON l.item_id=i.id WHERE l.space_id=$1 AND i.type='task' LIMIT 1", space),
		"projectId":   f.queryID("SELECT i.id::text FROM items i JOIN item_locations l ON l.item_id=i.id WHERE l.space_id=$1 AND i.type='project' LIMIT 1", space),
		"formId":      f.queryID("SELECT id::text FROM request_forms WHERE space_id=$1", space),
		"fieldId":     f.queryID("SELECT id::text FROM custom_fields WHERE space_id=$1", space),
		"viewId":      f.queryID("SELECT id::text FROM views WHERE scope_space_id=$1 LIMIT 1", space),
		"dashboardId": dashboard,
		"widgetId":    widget.body["widgets"].([]any)[0].(map[string]any)["id"].(string),
		"reportId":    report.body["id"].(string),
	}
	ids["commentId"] = f.queryID("INSERT INTO comments(id,item_id,author_id,body,body_text) VALUES (gen_random_uuid(),$1,$2,'{}','c') RETURNING id::text", ids["itemId"], owner.id)
	ids["attachmentId"] = f.queryID(`INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256)
VALUES (gen_random_uuid(),$1,$2,'a.bin','application/octet-stream',1,'aa/bb/aabbccdd-1111-4222-8333-444455556666',decode(repeat('00',32),'hex')) RETURNING id::text`, ids["itemId"], owner.id)
	ids["notificationId"] = f.queryID("INSERT INTO notifications(id,recipient_id,kind,item_id) VALUES (gen_random_uuid(),$1,'mention',$2) RETURNING id::text", owner.id, ids["itemId"])
	approval := f.call(owner, "POST", "/items/"+ids["itemId"]+"/approvals", map[string]any{"approver_ids": []string{owner.id}})
	f.want(approval, 201, "approval")
	ids["approvalId"] = approval.body["id"].(string)
	entry := f.call(owner, "POST", "/time-entries", map[string]any{"item_id": ids["itemId"], "date": "2026-11-13", "minutes": 30, "category": "Delivery", "billing_type": "billable"})
	f.want(entry, 201, "time entry")
	ids["entryId"] = entry.body["id"].(string)
	second := f.call(owner, "POST", "/items", map[string]any{"type": "task", "title": "Second", "locations": []map[string]any{{"space_id": space}}})
	f.want(second, 201, "second item")
	dep := f.call(owner, "POST", "/items/"+second.body["id"].(string)+"/dependencies", map[string]any{"predecessor_id": ids["itemId"], "type": "FS"})
	f.want(dep, 201, "dependency")
	ids["dependencyId"] = dep.body["id"].(string)
	blueprint := f.call(owner, "POST", "/spaces/"+space+"/blueprints", map[string]any{"name": "Plan", "source_item_id": ids["projectId"]})
	f.want(blueprint, 201, "blueprint")
	ids["blueprintId"] = blueprint.body["id"].(string)
	ids["groupId"] = f.group("Private group", owner)
	ids["userId"] = owner.id
	ids["workflowId"] = f.queryID("SELECT id::text FROM workflows WHERE is_default")
	ids["statusId"] = f.queryID("SELECT id::text FROM statuses WHERE workflow_id=$1 ORDER BY position LIMIT 1", ids["workflowId"])
	return ids
}

// resolve fills the path parameters from ids; ok is false when some parameter has no real id.
func resolve(path string, ids map[string]string) (string, bool) {
	ok := true
	out := pathParam.ReplaceAllStringFunc(path, func(p string) string {
		id, found := ids[strings.Trim(p, "{}")]
		ok = ok && found
		return id
	})
	return out, ok
}

// S7.1.3 (STAB-04, OWASP A01/IDOR): with real ids of a private Space, a signed-in user with no share is refused by
// every operation that takes those ids (reads 404, nothing succeeds), and a user with only the read role cannot write.
// The log lists the operations that could not be exercised with a real id, which is the residual-risk inventory.
func TestStrangersAndReadersCannotTouchAnotherSpacesResourcesThroughAnyEndpoint(t *testing.T) {
	f := newFixture(t)
	owner, outsider, reader := f.person("admin"), f.person("regular"), f.person("regular")
	ids := idorFixture(t, f, owner)
	f.share(ids["spaceId"], "user", reader.id, "read")
	protected, _ := contractOps(t)
	covered, uncovered := 0, []string{}
	for _, op := range protected {
		path, ok := resolve(op.path, ids)
		if !strings.Contains(op.path, "{") {
			continue
		}
		if !ok {
			uncovered = append(uncovered, op.method+" "+op.path)
			continue
		}
		covered++
		got := f.call(outsider, op.method, path, map[string]any{})
		switch {
		case got.status >= 500:
			t.Errorf("outsider %s %s answered %d", op.method, op.path, got.status)
		case op.method == "GET" && op.path == "/users/{userId}": // the directory is open to regular users by design; external users are checked below
		case op.method == "GET" && op.path == "/users/{userId}/capacity": // private by design (S10.1.1): users are listed openly, so 403 leaks nothing
			if got.status != 403 {
				t.Errorf("outsider GET %s answered %d, want 403", op.path, got.status)
			}
		case op.method == "GET" && (op.path == "/custom-fields/{fieldId}/impact" || op.path == "/custom-fields/{fieldId}/grants"):
			// Workspace configuration is refused before the field is looked up, so an unknown id answers the same and nothing leaks.
			unknown, _ := resolve(op.path, map[string]string{"fieldId": "00000000-0000-4000-8000-00000000dead"})
			if got.status != 403 || f.call(outsider, op.method, unknown, nil).status != 403 {
				t.Errorf("outsider GET %s answered %d, want 403 for a known and an unknown id alike", op.path, got.status)
			}
		case op.method == "GET" && got.status != 404:
			t.Errorf("outsider GET %s answered %d, want 404", op.path, got.status)
		case op.method == "DELETE" && strings.HasSuffix(op.path, "/star"): // D-51: removing one's own bookmark needs no access
		case got.status == 200 || got.status == 201 || got.status == 204:
			t.Errorf("outsider %s %s succeeded with %d", op.method, op.path, got.status)
		}
		if op.method == "GET" || strings.HasSuffix(op.path, "/star") || strings.HasSuffix(op.path, "/duplicate") || strings.HasSuffix(op.path, "/result") || strings.Contains(op.path, "/data") {
			continue // reading and copying are what the read role allows
		}
		if strings.HasPrefix(op.path, "/me/view-") {
			continue // a reader may remember their own layout (expansion, hidden tabs); the view must still be visible, see view_prefs.go
		}
		got = f.call(reader, op.method, path, map[string]any{})
		if got.status >= 500 || got.status == 200 || got.status == 201 || got.status == 204 {
			t.Errorf("read-only %s %s answered %d", op.method, op.path, got.status)
		}
	}
	external := f.person("external")
	f.want(f.call(external, "GET", "/users/"+owner.id, nil), 404, "an external user cannot browse the directory")
	if covered < 60 {
		t.Fatalf("only %d operations were exercised; the sweep is not covering the contract", covered)
	}
	sort.Strings(uncovered)
	t.Logf("%d operations exercised with real ids; %d have no real id in the fixture:\n%s", covered, len(uncovered), strings.Join(uncovered, "\n"))
}

// Hostile public-form tokens are "not found", never a server error (links do not expire by design; rotation and revocation are tested in forms_public_integration_test.go).
func TestHostilePublicTokensAreNotFound(t *testing.T) {
	f := newFixture(t)
	for _, token := range []string{"x", strings.Repeat("a", 5000), "%27%20OR%201=1--", "..%2f..%2fetc", "00000000-0000-0000-0000-000000000000"} {
		for _, call := range []struct{ method, suffix string }{{"GET", ""}, {"POST", "/submissions"}} {
			if got := f.call(nil, call.method, "/public/forms/"+token+call.suffix, map[string]any{}); got.status != 404 && got.status != 414 {
				t.Errorf("%s token %.20q answered %d", call.method, token, got.status)
			}
		}
	}
}
