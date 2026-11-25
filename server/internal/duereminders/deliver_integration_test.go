package duereminders

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/bootstrap"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/jackc/pgx/v5"
)

// DEF-002: reminders for several users and tasks are really queued, once each; a failing user does not stop the others; the Space
// named is one the recipient can see (DEF-004).
func TestDeliverQueuesEachReminderOnce(t *testing.T) {
	ctx := t.Context()
	pool, err := database.Open(ctx, testdb.NewMigrated(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	boot, err := bootstrap.Run(ctx, pool, "Pilot", "admin@example.test", func(context.Context, pgx.Tx, string, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 23, 8, 5, 0, 0, time.UTC)
	for _, stmt := range []string{
		`UPDATE users SET status='deactivated' WHERE id<>$1`,
		`INSERT INTO users(email,first_name,last_name,password_hash,user_type,status,timezone,due_reminders) SELECT e,'A','B','x','regular','active','UTC',jsonb_build_object('enabled',true,'days_before','[3,1,0]'::jsonb,'overdue',true,'overdue_every_days',3,'delivery',d,'hour',8)
FROM (VALUES ('one@example.test','individual'),('two@example.test','digest'),('bad@example.test','digest')) v(e,d) WHERE $1::uuid IS NOT NULL`,
		`INSERT INTO spaces(name,kind) SELECT n,'shared' FROM unnest(ARRAY['Hidden','Open']) n WHERE $1::uuid IS NOT NULL`,
		`INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) SELECT 'space',s.id,'user',u.id,'read' FROM spaces s, users u WHERE s.name='Open' AND u.user_type='regular' AND u.email LIKE '%@example.test' AND $1::uuid IS NOT NULL`,
		`INSERT INTO items(type,title,created_by,due_date) SELECT 'task','Due '||d,$1,date '2026-11-23'+d FROM unnest(ARRAY[3,1,0,-1,2]) d`,
		`INSERT INTO items(type,title,created_by,due_date) VALUES ('task','Unseen',$1,'2026-11-23')`,
		`INSERT INTO item_locations(item_id,space_id,rank) SELECT i.id,s.id,CASE s.name WHEN 'Hidden' THEN 'A' ELSE 'B' END FROM items i, spaces s WHERE s.name IN ('Hidden','Open') AND (i.title<>'Unseen' OR s.name='Hidden') AND $1::uuid IS NOT NULL`,
		`INSERT INTO item_assignees(item_id,user_id) SELECT i.id,u.id FROM items i, users u WHERE u.email IN ('one@example.test','two@example.test','bad@example.test') AND $1::uuid IS NOT NULL`,
	} {
		if _, err := pool.Exec(ctx, stmt, boot.AdminID); err != nil {
			t.Fatalf("%v\n%s", err, stmt)
		}
	}
	var sent []mail.Args
	fail := true
	run := func() error {
		return Deliver(ctx, pool, "https://lens.test", now, func(_ context.Context, _ pgx.Tx, a mail.Args) error {
			if fail && a.To == "bad@example.test" {
				return errors.New("queue refused")
			}
			sent = append(sent, a)
			return nil
		})
	}
	if err := run(); err == nil {
		t.Fatal("a failed user must be reported so the job retries")
	}
	perUser := map[string]int{}
	for _, a := range sent {
		perUser[a.To]++
		if a.Data.Space == "Hidden" || a.Data.Item == "Unseen" {
			t.Fatalf("reminder discloses what the recipient cannot see: %+v", a)
		}
		if a.To == "one@example.test" && a.Data.Space != "Open" {
			t.Fatalf("Space = %q, want Open", a.Data.Space)
		}
	}
	if perUser["one@example.test"] != 4 || perUser["two@example.test"] != 1 || len(sent) != 5 {
		t.Fatalf("queued = %v, want 4 individual and 1 digest", perUser)
	}
	fail = false
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 6 || sent[5].To != "bad@example.test" {
		t.Fatalf("the retry sends only what failed: %d", len(sent))
	}
	if err := run(); err != nil || len(sent) != 6 {
		t.Fatalf("a third run sends nothing: %d (%v)", len(sent), err)
	}
}
