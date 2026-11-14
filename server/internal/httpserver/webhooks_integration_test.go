package httpserver_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/twofactor"
	"github.com/anissaefuddin/pms/server/internal/webhooks"
)

type hookCall struct {
	header http.Header
	body   []byte
}

// receiver records what it gets; status decides the answer.
type receiver struct {
	srv    *httptest.Server
	mu     sync.Mutex
	calls  []hookCall
	status int
}

func newReceiver(t *testing.T, status int) *receiver {
	r := &receiver{status: status}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.calls = append(r.calls, hookCall{req.Header.Clone(), b})
		code := r.status
		r.mu.Unlock()
		w.WriteHeader(code)
	}))
	t.Cleanup(r.srv.Close)
	return r
}

func (r *receiver) got() []hookCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]hookCall(nil), r.calls...)
}

func (r *receiver) answer(status int) { r.mu.Lock(); r.status = status; r.mu.Unlock() }

func webhookEnv(t *testing.T) *autoEnv {
	e := newAutoEnv(t)
	svc := webhooks.New(e.f.pool).WithKeys(twofactor.Keys{Current: bytes.Repeat([]byte{7}, 32)})
	e.engine.Hook = svc.Handler
	e.engine.Tick = func(ctx context.Context) error { _, err := svc.Send(ctx); return err }
	old, oldBackoff := webhooks.AllowInternal, webhooks.Backoff
	t.Cleanup(func() { webhooks.AllowInternal, webhooks.Backoff = old, oldBackoff })
	return e
}

func (e *autoEnv) subscribe(t *testing.T, p *person, url string, types ...string) map[string]any {
	got := e.f.call(p, "POST", "/webhooks", map[string]any{"name": "hook", "url": url, "event_types": types})
	e.f.want(got, 201, "create webhook")
	return got.body
}

func TestWebhookRejectsInternalTargets(t *testing.T) {
	e := webhookEnv(t)
	webhooks.AllowInternal = false
	for _, u := range []string{"http://example.com/h", "https://127.0.0.1/h", "https://localhost/h", "https://169.254.169.254/latest/meta-data", "https://10.0.0.5/h", "https://[::1]/h", "https://user:pw@example.com/h", "https://metadata.google.internal/h"} {
		e.f.want(e.f.call(e.admin, "POST", "/webhooks", map[string]any{"name": "x", "url": u, "event_types": []string{"item.created"}}), 422, "ssrf "+u)
	}
	e.f.want(e.f.call(e.admin, "POST", "/webhooks", map[string]any{"name": "x", "url": "https://example.com/h", "event_types": []string{"nope"}}), 422, "unknown event type")
}

// E2E-PAR-11: signed delivery, secret shown once, only events the owner may see, owner-only management.
func TestWebhookSignedDeliveryAndVisibility(t *testing.T) {
	e := webhookEnv(t)
	webhooks.AllowInternal = true
	rcv := newReceiver(t, 200)
	outsider := e.f.person("regular") // no access to the Space
	mine := e.subscribe(t, e.editor, rcv.srv.URL, "item.created")
	secret, _ := mine["secret"].(string)
	if secret == "" {
		t.Fatal("create must return the secret")
	}
	hidden := e.subscribe(t, outsider, rcv.srv.URL, "item.created")
	list := e.f.call(e.editor, "GET", "/webhooks", nil)
	e.f.want(list, 200, "list")
	if _, leaked := list.body["items"].([]any)[0].(map[string]any)["secret"]; leaked {
		t.Fatal("secret must not be listed")
	}
	e.f.want(e.f.call(outsider, "PATCH", "/webhooks/"+mine["id"].(string), map[string]any{"enabled": false}), 404, "other user cannot edit")
	e.f.want(e.f.call(outsider, "GET", "/webhooks/"+mine["id"].(string)+"/deliveries", nil), 404, "other user cannot read log")
	e.f.want(e.f.call(outsider, "DELETE", "/webhooks/not-a-uuid", nil), 404, "bad id")

	e.item(t, e.admin, "Secret plan", nil)
	e.drain(t)
	calls := rcv.got()
	if len(calls) != 1 {
		t.Fatalf("exactly the subscriber who may see the item gets it, got %d calls", len(calls))
	}
	c := calls[0]
	ts, _ := strconv.ParseInt(c.header.Get("X-PMS-Timestamp"), 10, 64)
	if want := webhooks.Sign(secret, ts, c.body); c.header.Get("X-PMS-Signature") != want {
		t.Fatalf("bad signature %q want %q", c.header.Get("X-PMS-Signature"), want)
	}
	if webhooks.Sign("whsec_wrong", ts, c.body) == c.header.Get("X-PMS-Signature") {
		t.Fatal("signature must depend on the secret")
	}
	if d := time.Since(time.Unix(ts, 0)); d > time.Minute || d < -time.Minute || c.header.Get("X-PMS-Event") != "item.created" || c.header.Get("X-PMS-Delivery") == "" {
		t.Fatalf("headers: %v", c.header)
	}
	log := e.f.call(e.editor, "GET", "/webhooks/"+mine["id"].(string)+"/deliveries", nil)
	if items := log.body["items"].([]any); len(items) != 1 || items[0].(map[string]any)["status"] != "success" {
		t.Fatalf("log: %v", log.body)
	}
	if n := e.f.queryID("SELECT count(*)::text FROM webhook_deliveries WHERE subscription_id=$1", hidden["id"]); n != "0" {
		t.Fatalf("outsider got %s deliveries", n)
	}
	// Delivering the same event again does not send twice.
	e.f.exec("UPDATE domain_events SET dispatched_at=NULL")
	e.drain(t)
	if n := len(rcv.got()); n != 1 {
		t.Fatalf("redelivered event sent %d times", n)
	}
}

