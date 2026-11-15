package httpserver

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/connectors"
)

// RT-35: a provider that rejects the token must give the admin a problem+json 502 that names the reason.
func TestRT35ProviderRejectionIsStructured(t *testing.T) {
	rec := httptest.NewRecorder()
	writeConnectorError(rec, connectors.HTTPError{Status: 401})
	body := rec.Body.String()
	if rec.Code != 502 || rec.Header().Get("Content-Type") != "application/problem+json" ||
		!strings.Contains(body, `"code":"provider_error"`) || !strings.Contains(body, "HTTP 401") {
		t.Fatalf("got %d %s", rec.Code, body)
	}
	rec = httptest.NewRecorder()
	writeConnectorError(rec, errString("secret db detail"))
	if strings.Contains(rec.Body.String(), "secret db detail") {
		t.Fatal("unknown errors must stay generic")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
