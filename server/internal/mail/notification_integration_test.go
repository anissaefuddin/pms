package mail

import (
	"context"
	"net"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/bootstrap"
	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

// SET-05: a muted group sends no email, every other group still does, and comments are never emailed.
func TestNotificationWorkerHonoursEmailPreferences(t *testing.T) {
	ctx := t.Context()
	pool, err := database.Open(ctx, testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	boot, err := bootstrap.Run(ctx, pool, "Pilot", "admin@example.test", func(_ context.Context, _ pgx.Tx, _, _ string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET status='active',first_name='A',last_name='B',password_hash='x' WHERE id=$1", boot.AdminID); err != nil {
		t.Fatal(err)
	}
	var space, item string
	if err := pool.QueryRow(ctx, "INSERT INTO spaces(name,kind) VALUES ('Shared','shared') RETURNING id::text").Scan(&space); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "INSERT INTO items(type,title,created_by) VALUES ('task','T',$1) RETURNING id::text", boot.AdminID).Scan(&item); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", item, space); err != nil {
		t.Fatal(err)
	}
	smtp := newFakeSMTP(t, false)
	host, port, err := net.SplitHostPort(smtp.ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	worker := NotificationWorker{pool: pool, mail: &Worker{cfg: config.Config{SMTPHost: host, SMTPPort: port, SMTPFrom: "PMS <noreply@example.test>", PublicURL: "https://pilot.example.test"}}}
	send := func(kind string) {
		t.Helper()
		var id string
		if err := pool.QueryRow(ctx, "INSERT INTO notifications(id,recipient_id,kind,item_id) VALUES (gen_random_uuid(),$1,$2,$3) RETURNING id::text", boot.AdminID, kind, item).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if err := worker.Work(ctx, &river.Job[NotificationArgs]{Args: NotificationArgs{ID: id}}); err != nil {
			t.Fatal(err)
		}
	}
	sent := func() int {
		smtp.mu.Lock()
		defer smtp.mu.Unlock()
		return len(smtp.messages)
	}
	mute := func(groups ...string) {
		t.Helper()
		if _, err := pool.Exec(ctx, "UPDATE users SET email_muted=$2 WHERE id=$1", boot.AdminID, groups); err != nil {
			t.Fatal(err)
		}
	}
	for _, kind := range []string{"mention", "assigned", "shared", "status_changed", "approval_requested"} {
		send(kind)
	}
	if sent() != 5 {
		t.Fatalf("all on: %d emails, want 5", sent())
	}
	mute("mention", "assigned", "shared", "report")
	for _, kind := range []string{"mention", "assigned", "shared"} {
		send(kind)
	}
	if sent() != 5 {
		t.Fatalf("muted groups still emailed: %d", sent())
	}
	send("status_changed")
	send("approval_decided")
	if sent() != 7 {
		t.Fatalf("task_changes must still send: %d", sent())
	}
	mute("task_changes")
	send("status_changed")
	send("approval_requested")
	send("comment")
	if sent() != 7 {
		t.Fatalf("task_changes muted or comment emailed: %d", sent())
	}
}
