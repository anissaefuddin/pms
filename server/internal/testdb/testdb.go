// Package testdb gives each integration test its own empty database, so tests can be re-run and run in parallel.
package testdb

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// New creates an empty database on the server in PMS_TEST_DATABASE_URL and returns its URL.
// The test is skipped when the variable is unset; the database is dropped when the test ends.
func New(t *testing.T) string {
	t.Helper()
	baseURL := os.Getenv("PMS_TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("PMS_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	name := fmt.Sprintf("pms_test_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(ctx, baseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if c, err := pgx.Connect(ctx, baseURL); err == nil {
			_, _ = c.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
			c.Close(ctx)
		}
	})
	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}
