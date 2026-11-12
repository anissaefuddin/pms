package mail

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	htmlpkg "html"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/housekeeping"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

type Args struct {
	To       string `json:"to"`
	Template string `json:"template"`
	URL      string `json:"url"`
	Locale   string `json:"locale"`
}

func (Args) Kind() string { return "send_email" }

type Worker struct {
	river.WorkerDefaults[Args]
	cfg config.Config
}

func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	subject, text, html, err := render(job.Args)
	if err != nil {
		return err
	}
	if w.cfg.SMTPHost == "" {
		// Log mode for instances without SMTP; the link is a one-time secret, so only run like this on trusted logs.
		slog.Warn("SMTP is not configured; email logged instead of sent", "to", job.Args.To, "subject", subject, "url", job.Args.URL)
		return nil
	}
	from, err := mail.ParseAddress(w.cfg.SMTPFrom)
	if err != nil {
		return err
	}
	address := net.JoinHostPort(w.cfg.SMTPHost, w.cfg.SMTPPort)
	var auth smtp.Auth
	if w.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", w.cfg.SMTPUser, w.cfg.SMTPPassword, w.cfg.SMTPHost)
	}
	msg, err := message(w.cfg.SMTPFrom, from.Address, job.Args.To, subject, text, html)
	if err != nil {
		return err
	}
	return send(ctx, address, w.cfg.SMTPHost, auth, from.Address, job.Args.To, msg)
}

// message builds the mail text. Date and Message-ID are required by many receivers; without them mail is often marked as spam.
func message(fromHeader, fromAddress, to, subject, text, html string) ([]byte, error) {
	for _, field := range []string{fromHeader, to, subject} { // a line break here would let data inject extra headers
		if strings.ContainsAny(field, "\r\n") {
			return nil, fmt.Errorf("email header contains a line break")
		}
	}
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	boundary := fmt.Sprintf("pms-%x", id)
	domain := "localhost"
	if at := strings.LastIndex(fromAddress, "@"); at >= 0 {
		domain = fromAddress[at+1:]
	}
	var body strings.Builder
	for _, part := range []struct{ kind, content string }{{"text/plain", text}, {"text/html", html}} {
		fmt.Fprintf(&body, "--%s\r\nContent-Type: %s; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary, part.kind)
		w := quotedprintable.NewWriter(&body)
		_, _ = w.Write([]byte(part.content))
		_ = w.Close()
		body.WriteString("\r\n")
	}
	fmt.Fprintf(&body, "--%s--\r\n", boundary)
	headers := []string{"From: " + fromHeader, "To: " + to, "Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().UTC().Format(time.RFC1123Z), fmt.Sprintf("Message-ID: <%x@%s>", id, domain),
		"MIME-Version: 1.0", fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q", boundary), ""}
	return []byte(strings.Join(headers, "\r\n") + "\r\n" + body.String()), nil
}

// smtpTimeout bounds one whole delivery; without it a silent server holds a job worker forever.
var smtpTimeout = 30 * time.Second

// send requires STARTTLS for non-local SMTP hosts.
func send(ctx context.Context, address, host string, auth smtp.Auth, from, to string, message []byte) error {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(smtpTimeout)); err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	} else if host != "localhost" && host != "127.0.0.1" {
		return fmt.Errorf("SMTP server does not offer STARTTLS")
	}
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(message); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

type Queue struct{ client *river.Client[pgx.Tx] }

func New(pool *pgxpool.Pool, cfg config.Config) (*Queue, error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &Worker{cfg: cfg})
	// This is the process's only River client, so the daily cleanup is registered here too.
	river.AddWorker(workers, &housekeeping.Worker{Pool: pool})
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}}, Workers: workers,
		// Job args hold one-time links; keep finished jobs for an hour instead of River's 24 hours.
		// shortcut: discarded jobs keep River's default retention (needed to debug failures) and backups still contain job args; encrypt the args if that matters.
		CompletedJobRetentionPeriod: time.Hour,
		PeriodicJobs: []*river.PeriodicJob{river.NewPeriodicJob(river.PeriodicInterval(24*time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return housekeeping.Args{}, nil }, &river.PeriodicJobOpts{RunOnStart: true})},
	})
	if err != nil {
		return nil, err
	}
	return &Queue{client: client}, nil
}

func (q *Queue) Start(ctx context.Context) error { return q.client.Start(ctx) }
func (q *Queue) Stop(ctx context.Context) error  { return q.client.Stop(ctx) }
func (q *Queue) SendTx(ctx context.Context, tx pgx.Tx, args Args) error {
	_, err := q.client.InsertTx(ctx, tx, args, &river.InsertOpts{MaxAttempts: 8})
	return err
}

// render returns the subject, plain text and HTML body of a template. Only English exists (MVP); other locales fall back to it.
func render(args Args) (subject, text, html string, err error) {
	link := htmlpkg.EscapeString(args.URL)
	page := func(heading, intro, expires string) string {
		return fmt.Sprintf(`<!doctype html><html><body style="font-family:Arial,sans-serif;color:#111827"><h2>%s</h2><p>%s</p><p><a href="%s" style="background:#2563eb;color:#ffffff;padding:10px 16px;border-radius:6px;text-decoration:none">Open link</a></p><p style="color:#6b7280;font-size:13px">%s If the button does not work, copy this address into your browser:<br>%s</p></body></html>`, heading, intro, link, expires, link)
	}
	switch args.Template {
	case "set_password":
		return "Set your PMS password",
			fmt.Sprintf("Open %s to activate your account and set your password. This link expires in 7 days.", args.URL),
			page("Activate your PMS account", "You have been invited to PMS. Activate your account and choose a password.", "This link expires in 7 days."), nil
	case "reset_password":
		return "Reset your PMS password",
			fmt.Sprintf("Open %s to reset your password. This link expires in one hour.", args.URL),
			page("Reset your PMS password", "We received a request to reset your password. If it was not you, ignore this email.", "This link expires in one hour."), nil
	default:
		return "", "", "", fmt.Errorf("unknown email template %q", args.Template)
	}
}
