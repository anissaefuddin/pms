package httpserver_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/connectors"
	"github.com/anissaefuddin/pms/server/internal/connectors/github"
)

// The GitHub webhook has no session and no Origin: the signature is the only credential.
func TestGitHubWebhookSignatureAndCSRFExemption(t *testing.T) {
	if _, ok := connectors.Default.Get("github"); !ok {
		connectors.Default.Register(github.New())
	}
	f := newFixtureWith(t, 10, func(c *config.Config) { c.ConnectorKey = bytes.Repeat([]byte{7}, 32) })
	admin := f.person("admin")
	got := f.call(admin, "POST", "/connectors", map[string]any{"kind": "github", "name": "GH", "config": map[string]any{"repo": "o/r"}, "secrets": map[string]string{"token": "t", "webhook_secret": "whsec"}})
	f.want(got, 201, "install")
	id := got.body["id"].(string)
	body := `{"action":"opened"}`
	m := hmac.New(sha256.New, []byte("whsec"))
	m.Write([]byte(body))
	sig := "sha256=" + hex.EncodeToString(m.Sum(nil))
	post := func(path, event, signature string) int {
		req, _ := http.NewRequest("POST", f.srv.URL+path, strings.NewReader(body)) // no cookie, no Origin
		req.Header.Set("X-GitHub-Event", event)
		if signature != "" {
			req.Header.Set("X-Hub-Signature-256", signature)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	path := "/api/v1/connectors/" + id + "/github-webhook"
	if c := post(path, "issues", ""); c != 401 {
		t.Fatalf("unsigned: %d", c)
	}
	if c := post(path, "issues", "sha256="+strings.Repeat("0", 64)); c != 401 {
		t.Fatalf("wrong signature: %d", c)
	}
	if c := post("/api/v1/connectors/00000000-0000-0000-0000-000000000000/github-webhook", "issues", sig); c != 404 {
		t.Fatalf("unknown install: %d", c)
	}
	if c := post(path, "issues", sig); c != 204 {
		t.Fatalf("signed: %d", c)
	}
	var due bool
	if err := f.pool.QueryRow(t.Context(), `SELECT next_sync_at<=now() FROM connector_installs WHERE id=$1`, id).Scan(&due); err != nil || !due {
		t.Fatalf("signed issues event must make the install due: %v %v", due, err)
	}
	// Another route on the same prefix still needs Origin/CSRF.
	req, _ := http.NewRequest("POST", f.srv.URL+"/api/v1/connectors/"+id+"/sync", nil)
	req.AddCookie(admin.cookie)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("CSRF must still protect other connector routes: %d", resp.StatusCode)
	}
}
