package mail

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// LOG-01: a message that is not sent (SMTP off) is logged without its one-time link or the recipient address.
// The secret values below are synthetic.
func TestUnsentEmailLogHasNoLinkOrAddress(t *testing.T) {
	var out bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&out, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	const secret = "QA-SYNTHETIC-TOKEN-9f3c"
	const address = "qa-reader@qa-staging.invalid"
	args := Args{To: address, Template: "set_password", URL: "https://pms.example.test/invitations/" + secret}
	logUnsentEmail(args, "Set your password")
	line := out.String()
	if !strings.Contains(line, "email not sent") || !strings.Contains(line, "set_password") {
		t.Fatalf("the operator still needs the event and template: %q", line)
	}
	for _, leaked := range []string{secret, "invitations/", address, "qa-reader"} {
		if strings.Contains(line, leaked) {
			t.Fatalf("the log leaked %q: %q", leaked, line)
		}
	}
}