func TestWebhookRetryThenAutoDisable(t *testing.T) {
	e := webhookEnv(t)
	webhooks.AllowInternal = true
	webhooks.Backoff = func(int) time.Duration { return 0 }
	rcv := newReceiver(t, 500)
	sub := e.subscribe(t, e.editor, rcv.srv.URL, "item.created")
	id := sub["id"].(string)

	// Retry: first attempt fails, a later one succeeds, same event id on both.
	e.item(t, e.admin, "One", nil)
	e.drain(t)
	if n := len(rcv.got()); n < 1 {
		t.Fatal("no first attempt")
	}
	if st := e.f.queryID("SELECT status FROM webhook_deliveries WHERE subscription_id=$1", id); st != "pending" {
		t.Fatalf("after a failure the delivery waits for retry, got %s", st)
	}
	rcv.answer(200)
	e.drain(t)
	calls := rcv.got()
	if st := e.f.queryID("SELECT status FROM webhook_deliveries WHERE subscription_id=$1", id); st != "success" || calls[0].header.Get("X-PMS-Delivery") != calls[len(calls)-1].header.Get("X-PMS-Delivery") {
		t.Fatalf("retry should succeed with the same delivery id, status %s", st)
	}

	// Backoff really delays: with a long backoff a failed delivery is not tried again at once.
	webhooks.Backoff = func(int) time.Duration { return time.Hour }
	rcv.answer(500)
	before := len(rcv.got())
	e.item(t, e.admin, "Two", nil)
	e.drain(t)
	e.drain(t)
	if len(rcv.got()) != before+1 {
		t.Fatalf("backoff ignored: %d calls", len(rcv.got())-before)
	}

	// Auto-disable: 10 failed attempts in a row switch it off and nothing more is sent.
	webhooks.Backoff = func(int) time.Duration { return 0 }
	for i := range 3 {
		e.item(t, e.admin, "Fail "+strconv.Itoa(i), nil)
	}
	for range 12 {
		e.drain(t)
	}
	got := e.f.call(e.editor, "GET", "/webhooks", nil)
	s := got.body["items"].([]any)[0].(map[string]any)
	if s["enabled"] != false || s["disabled_reason"] == nil {
		t.Fatalf("should be disabled: %v", s)
	}
	n := len(rcv.got())
	e.item(t, e.admin, "After", nil)
	e.drain(t)
	if len(rcv.got()) != n {
		t.Fatal("disabled subscription still receives")
	}
	// Re-enabling resets the counter.
	rcv.answer(200)
	on := e.f.call(e.editor, "PATCH", "/webhooks/"+id, map[string]any{"enabled": true})
	e.f.want(on, 200, "re-enable")
	if on.body["consecutive_failures"].(float64) != 0 {
		t.Fatal("counter not reset")
	}
}

// With a key configured the signing secret is stored encrypted, deliveries are still signed with the plain secret, and a
// lost key fails the attempt instead of sending anything unsigned.
func TestWebhookSecretIsEncryptedAtRest(t *testing.T) {
	e := newAutoEnv(t)
	keys := twofactor.Keys{Current: bytes.Repeat([]byte{9}, 32)}
	svc := webhooks.New(e.f.pool).WithKeys(keys)
	e.engine.Hook = svc.Handler
	e.engine.Tick = func(ctx context.Context) error { _, err := svc.Send(ctx); return err }
	old, oldBackoff := webhooks.AllowInternal, webhooks.Backoff
	t.Cleanup(func() { webhooks.AllowInternal, webhooks.Backoff = old, oldBackoff })
	webhooks.AllowInternal = true
	rcv := newReceiver(t, 200)

	sub, err := svc.Create(context.Background(), e.editor.id, "hook", rcv.srv.URL, []string{"item.created"})
	if err != nil || sub.Secret == "" {
		t.Fatalf("create: %v %+v", err, sub)
	}
	stored := e.f.queryID("SELECT secret FROM webhook_subscriptions WHERE id=$1", sub.ID)
	if !strings.HasPrefix(stored, "v1:") || strings.Contains(stored, sub.Secret) {
		t.Fatalf("secret must be stored encrypted, got %q", stored)
	}
	e.item(t, e.admin, "Encrypted secret plan", nil)
	e.drain(t)
	calls := rcv.got()
	if len(calls) != 1 {
		t.Fatalf("want one delivery, got %d", len(calls))
	}
	ts, _ := strconv.ParseInt(calls[0].header.Get("X-PMS-Timestamp"), 10, 64)
	if calls[0].header.Get("X-PMS-Signature") != webhooks.Sign(sub.Secret, ts, calls[0].body) {
		t.Fatal("the delivery must be signed with the decrypted secret")
	}

	// A row copied to another subscription id does not decrypt, and a different key fails the attempt without sending.
	wrong := webhooks.New(e.f.pool).WithKeys(twofactor.Keys{Current: bytes.Repeat([]byte{1}, 32)})
	e.engine.Tick = func(ctx context.Context) error { _, err := wrong.Send(ctx); return err }
	e.item(t, e.admin, "Second plan", nil)
	e.drain(t)
	if n := len(rcv.got()); n != 1 {
		t.Fatalf("with the wrong key nothing may be sent, got %d deliveries", n)
	}
	if msg := e.f.queryID("SELECT COALESCE(last_error,'') FROM webhook_deliveries WHERE subscription_id=$1 AND attempts>0 AND status<>'success' ORDER BY created_at DESC LIMIT 1", sub.ID); !strings.Contains(msg, "cannot be decrypted") {
		t.Fatalf("the failed attempt must say why, got %q", msg)
	}
}
