// Package reqid carries a request's id, so the access log, the application log and the audit trail name the same request.
package reqid

import (
	"context"
	"log/slog"
	"sync/atomic"
)

type key struct{}

func With(ctx context.Context, id string) context.Context { return context.WithValue(ctx, key{}, id) }

// From returns the request id, or "" outside a request (a background job).
func From(ctx context.Context) string {
	id, _ := ctx.Value(key{}).(string)
	return id
}

type errorKey struct{}

// Track returns a context that remembers whether an ERROR record was written with it (or a context derived from it),
// so the access log can write one for a failed request that nothing else did, without writing two.
func Track(ctx context.Context) (context.Context, *atomic.Bool) {
	logged := new(atomic.Bool)
	return context.WithValue(ctx, errorKey{}, logged), logged
}

// Handler adds request_id to every record logged with a request's context (slog.ErrorContext and friends).
func Handler(next slog.Handler) slog.Handler { return handler{next} }

type handler struct{ slog.Handler }

func (h handler) Handle(ctx context.Context, r slog.Record) error {
	if logged, ok := ctx.Value(errorKey{}).(*atomic.Bool); ok && r.Level >= slog.LevelError {
		logged.Store(true)
	}
	if id := From(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return handler{h.Handler.WithAttrs(attrs)}
}
func (h handler) WithGroup(name string) slog.Handler { return handler{h.Handler.WithGroup(name)} }
