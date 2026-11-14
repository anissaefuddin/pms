package httpserver_test

import (
	"bytes"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/connectors"
	"github.com/anissaefuddin/pms/server/internal/connectors/github"
	"github.com/anissaefuddin/pms/server/internal/twofactor"
	"github.com/anissaefuddin/pms/server/internal/webhooks"
)

func problemCode(r reply) any { return r.body["code"] }

// Without PMS_CONNECTOR_KEY a secret is refused with key_not_configured and nothing is stored; the operations that store no secret still work.
func TestSecretsRefusedWithoutConnectorKey(t *testing.T) {
	if _, ok := connectors.Default.Get("github"); !ok {
		connectors.Default.Register(github.New())
	}
	old := webhooks.AllowInternal
	t.Cleanup(func() { webhooks.AllowInternal = old })
	webhooks.AllowInternal = true
	f := newFixtureWith(t, 10, func(c *config.Config) { c.ConnectorKey = nil })
	admin := f.person("admin")

	hook := f.call(admin, "POST", "/webhooks", map[string]any{"name": "h", "url": "https://example.com/h", "event_types": []string{"item.created"}})
	f.want(hook, 422, "webhook without key")
	if problemCode(hook) != "key_not_configured" || f.queryID("SELECT count(*)::text FROM webhook_subscriptions") != "0" {
		t.Fatalf("webhook: %v", hook.body)
	}
	conn := f.call(admin, "POST", "/connectors", map[string]any{"kind": "github", "name": "GH", "config": map[string]any{"repo": "o/r"}, "secrets": map[string]string{"token": "t"}})
	f.want(conn, 422, "connector secrets without key")
	if problemCode(conn) != "key_not_configured" || f.queryID("SELECT count(*)::text FROM connector_installs") != "0" {
		t.Fatalf("connector: %v", conn.body)
	}
	f.want(f.call(admin, "GET", "/webhooks", nil), 200, "list needs no key")
	f.want(f.call(admin, "GET", "/connectors", nil), 200, "list connectors needs no key")
}

// With a key a webhook is created and its secret is not stored in clear; after a rotation (new key, old as PREVIOUS) it still reads,
// and a service that has neither key cannot.
func TestConnectorKeyRotation(t *testing.T) {
	old := webhooks.AllowInternal
	t.Cleanup(func() { webhooks.AllowInternal = old })
	webhooks.AllowInternal = true
	f := newFixture(t) // ConnectorKey = 7s
	admin := f.person("admin")
	hook := f.call(admin, "POST", "/webhooks", map[string]any{"name": "h", "url": "https://example.com/h", "event_types": []string{"item.created"}})
	f.want(hook, 201, "create with key")
	id, secret := hook.body["id"].(string), hook.body["secret"].(string)
	stored := f.queryID("SELECT secret FROM webhook_subscriptions WHERE id=$1", id)
	if stored == secret || len(stored) < 3 || stored[:3] != "v1:" {
		t.Fatalf("stored = %q", stored)
	}
	oldKey, newKey := bytes.Repeat([]byte{7}, 32), bytes.Repeat([]byte{8}, 32)
	read := func(k twofactor.Keys) (string, error) {
		plain, _, err := k.Decrypt(stored, id)
		return plain, err
	}
	if got, err := read(twofactor.Keys{Current: newKey, Previous: oldKey}); err != nil || got != secret {
		t.Fatalf("rotated read = %q, %v", got, err)
	}
	if _, err := read(twofactor.Keys{Current: newKey}); err == nil {
		t.Fatal("the old key must be needed until the secret is re-saved")
	}
}
