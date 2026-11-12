package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/anissaefuddin/pms/server/internal/config"
)

func TestPanicBecomesProblemJSON(t *testing.T) {
	h := recoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 500 || rec.Header().Get("Content-Type") != "application/problem+json" || !strings.Contains(rec.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("panic not turned into problem+json: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Fatal("panic message leaked to the client")
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	for _, secure := range []bool{false, true} {
		s := &Server{cfg: config.Config{CookieSecure: secure}}
		for _, clear := range []bool{false, true} {
			rec := httptest.NewRecorder()
			if clear {
				s.clearSessionCookie(rec)
			} else {
				s.setSessionCookie(rec, "token", expires)
			}
			cookies := rec.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("secure=%t clear=%t: want one session cookie, got %v", secure, clear, cookies)
			}
			c := cookies[0]
			if c.Name != "pms_session" || c.Path != "/" || !c.HttpOnly || c.Secure != secure || c.SameSite != http.SameSiteLaxMode {
				t.Fatalf("secure=%t clear=%t: incorrect cookie attributes: %+v", secure, clear, c)
			}
			if clear && (c.MaxAge >= 0 || c.Value != "") {
				t.Fatalf("clearing cookie must expire it: %+v", c)
			}
			if !clear && !c.Expires.Equal(expires) {
				t.Fatalf("cookie expiry = %v, want supplied expiry %v", c.Expires, expires)
			}
		}
	}
}

func TestDecodeRejectsOversizedAndMalformedBodies(t *testing.T) {
	var dst struct{ A string }
	for name, body := range map[string]string{
		"over 1 MiB":    `{"A":"` + strings.Repeat("x", 2<<20) + `"}`,
		"unknown field": `{"B":1}`,
		"two documents": `{"A":"x"}{"A":"y"}`,
		"not json":      `nope`,
	} {
		rec := httptest.NewRecorder()
		if decode(rec, httptest.NewRequest("POST", "/x", strings.NewReader(body)), &dst) || rec.Code != 400 {
			t.Fatalf("%s: accepted or wrong status %d", name, rec.Code)
		}
	}
}

func TestStaticCachingAndMissingAssets(t *testing.T) {
	h := staticHandler(fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}, "assets/app-abc.js": {Data: []byte("console.log(1)")}})
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec
	}
	if rec := get("/settings/security"); rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-cache" || !strings.Contains(rec.Body.String(), "app") {
		t.Fatalf("SPA route must serve index.html with no-cache: %d %q %q", rec.Code, rec.Header().Get("Cache-Control"), rec.Body.String())
	}
	if rec := get("/assets/app-abc.js"); rec.Code != 200 || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("hashed asset must be immutable: %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	if rec := get("/assets/old-release.js"); rec.Code != 404 || strings.Contains(rec.Body.String(), "app") {
		t.Fatalf("a missing asset must be a plain 404, not index.html: %d %q", rec.Code, rec.Body.String())
	}
}
