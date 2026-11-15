package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestReadRequestsCanceledAfterSessionLookup(t *testing.T) {
	f := newFixture(t)
	user := f.person("admin")
	space := f.space("Cancellation fixture")
	f.share(space, "user", user.id, "full")
	for _, tc := range []struct{ path, table string }{
		{"/me", "account"},
		{"/spaces/" + space + "/views", "views"},
		{"/time-entries", "time_entries"},
		{"/spaces/" + space + "/items", "item_locations"},
	} {
		t.Run(tc.table, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			tx, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err := tx.Exec(ctx, "LOCK TABLE "+tc.table+" IN ACCESS EXCLUSIVE MODE"); err != nil {
				t.Fatal(err)
			}

			requestCtx, abort := context.WithCancel(ctx)
			defer abort()
			req := httptest.NewRequest(http.MethodGet, "/api/v1"+tc.path, nil).WithContext(requestCtx)
			req.AddCookie(user.cookie)
			rec := httptest.NewRecorder()
			done := make(chan struct{})
			go func() {
				defer close(done)
				f.srv.Config.Handler.ServeHTTP(rec, req)
			}()

			// Wait until the handler, past authentication, is blocked by this transaction.
			for {
				var waiting bool
				err := tx.QueryRow(ctx, `SELECT EXISTS (
					SELECT 1 FROM pg_locks
					WHERE relation=$1::regclass AND database=(SELECT oid FROM pg_database WHERE datname=current_database())
					AND NOT granted
				)`, tc.table).Scan(&waiting)
				if err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				select {
				case <-done:
					t.Fatal("handler completed before reaching the blocked database query")
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-time.After(10 * time.Millisecond):
				}
			}
			abort()
			select {
			case <-done:
				if rec.Flushed || rec.Body.Len() != 0 || rec.Code >= 500 {
					t.Fatalf("abandoned request wrote status %d: %s", rec.Code, rec.Body.String())
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}

	t.Run("parallel active requests", func(t *testing.T) {
		start := make(chan struct{})
		var requests sync.WaitGroup
		for range 8 {
			for _, path := range []string{"/me", "/spaces/" + space + "/views", "/time-entries"} {
				requests.Go(func() {
					<-start
					req := httptest.NewRequest(http.MethodGet, "/api/v1"+path, nil)
					req.AddCookie(user.cookie)
					rec := httptest.NewRecorder()
					f.srv.Config.Handler.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
						t.Errorf("%s: got status %d, body %s", path, rec.Code, rec.Body.String())
					}
				})
			}
		}
		close(start)
		requests.Wait()
	})
}
