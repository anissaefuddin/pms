package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
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

// S6.3.3 (OWASP A01, IDOR): with real ids of a private Space, its project, form, view and custom field, a signed-in
// user who has no share must be refused by every operation that takes those ids: reads answer 404 and nothing succeeds.
func TestOutsiderCannotTouchAnotherSpacesResourcesThroughAnyEndpoint(t *testing.T) {
	f := newFixture(t)
	owner, outsider := f.person("admin"), f.person("regular")
	made := f.call(owner, "POST", "/space-templates/project-management/spaces", map[string]any{"request_id": uuid.NewString()})
	if made.status != 201 {
		t.Fatalf("template: %+v", made)
	}
	space := made.body["id"].(string)
	ids := map[string]string{
		"spaceId": space,
		"itemId":  f.queryID("SELECT i.id::text FROM items i JOIN item_locations l ON l.item_id=i.id WHERE l.space_id=$1 AND i.type='task' LIMIT 1", space),
		"formId":  f.queryID("SELECT id::text FROM request_forms WHERE space_id=$1", space),
		"fieldId": f.queryID("SELECT id::text FROM custom_fields WHERE space_id=$1", space),
		"viewId":  f.queryID("SELECT id::text FROM views WHERE scope_space_id=$1 LIMIT 1", space),
	}
	if f.call(owner, "GET", "/spaces/"+space, nil).status != 200 {
		t.Fatal("owner lost access to own Space")
	}
	protected, _ := contractOps(t)
	checked := 0
	for _, op := range protected {
		params := pathParam.FindAllString(op.path, -1)
		path, resolvable := op.path, len(params) > 0
		for _, p := range params {
			id, ok := ids[strings.Trim(p, "{}")]
			if !ok {
				resolvable = false
				break
			}
			path = strings.Replace(path, p, id, 1)
		}
		if !resolvable {
			continue
		}
		checked++
		got := f.call(outsider, op.method, path, map[string]any{})
		switch {
		case got.status >= 500:
			t.Errorf("outsider %s %s answered %d", op.method, op.path, got.status)
		case op.method == "GET" && got.status != 404:
			t.Errorf("outsider GET %s answered %d, want 404", op.path, got.status)
		case op.method == "DELETE" && strings.HasSuffix(op.path, "/star"): // D-51: removing one's own bookmark needs no access
		case got.status == 200 || got.status == 201 || got.status == 204:
			t.Errorf("outsider %s %s succeeded with %d", op.method, op.path, got.status)
		}
	}
	if checked < 20 {
		t.Fatalf("only %d operations were exercised; the sweep is not covering the contract", checked)
	}
	t.Logf("%d operations on another Space's ids refused", checked)
}
