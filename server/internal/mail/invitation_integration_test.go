package mail

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/bootstrap"
	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

func TestInvitationWorkerMakesTokenAtDelivery(t *testing.T) {
	ctx := t.Context()
	pool, err := database.Open(ctx, testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = bootstrap.Run(ctx, pool, "Pilot", "admin@example.test", func(_ context.Context, _ pgx.Tx, _, _ string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	var id string
	err = pool.QueryRow(ctx, "INSERT INTO users(email,invitation_generation) VALUES ('member@example.test',2) RETURNING id::text").Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	smtp := newFakeSMTP(t, false)
	host, port, err := net.SplitHostPort(smtp.ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	worker := InvitationWorker{pool: pool, mail: &Worker{cfg: config.Config{
		SMTPHost: host, SMTPPort: port, SMTPFrom: "PMS <noreply@example.test>", PublicURL: "https://pilot.example.test",
	}}}
	job := func(generation int64) *river.Job[InvitationArgs] {
		return &river.Job[InvitationArgs]{Args: InvitationArgs{UserID: id, Generation: generation}}
	}
	if err := worker.Work(ctx, job(1)); err != nil {
		t.Fatal(err)
	}
	if err := worker.Work(ctx, job(2)); err != nil {
		t.Fatal(err)
	}
	smtp.mu.Lock()
	defer smtp.mu.Unlock()
	if len(smtp.messages) != 1 {
		t.Fatalf("SMTP deliveries = %d", len(smtp.messages))
	}
	plain := messageParts(t, smtp.messages[0])["text/plain"]
	const prefix = "https://pilot.example.test/invitations/"
	_, tail, ok := strings.Cut(plain, prefix)
	if !ok || len(strings.Fields(tail)) == 0 {
		t.Fatal("invite mail missing activation link")
	}
	token := strings.Fields(tail)[0]
	var matches bool
	err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_tokens WHERE user_id=$1 AND purpose='invite'
AND token_hash=$2 AND expires_at>now())`, id, security.TokenHash(token)).Scan(&matches)
	if err != nil || !matches {
		t.Fatalf("delivered invite not stored by hash: %v", err)
	}
}
