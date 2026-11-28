package mail

import (
	"context"
	"net"
	"strings"
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
	pool, err := database.Open(ctx, testdb.NewMigrated(t))
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
	if err := pool.QueryRow(ctx, "INSERT INTO spaces(name,kind,scope_state) VALUES ('Shared','shared','internal') RETURNING id::text").Scan(&space); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "INSERT INTO items(type,title,created_by,scope_state) VALUES ('task','T',$1,'internal') RETURNING id::text", boot.AdminID).Scan(&item); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'A')", item, space); err != nil {
		t.Fatal(err)
	}
	// A1.3: a notification is delivered only to a recipient who can view the item; the recipient is an explicit member here.
	if _, err := pool.Exec(ctx, "INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('space',$1,'user',$2,'full')", space, boot.AdminID); err != nil {
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
	send("approval_reminder")
	send("approval_escalated")
	if sent() != 9 {
		t.Fatalf("task_changes must still send: %d", sent())
	}
	mute("task_changes")
	send("status_changed")
	send("approval_requested")
	send("approval_reminder")
	send("approval_escalated")
	send("comment")
	if sent() != 9 {
		t.Fatalf("task_changes muted or comment emailed: %d", sent())
	}
}

// A1.7 regression: a notification about an item the recipient cannot view is never delivered, so notifications cannot leak
// content across Spaces. The Workspace admin recipient has no grant on the item's Space here.
func TestNotificationNeverLeaksAnItemTheRecipientCannotView(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.NewMigrated(t))
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
	if err := pool.QueryRow(ctx, "INSERT INTO spaces(name,kind,scope_state) VALUES ('Private','shared','internal') RETURNING id::text").Scan(&space); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "INSERT INTO items(type,title,created_by,scope_state) VALUES ('task','Secret',$1,'internal') RETURNING id::text", boot.AdminID).Scan(&item); err != nil {
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
	var id string
	if err := pool.QueryRow(ctx, "INSERT INTO notifications(id,recipient_id,kind,item_id) VALUES (gen_random_uuid(),$1,'mention',$2) RETURNING id::text", boot.AdminID, item).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err := worker.Work(ctx, &river.Job[NotificationArgs]{Args: NotificationArgs{ID: id}}); err != nil {
		t.Fatal(err)
	}
	smtp.mu.Lock()
	defer smtp.mu.Unlock()
	if n := len(smtp.messages); n != 0 {
		t.Fatalf("a recipient without access received %d emails about a private item", n)
	}
}

// DEF-004: the email names a Space the recipient can see, never another Space of a multi-location item, and none when there is none.
func TestNotificationNamesOnlyASpaceTheRecipientCanSee(t *testing.T) {
	ctx := t.Context()
	pool, err := database.Open(ctx, testdb.NewMigrated(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	boot, err := bootstrap.Run(ctx, pool, "Pilot", "admin@example.test", func(_ context.Context, _ pgx.Tx, _, _ string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	var id string
	if err := pool.QueryRow(ctx, `WITH u AS (UPDATE users SET status='active',first_name='A',last_name='B',password_hash='x' WHERE id=$1 RETURNING id),
hidden AS (INSERT INTO spaces(name,kind,scope_state) VALUES ('SecretClient','shared','internal') RETURNING id),
open AS (INSERT INTO spaces(name,kind,scope_state) VALUES ('OpenSpace','shared','internal') RETURNING id),
i AS (INSERT INTO items(type,title,created_by,scope_state) VALUES ('task','T',$1,'internal') RETURNING id),
l AS (INSERT INTO item_locations(item_id,space_id,rank) SELECT i.id,hidden.id,'A' FROM i,hidden UNION ALL SELECT i.id,open.id,'B' FROM i,open),
s AS (INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) SELECT 'space',id,'user',$1,'read' FROM open)
INSERT INTO notifications(id,recipient_id,kind,item_id) SELECT gen_random_uuid(),$1,'assigned',id FROM i RETURNING id::text`, boot.AdminID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	smtp := newFakeSMTP(t, false)
	host, port, err := net.SplitHostPort(smtp.ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	worker := NotificationWorker{pool: pool, mail: &Worker{cfg: config.Config{SMTPHost: host, SMTPPort: port, SMTPFrom: "PMS <noreply@example.test>", PublicURL: "https://pilot.example.test"}}}
	send := func() string {
		t.Helper()
		if err := worker.Work(ctx, &river.Job[NotificationArgs]{Args: NotificationArgs{ID: id}}); err != nil {
			t.Fatal(err)
		}
		smtp.mu.Lock()
		defer smtp.mu.Unlock()
		return smtp.messages[len(smtp.messages)-1]
	}
	if msg := send(); !strings.Contains(msg, "OpenSpace") || strings.Contains(msg, "SecretClient") {
		t.Fatalf("the email must name OpenSpace only: %s", msg)
	}
	// Access through the item alone shows no Space at all.
	if _, err := pool.Exec(ctx, "UPDATE shares SET target_type='item', target_id=(SELECT item_id FROM notifications WHERE id=$1) WHERE target_id=(SELECT id FROM spaces WHERE name='OpenSpace')", id); err != nil {
		t.Fatal(err)
	}
	if msg := send(); strings.Contains(msg, "OpenSpace") || strings.Contains(msg, "SecretClient") {
		t.Fatalf("item-only access must not name a Space: %s", msg)
	}
}
