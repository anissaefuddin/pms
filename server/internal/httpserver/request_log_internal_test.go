package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// DEF-030: every 5xx leaves exactly one ERROR line with the request's id, whether or not the handler logged; 4xx leave none;
// nothing the request carried is written.
func TestEveryServerErrorIsLoggedOnce(t *testing.T) {
	var mu sync.Mutex
	var out bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(reqid.Handler(slog.NewJSONHandler(lockedWriter{&mu, &out}, nil))))
	t.Cleanup(func() { slog.SetDefault(old) })
	s := &Server{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "r"}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "d"}, []string{"method", "route"}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /silent", func(w http.ResponseWriter, _ *http.Request) { writeError(w, 500, "could not check the code") })
	mux.HandleFunc("POST /bare", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) })
	mux.HandleFunc("POST /gateway", func(w http.ResponseWriter, _ *http.Request) { writeError(w, 502, "connector operation failed") })
	mux.HandleFunc("POST /logged", func(w http.ResponseWriter, r *http.Request) {
		slog.ErrorContext(r.Context(), "specific", "error", "disk full")
		writeError(w, 500, "could not save")
	})
	mux.HandleFunc("POST /shared", func(w http.ResponseWriter, _ *http.Request) { writeItemError(w, errors.New("boom")) })
	mux.HandleFunc("POST /panic", func(http.ResponseWriter, *http.Request) { panic("boom") })
	for _, code := range []int{400, 403, 404, 409, 503} {
		mux.HandleFunc(fmt.Sprintf("POST /status%d", code), func(w http.ResponseWriter, _ *http.Request) { writeError(w, code, "no") })
	}
	handler := s.observe(s.requestID(recoverPanic(mux)))
	send := func(path, id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path+"?token=SECRET-QUERY", strings.NewReader(`{"password":"SECRET-BODY"}`))
		req.Header.Set("X-Request-ID", id)
		req.Header.Set("Authorization", "Bearer SECRET-KEY")
		req.Header.Set("Cookie", "pms_session=SECRET-COOKIE")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	errorsFor := func(id string) []map[string]any {
		mu.Lock()
		defer mu.Unlock()
		var found []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
			var entry map[string]any
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				t.Fatalf("not JSON: %s", line)
			}
			if entry["level"] == "ERROR" && entry["request_id"] == id {
				found = append(found, entry)
			}
		}
		return found
	}
	for path, want := range map[string]int{"/silent": 1, "/bare": 1, "/gateway": 1, "/logged": 1, "/shared": 1, "/panic": 1,
		"/status400": 0, "/status403": 0, "/status404": 0, "/status409": 0, "/status503": 0} {
		id := "id-" + path[1:]
		send(path, id)
		if got := errorsFor(id); len(got) != want {
			t.Errorf("%s: %d ERROR lines, want %d: %v", path, len(got), want, got)
		}
	}
	line := errorsFor("id-silent")[0]
	if line["method"] != "POST" || line["route"] != "POST /silent" || line["status"] != float64(500) || line["category"] != "internal_error" ||
		!strings.HasPrefix(fmt.Sprint(line["source"]), "request_log_internal_test.go:") {
		t.Errorf("context: %v", line)
	}
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			send([]string{"/silent", "/logged"}[i%2], fmt.Sprintf("par-%d", i))
		}()
	}
	wg.Wait()
	for i := range 50 {
		if got := errorsFor(fmt.Sprintf("par-%d", i)); len(got) != 1 {
			t.Errorf("concurrent %d: %v", i, got)
		}
	}
	if strings.Contains(out.String(), "SECRET-") {
		t.Errorf("request secrets reached the log")
	}
}

type lockedWriter struct {
	mu *sync.Mutex
	w  *bytes.Buffer
}

func (l lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
