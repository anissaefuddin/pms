package mail

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"github.com/anissaefuddin/pms/server/internal/previews"
	"github.com/anissaefuddin/pms/server/internal/storage"
	htmlpkg "html"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	neturl "net/url"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/housekeeping"
	"github.com/anissaefuddin/pms/server/internal/projectexcel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

type Args struct {
	To       string       `json:"to"`
	Template string       `json:"template"`
	URL      string       `json:"url"`
	Locale   string       `json:"locale"`
	Message  string       `json:"message,omitempty"`
	Subject  string       `json:"subject,omitempty"` // report_digest only
	HTML     string       `json:"html,omitempty"`    // report_digest only; the sender escapes every value in it
	Data     TemplateData `json:"data,omitempty"`
	// Form confirmation branding (FORM-04, request_received only); every value is escaped when rendered.
	SenderName string `json:"sender_name,omitempty"`
	LogoURL    string `json:"logo_url,omitempty"`
	Greeting   string `json:"greeting,omitempty"`
	Closing    string `json:"closing,omitempty"`
}

type TemplateData struct {
	Kind, Preheader, Actor, Item, ItemType, Space, Status, Before, After        string
	Due, OccurredAt, Access, Workspace, Inviter, Expires, Period, Metric, Total string
	Rows                                                                        []ReportRow `json:"rows,omitempty"`
	Days                                                                        int         `json:"days,omitempty"`
}

type ReportRow struct{ Label, Break, Value string }

func (Args) Kind() string { return "send_email" }

type Worker struct {
	river.WorkerDefaults[Args]
	cfg config.Config
}

func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	return w.deliver(ctx, job.Args)
}

func (w *Worker) deliver(ctx context.Context, args Args) error {
	subject, text, html, err := render(args)
	if err != nil {
		return err
	}
	if w.cfg.SMTPHost == "" {
		// Log mode for instances without SMTP; the link is a one-time secret, so only run like this on trusted logs.
		slog.Warn("SMTP is not configured; email logged instead of sent", "to", args.To, "subject", subject, "url", args.URL)
		return nil
	}
	sender := w.cfg.SMTPFrom
	if (args.Template == "set_password" || args.Template == "reset_password" || args.Template == "login_code") && w.cfg.SMTPFromAuth != "" {
		sender = w.cfg.SMTPFromAuth
	}
	from, err := mail.ParseAddress(sender)
	if err != nil {
		return err
	}
	if args.Template == "request_received" && args.SenderName != "" {
		from.Name = args.SenderName // the address stays the instance's own; only the display name is the form's
		sender = from.String()      // RFC 5322 quoting keeps odd characters in the name from altering the header
	}
	address := net.JoinHostPort(w.cfg.SMTPHost, w.cfg.SMTPPort)
	var auth smtp.Auth
	if w.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", w.cfg.SMTPUser, w.cfg.SMTPPassword, w.cfg.SMTPHost)
	}
	msg, err := message(sender, from.Address, w.cfg.SMTPReplyTo, args.To, subject, text, html)
	if err != nil {
		return err
	}
	return send(ctx, address, w.cfg.SMTPHost, auth, from.Address, args.To, msg)
}

