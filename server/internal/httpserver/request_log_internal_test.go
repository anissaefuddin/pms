package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/reqid"
	"github.com/prometheus/client_golang/prometheus"
)

// DEF-013: the application log line and the access log line of one request carry the same id, also for an error written by a shared
// error writer and for a panic, and the access log still knows the route.
func TestApplicationAndAccessLogShareTheRequestID(t *testing.T) {
	var out bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(reqid.Handler(slog.NewJSONHandler(&out, nil))))
	t.Cleanup(func() { slog.SetDefault(old) })
	s := &Server{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "r"}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "d"}, []string{"method", "route"}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /fail", func(w http.ResponseWriter, _ *http.Request) { writeItemError(w, errors.New("boom")) })
	mux.HandleFunc("GET /panic", func(http.ResponseWriter, *http.Request) { panic("boom") })
	for _, path := range []string{"/fail", "/panic"} {
		out.Reset()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("X-Request-ID", "trace-1")
		s.observe(s.requestID(recoverPanic(mux))).ServeHTTP(httptest.NewRecorder(), req)
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("%s: %s", path, out.String())
		}
		for _, line := range lines {
			var entry map[string]any
			if err := json.Unmarshal([]byte(line), &entry); err != nil || entry["request_id"] != "trace-1" {
				t.Fatalf("%s: %s", path, line)
			}
			if entry["msg"] == "request" && entry["route"] != "GET "+path {
				t.Fatalf("the access log lost the route: %s", line)
			}
		}
	}
}
