package config

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestDatabaseURLFromPartsEscapesPassword(t *testing.T) {
	const password = "p/a@s:s#w?o%r d+="
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "postgres")
	t.Setenv("DB_PASSWORD", password)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := pgx.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Password != password || parsed.Host != "postgres" || parsed.Database != "pms" {
		t.Fatalf("password did not survive the URL: %q host=%q db=%q", parsed.Password, parsed.Host, parsed.Database)
	}
}

func TestDatabaseURLRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PASSWORD", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error when no database is configured")
	}
}

func TestMaxSeatsMustBePositive(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	for _, bad := range []string{"0", "-1", "many"} {
		t.Setenv("MAX_SEATS", bad)
		if _, err := Load(); err == nil {
			t.Fatalf("MAX_SEATS=%q was accepted", bad)
		}
	}
}
