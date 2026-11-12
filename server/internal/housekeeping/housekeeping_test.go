package housekeeping_test

import (
	"context"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/bootstrap"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/housekeeping"
	"github.com/anissaefuddin/pms/server/internal/testdb"
)

func TestRunKeepsLiveRowsAndRemovesDeadOnes(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	boot, err := bootstrap.Run(ctx, pool, "Acme", "admin@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	// sessions: live, idle limit passed, absolute limit passed
	exec("INSERT INTO sessions(token_hash,user_id,expires_at,absolute_expires_at) VALUES ('\\x01',$1,now()+interval '1 day',now()+interval '20 days')", boot.AdminID)
	exec("INSERT INTO sessions(token_hash,user_id,expires_at,absolute_expires_at) VALUES ('\\x02',$1,now()-interval '1 minute',now()+interval '20 days')", boot.AdminID)
	exec("INSERT INTO sessions(token_hash,user_id,expires_at,absolute_expires_at) VALUES ('\\x03',$1,now()-interval '2 minutes',now()-interval '1 minute')", boot.AdminID)
	// tokens: bootstrap's own invite (live), an expired reset, a used reset, a live reset
	exec("INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) VALUES ('\\x11',$1,'reset',now()-interval '1 hour')", boot.AdminID)
	exec("INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at,used_at) VALUES ('\\x12',$1,'reset',now()+interval '1 hour',now())", boot.AdminID)
	exec("INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) VALUES ('\\x13',$1,'reset',now()+interval '1 hour')", boot.AdminID)
	// an invited user whose invitation expired keeps holding a seat
	exec("INSERT INTO users(email,user_type,status) VALUES ('late@example.com','regular','invited')")
	exec("INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) SELECT '\\x21',id,'invite',now()-interval '1 day' FROM users WHERE email='late@example.com'")

	if err := housekeeping.Run(ctx, pool); err != nil {
		t.Fatal(err)
	}
	count := func(table string) (n int) {
		t.Helper()
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	// kept: 1 session; tokens: bootstrap invite + live reset; users: admin + late
	if got := count("sessions"); got != 1 {
		t.Fatalf("sessions left: %d, want 1", got)
	}
	if got := count("user_tokens"); got != 2 {
		t.Fatalf("tokens left: %d, want 2", got)
	}
	if got := count("users"); got != 2 {
		t.Fatalf("users left: %d, want 2 (invited users must keep their seat)", got)
	}
}
