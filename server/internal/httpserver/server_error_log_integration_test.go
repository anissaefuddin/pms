package httpserver_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/reqid"
)

// DEF-030 with a real database failure, on a path whose handler logs nothing itself.
func TestDatabaseFailureIsLoggedWithTheRequestID(t *testing.T) {
	f := newFixture(t)
	member := f.person("regular")
	var out bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(reqid.Handler(slog.NewJSONHandler(&out, nil))))
	t.Cleanup(func() { slog.SetDefault(previous) })
	f.exec("ALTER TABLE user_totp RENAME TO user_totp_fault")
	got := f.call(member, "GET", "/me/2fa", nil, "X-Request-ID", "fault-1")
	f.exec("ALTER TABLE user_totp_fault RENAME TO user_totp")
	f.want(got, 500, "injected fault")
	if body := strings.ToLower(fmt.Sprint(got.body)); strings.Contains(body, "user_totp") || strings.Contains(body, "sqlstate") {
		t.Fatalf("internal error leaked: %s", body)
	}
	found := 0
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.Contains(line, `"level":"ERROR"`) && strings.Contains(line, `"request_id":"fault-1"`) {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("ERROR lines: %d\n%s", found, out.String())
	}
}
