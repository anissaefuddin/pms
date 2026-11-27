package httpserver_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/webhooks"
)

// B3-13: a queued webhook payload is not sent after its subscription owner loses access to the item it names. Access is checked
// when the job runs, not only when the event was queued.
func TestQueuedWebhookIsNotSentAfterAccessEnds(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	client := f.createClient(admin, "WH-A")["id"].(string)
	f.addMember(admin, client, admin)
	f.addMember(admin, client, member)
	space := f.scopedSpace("WH client", "client", client)
	f.share(space, "user", admin.id, "full")
	f.share(space, "user", member.id, "read")
	item := f.itemIn(space, "client", client, admin.id, "Hook item")
	hook := f.call(member, http.MethodPost, "/webhooks", map[string]any{
		"name": "client hook", "url": "https://example.com/hook", "event_types": []string{"item.created"},
	})
	f.want(hook, http.StatusCreated, "create subscription")
	sub := hook.body["id"].(string)
	f.exec(`INSERT INTO webhook_deliveries(id,subscription_id,event_id,event_type,payload)
VALUES (gen_random_uuid(),$1,gen_random_uuid(),'item.created',$2::jsonb)`, sub, fmt.Sprintf(`{"item_id":"%s"}`, item))

	f.revokeMember(admin, client, member)
	svc := webhooks.New(f.pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := svc.Send(ctx); err != nil {
		t.Fatal(err)
	}
	var status, last string
	f.pool.QueryRow(ctx, "SELECT status, coalesce(last_error,'') FROM webhook_deliveries WHERE subscription_id=$1", sub).Scan(&status, &last)
	if status != "failed" || last == "" {
		t.Fatalf("a payload whose owner lost access must be blocked, got status %q error %q", status, last)
	}
}
