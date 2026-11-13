package httpserver_test

import (
	"os"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/automation"
)

// TestBrowserM9Flows runs E2E-PAR-04 (a user builds a rule in the UI and it acts on a new item) against the real API and PostgreSQL.
// The dispatcher normally runs as a River periodic job; here it is looped so the browser run does not wait for the queue.
func TestBrowserM9Flows(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	admin := f.person("admin")
	f.exec("UPDATE users SET first_name='Ada',last_name='Admin' WHERE id=$1", admin.id)
	reviewer := f.person("regular")
	second := f.person("regular")
	f.exec("UPDATE users SET first_name='Bea',last_name='Reviewer' WHERE id=$1", reviewer.id)
	f.exec("UPDATE users SET first_name='Cal',last_name='Reviewer' WHERE id=$1", second.id)
	work := f.space("M9 Work")
	f.share(work, "user", admin.id, "full")
	f.share(work, "user", reviewer.id, "read")
	f.share(work, "user", second.id, "read")
	engine := automation.New(f.pool)
	go func() {
		for t.Context().Err() == nil {
			_, _ = engine.Dispatch(t.Context())
			time.Sleep(200 * time.Millisecond)
		}
	}()
	runBrowserFixture(t, f, []string{"m9-live.spec.ts"}, "PMS_FIXTURE_M9_ADMIN="+admin.cookie.String(), "PMS_FIXTURE_M9_WORK="+work,
		"PMS_FIXTURE_M9_REVIEWER="+reviewer.cookie.String(), "PMS_FIXTURE_M9_SECOND="+second.id)
}
