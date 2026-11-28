package bootstrap_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/bootstrap"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/dbgen"
	"github.com/anissaefuddin/pms/server/internal/testdb"
)

func TestBootstrapCreatesFoundationOnce(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.NewMigrated(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	result, err := bootstrap.Run(ctx, pool, "Test Account", "Admin@Example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.AdminID == "" || result.InviteToken == "" {
		t.Fatal("bootstrap did not return the admin and the invitation token")
	}
	count := func(query string) (n int) {
		t.Helper()
		if err := pool.QueryRow(ctx, query).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for query, want := range map[string]int{
		"SELECT count(*) FROM account": 1,
		"SELECT count(*) FROM users WHERE user_type='admin' AND status='invited' AND password_hash IS NULL AND email='admin@example.com'":                                                   1,
		"SELECT count(*) FROM spaces WHERE name='General' AND kind='shared' AND scope_state='internal' AND client_id IS NULL":                                                               1, // B3-SC: explicit internal, never a guess
		"SELECT count(*) FROM spaces WHERE name='General' AND kind='shared' AND owner_id IS NULL":                                                                                           1,
		"SELECT count(*) FROM space_admins sa JOIN spaces s ON s.id=sa.space_id AND s.name='General' JOIN shares sh ON sh.target_id=s.id AND sh.principal_id=sa.user_id AND sh.role='full'": 1, // DEF-001: never orphaned
		"SELECT count(*) FROM spaces WHERE kind='personal'":                                                                                                                                 0, // created when the invitation is accepted
		"SELECT count(*) FROM workflows WHERE is_default":                                                                                                                                   1,
		"SELECT count(*) FROM statuses":                                               5,
		"SELECT count(*) FROM user_tokens WHERE purpose='invite' AND used_at IS NULL": 1,
	} {
		if got := count(query); got != want {
			t.Fatalf("%s = %d, want %d", query, got, want)
		}
	}
	// sqlc-generated queries run against the real schema (no mocks).
	q := dbgen.New(pool)
	if n, err := q.CountSeatsInUse(ctx); err != nil || n != 1 {
		t.Fatalf("CountSeatsInUse = %d (%v), want 1: the invited admin holds a seat", n, err)
	}
	if name, err := q.AccountName(ctx); err != nil || name != "Test Account" {
		t.Fatalf("AccountName = %q (%v)", name, err)
	}
	if statuses, err := q.ListStatusesOfDefaultWorkflow(ctx); err != nil || len(statuses) != 5 || statuses[0].Name != "New" || statuses[4].Name != "Cancelled" {
		t.Fatalf("default workflow statuses = %+v (%v)", statuses, err)
	}
	var validity time.Duration
	var seconds float64
	if err := pool.QueryRow(ctx, "SELECT extract(epoch FROM expires_at - created_at) FROM user_tokens").Scan(&seconds); err != nil {
		t.Fatal(err)
	}
	validity = time.Duration(seconds) * time.Second
	if (validity - bootstrap.InviteValidity).Abs() > time.Minute {
		t.Fatalf("invitation valid for %v, want %v", validity, bootstrap.InviteValidity)
	}
	if _, err := bootstrap.Run(ctx, pool, "Test Account", "other@example.com", nil); err == nil {
		t.Fatal("second bootstrap with another email succeeded")
	}
	if got := count("SELECT count(*) FROM users WHERE id<>'00000000-0000-4000-8000-0000000000e1'"); got != 1 {
		t.Fatalf("second bootstrap changed data: %d users", got)
	}
	// Recovery: the same still-invited admin gets a fresh link, the old one stops working, nothing else is created.
	again, err := bootstrap.Run(ctx, pool, "Test Account", "admin@example.com", nil)
	if err != nil {
		t.Fatalf("re-issuing the invitation failed: %v", err)
	}
	if again.AdminID != result.AdminID || again.InviteToken == result.InviteToken {
		t.Fatal("re-issue must keep the admin and mint a new token")
	}
	for query, want := range map[string]int{
		"SELECT count(*) FROM user_tokens WHERE purpose='invite' AND used_at IS NULL": 1,
		"SELECT count(*) FROM account": 1, "SELECT count(*) FROM users WHERE id<>'00000000-0000-4000-8000-0000000000e1'": 1, "SELECT count(*) FROM spaces": 1, "SELECT count(*) FROM workflows": 1, "SELECT count(*) FROM statuses": 5,
	} {
		if got := count(query); got != want {
			t.Fatalf("after re-issue: %s = %d, want %d", query, got, want)
		}
	}
	// Once the admin is active there is nothing to recover.
	if _, err := pool.Exec(ctx, "UPDATE users SET status='active',password_hash='h',first_name='A',last_name='B'"); err != nil {
		t.Fatal(err)
	}
	if _, err := bootstrap.Run(ctx, pool, "Test Account", "admin@example.com", nil); err == nil {
		t.Fatal("bootstrap succeeded for an already active admin")
	}
}

// Operators running bootstrap at the same time must still end up with one workspace and one usable invitation.
// (Runs after the first one take the "re-issue" path for the same still-invited admin, so several may report success.)
func TestBootstrapConcurrentRunsCreateOneAccount(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.NewMigrated(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var wg sync.WaitGroup
	results := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := bootstrap.Run(ctx, pool, "Acme", "admin@example.com", nil)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded == 0 {
		t.Fatal("no bootstrap run succeeded")
	}
	for query, want := range map[string]int{
		"SELECT count(*) FROM account": 1, "SELECT count(*) FROM users WHERE id<>'00000000-0000-4000-8000-0000000000e1'": 1, "SELECT count(*) FROM spaces": 1,
		"SELECT count(*) FROM workflows": 1, "SELECT count(*) FROM statuses": 5,
		"SELECT count(*) FROM user_tokens WHERE purpose='invite' AND used_at IS NULL": 1,
	} {
		var got int
		if err := pool.QueryRow(ctx, query).Scan(&got); err != nil || got != want {
			t.Fatalf("%s = %d (%v), want %d", query, got, err, want)
		}
	}
}
