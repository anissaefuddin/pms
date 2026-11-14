package httpserver_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/connectors"
	"github.com/anissaefuddin/pms/server/internal/connectors/github"
)

// Admin-only connector routes: lifecycle, conflicts, authorization, and secrets never leaving the server.
func TestConnectorRoutes(t *testing.T) {
	if _, ok := connectors.Default.Get("github"); !ok {
		connectors.Default.Register(github.New())
	}
	f := newFixtureWith(t, 10, func(c *config.Config) { c.ConnectorKey = bytes.Repeat([]byte{7}, 32) })
	admin, regular := f.person("admin"), f.person("regular")
	body := map[string]any{"kind": "github", "name": "GH", "config": map[string]any{"repo": "o/r"}, "secrets": map[string]string{"token": "TOPSECRET-token", "webhook_secret": "TOPSECRET-hook"}}

	f.want(f.call(nil, "GET", "/connectors", nil), 401, "anonymous list")
	f.want(f.call(regular, "GET", "/connectors", nil), 403, "regular list")
	f.want(f.call(regular, "POST", "/connectors", body), 403, "regular install")
	f.want(f.call(admin, "POST", "/connectors", map[string]any{"kind": "nope", "name": "x"}), 422, "unknown kind")
	f.want(f.call(admin, "POST", "/connectors", map[string]any{"kind": "github", "name": "x", "config": map[string]any{"repo": "bad"}, "secrets": map[string]string{"token": "t"}}), 422, "invalid config")
	got := f.call(admin, "POST", "/connectors", body)
	f.want(got, 201, "install")
	id := got.body["id"].(string)
	base := "/connectors/" + id

	f.want(f.call(admin, "PATCH", base, map[string]any{"name": "Renamed"}), 200, "rename")
	f.want(f.call(regular, "PATCH", base, map[string]any{"name": "x"}), 403, "regular patch")
	f.want(f.call(admin, "PATCH", "/connectors/00000000-0000-0000-0000-000000000000", map[string]any{"name": "x"}), 404, "unknown id")
	f.want(f.call(admin, "PATCH", "/connectors/not-a-uuid", map[string]any{"name": "x"}), 404, "bad id")
	f.want(f.call(admin, "POST", base+"/disable", nil), 200, "disable")
	f.want(f.call(admin, "POST", base+"/sync", nil), 409, "sync while disabled")
	f.want(f.call(admin, "POST", base+"/enable", nil), 200, "enable")
	f.want(f.call(regular, "POST", base+"/disable", nil), 403, "regular disable")

	// An open conflict is listed and resolving it is admin-only.
	var item string
	if err := f.pool.QueryRow(t.Context(), `INSERT INTO items(type,title,created_by) SELECT 'task','T',id FROM users WHERE email=(SELECT email FROM users WHERE user_type='admin' LIMIT 1) RETURNING id::text`).Scan(&item); err != nil {
		t.Fatal(err)
	}
	var conflict string
	if err := f.pool.QueryRow(t.Context(), `INSERT INTO connector_conflicts(connector_id,external_id,item_id,item_title,remote_title,remote_version) VALUES ($1,'7',$2,'mine','theirs','v1') RETURNING id::text`, id, item).Scan(&conflict); err != nil {
		t.Fatal(err)
	}
	list := f.call(admin, "GET", base+"/conflicts", nil)
	f.want(list, 200, "conflicts")
	if items, _ := list.body["items"].([]any); len(items) != 1 {
		t.Fatalf("conflicts: %v", list.body)
	}
	f.want(f.call(regular, "GET", base+"/conflicts", nil), 403, "regular conflicts")
	f.want(f.call(regular, "POST", "/connector-conflicts/"+conflict+"/resolve", map[string]any{"resolution": "item"}), 403, "regular resolve")
	f.want(f.call(admin, "POST", "/connector-conflicts/"+conflict+"/resolve", map[string]any{"resolution": "maybe"}), 422, "bad resolution")
	f.want(f.call(admin, "POST", "/connector-conflicts/00000000-0000-0000-0000-000000000000/resolve", map[string]any{"resolution": "item"}), 404, "unknown conflict")

	// Secrets: names only, in every response that carries an install.
	all := f.call(admin, "GET", "/connectors", nil)
	f.want(all, 200, "list")
	raw, _ := json.Marshal(all.body)
	if strings.Contains(string(raw), "TOPSECRET") || !strings.Contains(string(raw), "webhook_secret") {
		t.Fatalf("list must show secret names only: %s", raw)
	}
	raw, _ = json.Marshal(got.body)
	if strings.Contains(string(raw), "TOPSECRET") {
		t.Fatal("install response leaked a secret")
	}
}
