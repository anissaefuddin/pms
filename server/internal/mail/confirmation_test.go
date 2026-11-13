package mail

import (
	"net"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
)

// FORM-04: the form's wording is rendered with every value escaped, only an https logo is shown, and the sender name
// reaches the From header while the address stays the instance's own.
func TestBrandedConfirmationEscapesEverythingAndKeepsTheInstanceAddress(t *testing.T) {
	args := Args{To: "person@example.test", Template: "request_received", SenderName: "Acme Support", LogoURL: "https://cdn.example.test/logo.png?a=1&b=2",
		Greeting: `Hi <b>Sam</b>`, Message: "Got \"it\"\n<script>alert(1)</script>", Closing: "Thanks & bye"}
	_, text, html, err := render(args)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"<script>", "<b>Sam</b>"} {
		if strings.Contains(html, bad) {
			t.Errorf("unescaped %q in html: %s", bad, html)
		}
	}
	for _, want := range []string{"&lt;script&gt;", "&lt;b&gt;Sam&lt;/b&gt;", "Thanks &amp; bye", `src="https://cdn.example.test/logo.png?a=1&amp;b=2"`, "<br>"} {
		if !strings.Contains(html, want) {
			t.Errorf("html lacks %q: %s", want, html)
		}
	}
	if !strings.Contains(text, "Hi <b>Sam</b>") || !strings.Contains(text, "Thanks & bye") {
		t.Errorf("plain text must carry the wording as written: %q", text)
	}
	// Anything but https never becomes an image, and a quote in the address cannot break out of the attribute.
	for _, logo := range []string{"http://cdn.example.test/l.png", "javascript:alert(1)", "https:///nohost"} {
		a := args
		a.LogoURL = logo
		if _, _, h, _ := render(a); strings.Contains(h, "<img") {
			t.Errorf("logo %q rendered as an image", logo)
		}
	}
	a := args
	a.LogoURL = `https://x.test/"onerror="x`
	if _, _, h, _ := render(a); strings.Contains(h, `"onerror="`) {
		t.Errorf("logo broke out of the attribute: %s", h)
	}
	// Without any branding the old plain confirmation is unchanged.
	if _, plain, h, _ := render(Args{Template: "request_received", Message: "Plain <b>x</b>"}); plain != "Plain <b>x</b>" || h != "<p>Plain &lt;b&gt;x&lt;/b&gt;</p>" {
		t.Errorf("default confirmation changed: %q %q", plain, h)
	}

	smtp := newFakeSMTP(t, false)
	host, port, _ := net.SplitHostPort(smtp.ln.Addr().String())
	w := &Worker{cfg: config.Config{SMTPHost: host, SMTPPort: port, SMTPFrom: "PMS <noreply@pms.test>"}}
	if err := w.deliver(t.Context(), args); err != nil {
		t.Fatal(err)
	}
	smtp.mu.Lock()
	defer smtp.mu.Unlock()
	if len(smtp.messages) != 1 || !strings.Contains(smtp.messages[0], "From: \"Acme Support\" <noreply@pms.test>") {
		t.Fatalf("sender name or address wrong: %q", smtp.messages)
	}
}
