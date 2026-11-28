package mail

import (
	"net"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/testdb"
)

// PROMPT #026 Workstream B: a report digest queued while the subscriber could read the report is not sent after their Customer access
// ends. The fake SMTP server counts what really leaves, so the test measures delivery, not only the queue.
func TestQueuedReportDigestIsNotSentAfterAccessEnds(t *testing.T) {
	ctx := t.Context()
	pool, err := database.Open(ctx, testdb.NewMigrated(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	one := func(sql string, args ...any) string {
		t.Helper()
		var id string
		if err := pool.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	admin := one(`INSERT INTO users(email,user_type,first_name,last_name,password_hash,status) VALUES ('admin@mail.test','admin','A','B','x','active') RETURNING id::text`)
	member := one(`INSERT INTO users(email,user_type,first_name,last_name,password_hash,status) VALUES ('member@mail.test','regular','M','B','x','active') RETURNING id::text`)
	client := one(`INSERT INTO clients(code,name,created_by) VALUES ('DIGEST-MAIL','Customer',$1) RETURNING id::text`, admin)
	if _, err := pool.Exec(ctx, `INSERT INTO client_memberships(client_id,user_id,status,granted_by) VALUES ($1,$2,'active',$3),($1,$3,'active',$3)`, client, member, admin); err != nil {
		t.Fatal(err)
	}
	space := one(`INSERT INTO spaces(name,kind,scope_state,client_id) VALUES ('Mail client','shared','client',$1) RETURNING id::text`, client)
	if _, err := pool.Exec(ctx, `INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('space',$1,'user',$2,'read')`, space, member); err != nil {
		t.Fatal(err)
	}
	report := one(`INSERT INTO reports(owner_id,space_id,name,definition,sharing) VALUES ($1,$2,'Client time','{"layout":"table","query":{}}','space') RETURNING id::text`, admin, space)

	smtp := newFakeSMTP(t, false)
	host, port, err := net.SplitHostPort(smtp.ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	worker := &Worker{pool: pool, cfg: config.Config{SMTPHost: host, SMTPPort: port, SMTPFrom: "PMS <noreply@example.test>", PublicURL: "https://pms.example.test"}}
	digest := Args{To: "member@mail.test", Template: "report_digest", Subject: "Client time", HTML: "<p>summary</p>", ReportID: report, RecipientID: member}
	sent := func() int {
		smtp.mu.Lock()
		defer smtp.mu.Unlock()
		return len(smtp.messages)
	}

	// Positive control: while the member can read the Space, the queued digest is delivered.
	if err := worker.deliver(ctx, digest); err != nil {
		t.Fatal(err)
	}
	if n := sent(); n != 1 {
		t.Fatalf("the digest must be delivered while the subscriber can read the report, got %d", n)
	}

	// The membership is revoked; the same queued digest must not leave now.
	if _, err := pool.Exec(ctx, `UPDATE client_memberships SET status='revoked', revoked_at=now(), revoked_by=$3 WHERE client_id=$1 AND user_id=$2`, client, member, admin); err != nil {
		t.Fatal(err)
	}
	if err := worker.deliver(ctx, digest); err != nil {
		t.Fatal(err)
	}
	if n := sent(); n != 1 {
		t.Fatalf("a digest queued before revocation was sent after it: %d messages", n)
	}
}