// message builds the mail text. Date and Message-ID are required by many receivers; without them mail is often marked as spam.
func message(fromHeader, fromAddress, replyTo, to, subject, text, html string) ([]byte, error) {
	for _, field := range []string{fromHeader, replyTo, to, subject} { // a line break here would let data inject extra headers
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
	if replyTo != "" {
		headers = append([]string{"Reply-To: " + replyTo}, headers...)
	}
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

// Option lets packages that depend on mail (and so cannot be imported by it) add their own workers and periodic jobs to the one River client.
type Option func(workers *river.Workers, periodic *[]*river.PeriodicJob)

func New(pool *pgxpool.Pool, cfg config.Config, options ...Option) (*Queue, error) {
	workers := river.NewWorkers()
	mailer := &Worker{cfg: cfg}
	river.AddWorker(workers, mailer)
	river.AddWorker(workers, &projectexcel.ExportWorker{Pool: pool})
	river.AddWorker(workers, &previews.Worker{Pool: pool, Store: storage.LocalStorage{Dir: cfg.FilesDir}})
	river.AddWorker(workers, &InvitationWorker{pool: pool, mail: mailer})
	river.AddWorker(workers, &NotificationWorker{pool: pool, mail: mailer})
	// This is the process's only River client, so the daily cleanup is registered here too.
	river.AddWorker(workers, &housekeeping.Worker{Pool: pool, FilesDir: sweepDir(cfg.Storage, cfg.FilesDir)})
	periodic := []*river.PeriodicJob{river.NewPeriodicJob(river.PeriodicInterval(24*time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return housekeeping.Args{}, nil }, &river.PeriodicJobOpts{RunOnStart: true})}
	for _, option := range options {
		option(workers, &periodic)
	}
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}}, Workers: workers,
		// Job args hold one-time links; keep finished jobs for an hour instead of River's 24 hours.
		// shortcut: discarded jobs keep River's default retention (needed to debug failures) and backups still contain job args; encrypt the args if that matters.
		CompletedJobRetentionPeriod: time.Hour,
		PeriodicJobs:                periodic,
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
	switch args.Template {
	case "notification":
		return renderProductEmail(args)
	case "request_received", "request_status":
		// Message is composed by the server from the form name, the item title and the status; it holds no internal data.
		subject = map[string]string{"request_received": "We received your request", "request_status": "Your request was updated"}[args.Template]
		if args.Template == "request_received" && (args.Greeting != "" || args.Closing != "" || args.LogoURL != "") {
			return subject, brandedText(args), brandedHTML(args), nil
		}
		return subject, args.Message, fmt.Sprintf(`<p>%s</p>`, htmlpkg.EscapeString(args.Message)), nil
	case "report_digest":
		return renderProductEmail(args)
	case "set_password":
		return renderProductEmail(args)
	case "login_code":
		// The code is the whole secret: six digits, ten minutes, one use. Message carries it (like the reset link, it sits in the job row for a while).
		return "Your PMS sign-in code is " + args.Message,
			fmt.Sprintf("Your PMS sign-in code is %s. It expires in 10 minutes and works once. If you did not ask for it, ignore this email.", args.Message),
			fmt.Sprintf(`<!doctype html><html><body style="font-family:Arial,sans-serif;color:#111827"><h2>Your sign-in code</h2><p style="font-size:28px;letter-spacing:6px;font-weight:bold">%s</p><p style="color:#6b7280;font-size:13px">It expires in 10 minutes and works once. If you did not ask for it, ignore this email.</p></body></html>`, htmlpkg.EscapeString(args.Message)), nil
	case "reset_password":
		return renderProductEmail(args)
	default:
		return "", "", "", fmt.Errorf("unknown email template %q", args.Template)
	}
}

func (q *Queue) ExportProjectTx(ctx context.Context, tx pgx.Tx, args projectexcel.ExportArgs) error {
	_, err := q.client.InsertTx(ctx, tx, args, &river.InsertOpts{MaxAttempts: 3})
	return err
}

func brandedText(args Args) string {
	parts := []string{}
	for _, p := range []string{args.Greeting, args.Message, args.Closing} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "\n\n")
}

// brandedHTML renders the confirmation with the form's greeting, message, closing and logo. Every value is escaped; the
// logo address must be https (checked when the form is saved and again here).
func brandedHTML(args Args) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html><body style="font-family:Arial,sans-serif;color:#111827">`)
	if u, err := neturl.Parse(args.LogoURL); err == nil && u.Scheme == "https" && u.Host != "" {
		fmt.Fprintf(&b, `<p><img src="%s" alt="" style="max-height:48px"></p>`, htmlpkg.EscapeString(args.LogoURL))
	}
	for _, p := range []string{args.Greeting, args.Message, args.Closing} {
		if p != "" {
			fmt.Fprintf(&b, "<p>%s</p>", strings.ReplaceAll(htmlpkg.EscapeString(p), "\n", "<br>"))
		}
	}
	b.WriteString("</body></html>")
	return b.String()
}

// sweepDir limits the orphan sweep to local files; remote backends are not listed (see D-83).
func sweepDir(backend, dir string) string {
	if backend == "" || backend == "local" {
		return dir
	}
	return ""
}
