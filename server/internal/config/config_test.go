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

func TestOTPKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	for _, tc := range []struct {
		value string
		ok    bool
	}{{"", true}, {"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", true}, {"c2hvcnQ=", false}, {"not base64!", false}} {
		t.Setenv("PMS_OTP_KEY", tc.value)
		cfg, err := Load()
		if (err == nil) != tc.ok {
			t.Errorf("PMS_OTP_KEY=%q: %v", tc.value, err)
		}
		if err == nil && (tc.value != "") != (len(cfg.OTPKey) == 32) {
			t.Errorf("PMS_OTP_KEY=%q loaded %d bytes", tc.value, len(cfg.OTPKey))
		}
	}
}

func TestStorageBackendNeedsItsSettings(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	for _, c := range []struct {
		backend, key string
		ok           bool
	}{{"", "", true}, {"local", "", true}, {"s3", "", false}, {"s3", "PMS_S3_URL", false}, {"pydio", "", false}, {"pydio", "PMS_PYDIO_URL", false}, {"ftp", "", false}} {
		t.Setenv("PMS_STORAGE", c.backend)
		for _, k := range []string{"PMS_S3_URL", "PMS_S3_ACCESS_KEY", "PMS_S3_SECRET_KEY", "PMS_PYDIO_URL", "PMS_PYDIO_TOKEN"} {
			t.Setenv(k, "")
		}
		if c.key != "" {
			t.Setenv(c.key, "https://s3.example")
		}
		if _, err := Load(); (err == nil) != c.ok {
			t.Fatalf("%+v: %v", c, err)
		}
	}
	t.Setenv("PMS_STORAGE", "s3")
	t.Setenv("PMS_S3_URL", "https://s3.example/pms")
	t.Setenv("PMS_S3_ACCESS_KEY", "a")
	t.Setenv("PMS_S3_SECRET_KEY", "b")
	if cfg, err := Load(); err != nil || cfg.Storage != "s3" {
		t.Fatalf("s3: %+v %v", cfg, err)
	}
}

func TestPydioLinkNeedsTokenAndRoots(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("PMS_PYDIO_LINK_URL", "https://dms.example.com/")
	t.Setenv("PMS_PYDIO_LINK_TOKEN", "tok")
	t.Setenv("PMS_PYDIO_LINK_ROOTS", "")
	if _, err := Load(); err == nil {
		t.Fatal("a link URL without roots must fail the start")
	}
	t.Setenv("PMS_PYDIO_LINK_ROOTS", " /projects/ ,common-files/Clients")
	cfg, err := Load()
	if err != nil || cfg.PydioLinkURL != "https://dms.example.com" || len(cfg.PydioLinkRoots) != 2 || cfg.PydioLinkRoots[0] != "projects" || cfg.PydioLinkRoots[1] != "common-files/Clients" {
		t.Fatalf("cfg = %+v, %v", cfg, err)
	}
	t.Setenv("PMS_PYDIO_LINK_URL", "https://dms.example.com/io")
	if _, err := Load(); err == nil {
		t.Fatal("a link URL with a path must fail the start")
	}
}

// B3-17: public links are off unless PMS_PUBLIC_LINKS is exactly "true".
func TestPublicLinksDefaultOff(t *testing.T) {
	t.Setenv("PMS_PUBLIC_LINKS", "")
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/x?sslmode=disable")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicLinks {
		t.Fatal("public links must be off by default")
	}
	t.Setenv("PMS_PUBLIC_LINKS", "true")
	if cfg, err = Load(); err != nil || !cfg.PublicLinks {
		t.Fatalf("PMS_PUBLIC_LINKS=true must enable public links: %v", err)
	}
}

// PY-14 (B4-08): a fully configured Pydio storage backend is still refused at startup.
func TestPydioStorageIsRefusedEvenWhenFullyConfigured(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("PMS_STORAGE", "pydio")
	t.Setenv("PMS_PYDIO_URL", "https://cells.example.com/io")
	t.Setenv("PMS_PYDIO_TOKEN", "token-value")
	if _, err := Load(); err == nil {
		t.Fatal("PMS_STORAGE=pydio must fail at startup, even with URL and token")
	}
}

// B4-02: the Pydio link origin must be https when it is configured from the environment.
func TestPydioLinkOriginMustBeHTTPS(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("PMS_PYDIO_LINK_TOKEN", "link-token")
	t.Setenv("PMS_PYDIO_LINK_ROOTS", "projects")
	t.Setenv("PMS_PYDIO_LINK_URL", "http://dms.example.com")
	if _, err := Load(); err == nil {
		t.Fatal("an http Pydio link origin must be refused")
	}
	t.Setenv("PMS_PYDIO_LINK_URL", "https://dms.example.com")
	t.Setenv("PMS_PYDIO_LINK_TOKEN", "link-token")
	t.Setenv("PMS_PYDIO_LINK_ROOTS", "projects")
	if _, err := Load(); err != nil {
		t.Fatalf("an https origin must load: %v", err)
	}
}

// PROMPT #028 Wave 1: link-first needs an https origin and approved roots, not a Pydio credential. Listing is separate and stricter.
func TestStandaloneLinkConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("PMS_PYDIO_LINK_URL", "https://dms.example.com")
	t.Setenv("PMS_PYDIO_LINK_TOKEN", "")
	t.Setenv("PMS_PYDIO_LINK_ROOTS", "projects,common-files/Clients")
	t.Setenv("PMS_PYDIO_LISTING", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("link-only configuration must load without a Pydio credential: %v", err)
	}
	if cfg.PydioListing {
		t.Fatal("listing must stay OFF by default")
	}
	t.Setenv("PMS_PYDIO_LISTING", "true")
	if _, err := Load(); err == nil {
		t.Fatal("listing without a credential must be refused")
	}
	t.Setenv("PMS_PYDIO_LISTING", "false")
	for _, roots := range []string{"projects/../x", "projects//x", "projects\\x", ""} {
		t.Setenv("PMS_PYDIO_LINK_ROOTS", roots)
		if _, err := Load(); err == nil {
			t.Fatalf("roots %q must be refused", roots)
		}
	}
}
