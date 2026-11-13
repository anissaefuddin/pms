package httpserver_test

import (
	"os"
	"testing"
)

// TestBrowserTwoFactor runs the 2FA browser flow (S3.5.1) through the real UI, API and PostgreSQL:
// set up, sign in with a code, then turn it off again.
func TestBrowserTwoFactor(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture")
	}
	f := newFixture(t)
	user := f.withPassword(f.person("regular"))
	runBrowserFixture(t, f, []string{"two-factor-live.spec.ts"},
		"PMS_FIXTURE_2FA_EMAIL="+f.email(user),
		"PMS_FIXTURE_2FA_PASSWORD="+twoFactorPassword,
	)
}
