package reqid

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestHandlerAddsTheRequestID(t *testing.T) {
	var out bytes.Buffer
	log := slog.New(Handler(slog.NewJSONHandler(&out, nil))).With("app", "pms")
	log.ErrorContext(With(context.Background(), "req-1"), "boom")
	if !strings.Contains(out.String(), `"request_id":"req-1"`) || !strings.Contains(out.String(), `"app":"pms"`) {
		t.Fatal(out.String())
	}
	out.Reset()
	log.Error("job")
	if strings.Contains(out.String(), "request_id") {
		t.Fatalf("a job has no request: %s", out.String())
	}
}
