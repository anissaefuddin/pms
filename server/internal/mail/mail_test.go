package mail

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRenderFallsBackToEnglish(t *testing.T) {
	subject, text, html, err := render(Args{Template: "reset_password", URL: "https://example.test/reset", Locale: "id"})
	if err != nil {
		t.Fatal(err)
	}
	if subject != "Reset your PMS password" || text == "" || !strings.Contains(html, `href="https://example.test/reset"`) {
		t.Fatalf("unexpected template: %q %q", subject, text)
	}
}

func TestRenderRejectsUnknownTemplate(t *testing.T) {
	if _, _, _, err := render(Args{Template: "missing"}); err == nil {
		t.Fatal("expected template error")
	}
}

func TestMessageHasDateAndMessageID(t *testing.T) {
	raw, err := message("PMS <noreply@acme.test>", "noreply@acme.test", "u@x.test", "Hi", "body", "<p>body</p>")
	if err != nil {
		t.Fatal(err)
	}
	m := string(raw)
	for _, want := range []string{"\r\nDate: ", "\r\nMessage-ID: <", "@acme.test>", "\r\nTo: u@x.test"} {
		if !strings.Contains(m, want) {
			t.Fatalf("message lacks %q:\n%s", want, m)
		}
	}
}

// A server that accepts the connection but never greets (as on port 465, which expects TLS first) must not hang the worker.
func TestSendGivesUpOnSilentServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	old := smtpTimeout
	smtpTimeout = 200 * time.Millisecond
	defer func() { smtpTimeout = old }()
	done := make(chan error, 1)
	go func() {
		done <- send(context.Background(), ln.Addr().String(), "localhost", nil, "a@x.test", "b@x.test", []byte("x"))
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a timeout error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("send did not give up on a silent server")
	}
}

// fakeSMTP is a minimal plain SMTP server: it records delivered messages and can refuse recipients.
type fakeSMTP struct {
	ln       net.Listener
	mu       sync.Mutex
	messages []string
	refuse   bool
}

func newFakeSMTP(t *testing.T, refuse bool) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{ln: ln, refuse: refuse}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	return f
}

func (f *fakeSMTP) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	reply := func(s string) { fmt.Fprintf(c, "%s\r\n", s) }
	reply("220 fake ready")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		switch cmd := strings.ToUpper(strings.TrimSpace(line)); {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			reply("250 fake")
		case strings.HasPrefix(cmd, "MAIL"):
			reply("250 ok")
		case strings.HasPrefix(cmd, "RCPT"):
			if f.refuse {
				reply("550 mailbox unavailable")
			} else {
				reply("250 ok")
			}
		case cmd == "DATA":
			reply("354 go ahead")
			var body strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil || l == ".\r\n" {
					break
				}
				body.WriteString(l)
			}
			f.mu.Lock()
			f.messages = append(f.messages, body.String())
			f.mu.Unlock()
			reply("250 queued")
		case cmd == "QUIT":
			reply("221 bye")
			return
		default:
			reply("250 ok")
		}
	}
}

func TestSendDeliversExactlyOneMessage(t *testing.T) {
	f := newFakeSMTP(t, false)
	subject, text, html, _ := render(Args{Template: "set_password", URL: "https://pms.test/reset-password?token=abc"})
	msg, err := message("PMS <noreply@pms.test>", "noreply@pms.test", "admin@pms.test", subject, text, html)
	if err != nil {
		t.Fatal(err)
	}
	if err := send(context.Background(), f.ln.Addr().String(), "127.0.0.1", nil, "noreply@pms.test", "admin@pms.test", msg); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.messages) != 1 || !strings.Contains(f.messages[0], "Subject: Set your PMS password") {
		t.Fatalf("expected one invitation message, got %q", f.messages)
	}
	if parts := messageParts(t, f.messages[0]); !strings.Contains(parts["text/plain"], "token=abc") || !strings.Contains(parts["text/html"], "token=abc") {
		t.Fatalf("both parts must carry the link: %v", parts)
	}
}

// A refused recipient must surface as an error so River retries the job.
func TestSendReportsRefusal(t *testing.T) {
	f := newFakeSMTP(t, true)
	err := send(context.Background(), f.ln.Addr().String(), "127.0.0.1", nil, "a@pms.test", "b@pms.test", []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "550") {
		t.Fatalf("expected the 550 refusal, got %v", err)
	}
}

// The delivered message is multipart/alternative with a plain-text and an HTML part, both carrying the link.
func TestMessageHasTextAndHTMLParts(t *testing.T) {
	_, text, html, _ := render(Args{Template: "set_password", URL: "https://pms.test/invitations/abc?x=1&y=2"})
	raw, err := message("PMS <noreply@pms.test>", "noreply@pms.test", "u@x.test", "Set your PMS password", text, html)
	if err != nil {
		t.Fatal(err)
	}
	got := messageParts(t, string(raw))
	if !strings.Contains(got["text/plain"], "https://pms.test/invitations/abc?x=1&y=2") {
		t.Fatalf("plain part lacks the link: %q", got["text/plain"])
	}
	if !strings.Contains(got["text/html"], `href="https://pms.test/invitations/abc?x=1&amp;y=2"`) {
		t.Fatalf("html part lacks an escaped link: %q", got["text/html"])
	}
}

func TestMessageRejectsHeaderInjection(t *testing.T) {
	for _, to := range []string{"a@x.test\r\nBcc: evil@x.test", "a@x.test\nBcc: evil@x.test"} {
		if _, err := message("PMS <n@x.test>", "n@x.test", to, "Hi", "t", "<p>t</p>"); err == nil {
			t.Fatalf("recipient %q with a line break was accepted", to)
		}
	}
	if _, err := message("PMS <n@x.test>", "n@x.test", "a@x.test", "Hi\r\nBcc: evil@x.test", "t", "<p>t</p>"); err == nil {
		t.Fatal("subject with a line break was accepted")
	}
}

// messageParts returns the decoded bodies of a multipart/alternative message by media type.
func messageParts(t *testing.T, raw string) map[string]string {
	t.Helper()
	parsed, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	mediaType, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/alternative" {
		t.Fatalf("content type %q (%v)", mediaType, err)
	}
	reader := multipart.NewReader(parsed.Body, params["boundary"])
	got := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}
		body, _ := io.ReadAll(part) // multipart.Part already decodes quoted-printable
		kind, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		got[kind] = string(body)
	}
	return got
}
