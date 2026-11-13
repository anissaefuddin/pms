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

func TestTwoFactorKeys(t *testing.T) {
	good := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" // 32 bytes
	t.Setenv("DATABASE_URL", "postgres://x")
	for name, c := range map[string]struct {
		key, previous string
		ok            bool
	}{
		"no keys":                  {"", "", true},
		"current only":             {good, "", true},
		"rotation":                 {good, good, true},
		"not base64":               {"not base64!", "", false},
		"wrong length":             {"c2hvcnQ=", "", false},
		"previous without current": {"", good, false},
	} {
		t.Setenv("PMS_2FA_KEY", c.key)
		t.Setenv("PMS_2FA_KEY_PREVIOUS", c.previous)
		cfg, err := Load()
		if (err == nil) != c.ok {
			t.Errorf("%s: err=%v, want ok=%v", name, err, c.ok)
		}
		if err == nil && (c.key != "") != (len(cfg.TwoFactorKey) == 32) {
			t.Errorf("%s: key loaded=%v", name, len(cfg.TwoFactorKey))
		}
	}
}
