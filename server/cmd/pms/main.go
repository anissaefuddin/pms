package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anissaefuddin/pms/server/internal/automation"
	"github.com/anissaefuddin/pms/server/internal/bootstrap"
	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/connectors"
	"github.com/anissaefuddin/pms/server/internal/connectors/gcal"
	"github.com/anissaefuddin/pms/server/internal/connectors/github"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/httpserver"
	mailqueue "github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/reportdigest"
	"github.com/anissaefuddin/pms/server/internal/twofactor"
	"github.com/anissaefuddin/pms/server/internal/webhooks"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command == "version" {
		fmt.Println(version)
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	switch command {
	case "serve":
		return serve(ctx, pool, cfg)
	case "migrate":
		// database.Open above already applied pending migrations; running it again is a no-op.
		slog.Info("migrations are up to date")
		return nil
	case "bootstrap":
		flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
		account := flags.String("account", "", "account name")
		email := flags.String("admin-email", "", "administrator email")
		if err := flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		queue, err := mailqueue.New(pool, cfg)
		if err != nil {
			return err
		}
		result, err := bootstrap.Run(ctx, pool, *account, *email, func(ctx context.Context, tx pgx.Tx, to, _ string) error {
			if cfg.SMTPHost == "" {
				return nil // The CLI prints the link when SMTP is not configured.
			}
			var id string
			if err := tx.QueryRow(ctx, "SELECT id::text FROM users WHERE email=$1", to).Scan(&id); err != nil {
				return err
			}
			return queue.InviteTx(ctx, tx, id)
		})
		if err != nil {
			return err
		}
		slog.Info("instance bootstrapped", "admin_id", result.AdminID, "admin_email", *email)
		if cfg.SMTPHost == "" {
			slog.Warn("SMTP is not configured: give the administrator this link to activate the account (valid 7 days)", "url", fmt.Sprintf("%s/invitations/%s", cfg.PublicURL, result.InviteToken))
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q (use serve, migrate, bootstrap, or version)", command)
	}
}

func serve(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) error {
	if cfg.TwoFactorKey == nil {
		slog.Warn("PMS_2FA_KEY is not set: two-factor authentication is unavailable to every user")
	}
	if cfg.ConnectorKey == nil {
		slog.Warn("PMS_CONNECTOR_KEY is not set: connectors that need secrets cannot be installed, and webhook signing secrets are stored unencrypted")
	}
	engine := automation.New(pool)
	hooks := webhooks.New(pool).WithKeys(twofactor.Keys{Current: cfg.ConnectorKey, Previous: cfg.ConnectorKeyPrevious})
	webhooks.AllowInternal = os.Getenv("PMS_WEBHOOK_ALLOW_INTERNAL") == "true" // test mode only
	engine.Hook = hooks.Handler
	engine.Tick = func(ctx context.Context) error { _, err := hooks.Send(ctx); return err }
	connectors.Default.Register(github.New())
	connectors.Default.Register(gcal.New())
	connectorSync := &connectors.Service{Pool: pool, Keys: twofactor.Keys{Current: cfg.ConnectorKey, Previous: cfg.ConnectorKeyPrevious}}
	queue, err := mailqueue.New(pool, cfg, reportdigest.Option(pool, cfg.PublicURL), engine.Option(), httpserver.ApprovalDeadlines(pool), connectors.Option(connectorSync))
	if err != nil {
		return err
	}
	engine.Queue = queue
	if err := queue.Start(ctx); err != nil {
		return fmt.Errorf("start job queue: %w", err)
	}
	server := &http.Server{Addr: cfg.ListenAddress, Handler: httpserver.New(pool, cfg, queue),
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ReadHeaderTimeout: 5 * time.Second,
		// Long uploads and SSE set their own deadlines in their handlers.
		ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second,
	}
	// Metrics listen on an internal port that Traefik never routes to (compose does not publish it).
	metrics := &http.Server{Addr: cfg.MetricsAddress, Handler: httpserver.Metrics(), ReadHeaderTimeout: 5 * time.Second}
	errors := make(chan error, 2)
	go func() {
		slog.Info("server started", "address", cfg.ListenAddress, "metrics_address", cfg.MetricsAddress, "version", version)
		errors <- server.ListenAndServe()
	}()
	go func() { errors <- metrics.ListenAndServe() }()
	select {
	case err := <-errors:
		if err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = metrics.Shutdown(shutdownCtx)
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return queue.Stop(shutdownCtx)
}
