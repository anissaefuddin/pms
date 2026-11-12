package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/bootstrap"
	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/httpserver"
	mailqueue "github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
)

const publicURL = "http://pms.test"

// TestAuthContract exercises the M0 auth endpoints against api/openapi.yaml (paths, status codes, payloads).
func TestAuthContract(t *testing.T) {
	dbURL := testdb.New(t)
	// Each run owns its collectors, including gauge callbacks that capture its pool.
	registerer, gatherer := prometheus.DefaultRegisterer, prometheus.DefaultGatherer
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer, prometheus.DefaultGatherer = registry, registry
	t.Cleanup(func() {
		prometheus.DefaultRegisterer, prometheus.DefaultGatherer = registerer, gatherer
	})
	ctx := context.Background()
	pool, err := database.Open(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	cfg := config.Config{DatabaseURL: dbURL, PublicURL: publicURL, MaxSeats: 3}
	queue, err := mailqueue.New(pool, cfg)
	if err != nil {
		t.Fatal(err)
	}
	boot, err := bootstrap.Run(ctx, pool, "Test Account", "admin@example.com", func(ctx context.Context, tx pgx.Tx, to, token string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(httpserver.New(pool, cfg, queue))
	defer srv.Close()

	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	forwarded, pinIP := 0, ""
	do := func(method, path string, body any, origin bool) (*http.Response, map[string]any) {
		t.Helper()
		var rdr bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rdr = *bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, srv.URL+"/api/v1"+path, &rdr)
		req.Header.Set("Content-Type", "application/json")
		if origin {
			req.Header.Set("Origin", publicURL)
		}
		// The token endpoints are rate limited per client IP. Pretend each call comes from its own address (as behind a
		// proxy) so one long scenario does not trip the limiter; pinIP fixes the address for the limiter test itself.
		forwarded++
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.1.%d.%d", forwarded/250, forwarded%250))
		if pinIP != "" {
			req.Header.Set("X-Forwarded-For", pinIP)
		}
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp, out
	}
	expect := func(name string, resp *http.Response, out map[string]any, status int, code string) {
		t.Helper()
		if resp.StatusCode != status {
			t.Fatalf("%s: status %d, want %d (%v)", name, resp.StatusCode, status, out)
		}
		if code != "" {
			if out["code"] != code || resp.Header.Get("Content-Type") != "application/problem+json" || out["request_id"] == "" {
				t.Fatalf("%s: not a problem+json with code %q: %v", name, code, out)
			}
		}
	}

	resp, out := do("GET", "/me", nil, false)
	expect("me without session", resp, out, 401, "unauthenticated")

	resp, out = do("POST", "/auth/login", map[string]string{"email": "admin@example.com", "password": "x"}, false)
	expect("login without Origin", resp, out, 403, "forbidden")

	// Bootstrap leaves the administrator `invited`: not able to log in, and counted as a seat.
	var adminStatus string
	if err := pool.QueryRow(ctx, "SELECT status::text FROM users WHERE email='admin@example.com'").Scan(&adminStatus); err != nil || adminStatus != "invited" {
		t.Fatalf("bootstrapped admin status = %q (%v), want invited", adminStatus, err)
	}
	resp, out = do("POST", "/auth/login", map[string]string{"email": "admin@example.com", "password": "correct horse battery"}, true)
	expect("invited admin cannot log in", resp, out, 401, "unauthenticated")
	adminAccept := func(password string) map[string]any {
		return map[string]any{"token": boot.InviteToken, "first_name": "Ada", "last_name": "Admin", "password": password, "accept_terms": true}
	}
	resp, out = do("POST", "/auth/invitations/accept", adminAccept("short"), true)
	expect("admin accept with short password", resp, out, 422, "validation_failed")
	resp, out = do("POST", "/auth/invitations/accept", adminAccept("correct horse battery"), true)
	expect("admin accept", resp, out, 200, "")
	if out["user_type"] != "admin" || out["status"] != "active" || out["first_name"] != "Ada" || out["personal_space_id"] == "" {
		t.Fatalf("admin accept body does not match Me: %v", out)
	}
	resp, out = do("POST", "/auth/invitations/accept", adminAccept("another password 1"), true)
	expect("admin accept token reuse", resp, out, 410, "token_invalid")

	resp, out = do("POST", "/auth/login", map[string]string{"email": "admin@example.com", "password": "wrong password"}, true)
	expect("login wrong password", resp, out, 401, "unauthenticated")
	resp, out = do("POST", "/auth/login", map[string]string{"email": "ADMIN@example.com", "password": "correct horse battery"}, true)
	expect("login", resp, out, 200, "")
	if out["user_type"] != "admin" || out["status"] != "active" || out["personal_space_id"] == "" || out["first_name"] == nil {
		t.Fatalf("login body does not match Me: %v", out)
	}
	var session *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == "pms_session" {
			session = ck
		}
	}
	if session == nil || !session.HttpOnly {
		t.Fatalf("session cookie missing or not HttpOnly: %v", session)
	}
	var createdAt, idleExpiresAt, absoluteExpiresAt time.Time
	if err := pool.QueryRow(ctx, "SELECT created_at,expires_at,absolute_expires_at FROM sessions WHERE token_hash=$1", security.TokenHash(session.Value)).Scan(&createdAt, &idleExpiresAt, &absoluteExpiresAt); err != nil {
		t.Fatal(err)
	}
	// Allow DB clock/request latency and the cookie's whole-second timestamp precision.
	const expiryTolerance = 5 * time.Second
	if lifetime := absoluteExpiresAt.Sub(createdAt); (lifetime - config.AbsoluteSessionTimeout).Abs() > expiryTolerance {
		t.Fatalf("absolute session lifetime = %v, want %v", lifetime, config.AbsoluteSessionTimeout)
	}
	if idle := idleExpiresAt.Sub(createdAt); (idle - config.IdleSessionTimeout).Abs() > expiryTolerance {
		t.Fatalf("idle session lifetime = %v, want %v", idle, config.IdleSessionTimeout)
	}
	if difference := session.Expires.Sub(absoluteExpiresAt); difference.Abs() > expiryTolerance {
		t.Fatalf("cookie expiry %v differs from database absolute expiry %v", session.Expires, absoluteExpiresAt)
	}
	resp, out = do("GET", "/me", nil, false)
	expect("me", resp, out, 200, "")
	if out["capabilities"].(map[string]any)["manage_users"] != true {
		t.Fatalf("admin must manage users: %v", out)
	}

	resp, out = do("POST", "/auth/password/change", map[string]string{"current_password": "nope", "new_password": "new password 123"}, true)
	expect("change wrong current", resp, out, 401, "unauthenticated")
	resp, out = do("POST", "/auth/password/change", map[string]string{"current_password": "correct horse battery", "new_password": "short"}, true)
	expect("change short", resp, out, 422, "validation_failed")
	resp, out = do("POST", "/auth/password/change", map[string]string{"current_password": "correct horse battery", "new_password": "  PASSWORD  "}, true)
	expect("change to a common password", resp, out, 422, "validation_failed")
	// D-14: the caller's session token rotates on change, its absolute expiry does not move.
	srvURL, _ := url.Parse(srv.URL)
	sessionValue := func() string {
		for _, ck := range jar.Cookies(srvURL) {
			if ck.Name == "pms_session" {
				return ck.Value
			}
		}
		return ""
	}
	oldSession := sessionValue()
	var expiresBefore time.Time
	if err := pool.QueryRow(ctx, "SELECT absolute_expires_at FROM sessions WHERE token_hash=$1", security.TokenHash(oldSession)).Scan(&expiresBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) SELECT '\\x0c',id,'reset',now()+interval '1 hour' FROM users WHERE email='admin@example.com'"); err != nil {
		t.Fatal(err)
	}
	resp, out = do("POST", "/auth/password/change", map[string]string{"current_password": "correct horse battery", "new_password": "new password 123"}, true)
	expect("change", resp, out, 204, "")
	var resetLinksLeft int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM user_tokens t JOIN users u ON u.id=t.user_id WHERE u.email='admin@example.com' AND t.purpose='reset'").Scan(&resetLinksLeft); err != nil || resetLinksLeft != 0 {
		t.Fatalf("an emailed reset link survived a password change: %d (%v)", resetLinksLeft, err)
	}
	if newSession := sessionValue(); newSession == "" || newSession == oldSession {
		t.Fatal("change password must set a new session token")
	}
	var expiresAfter time.Time
	if err := pool.QueryRow(ctx, "SELECT absolute_expires_at FROM sessions WHERE token_hash=$1", security.TokenHash(sessionValue())).Scan(&expiresAfter); err != nil || !expiresAfter.Equal(expiresBefore) {
		t.Fatalf("absolute expiry moved: before=%v after=%v err=%v", expiresBefore, expiresAfter, err)
	}
	staleReq, _ := http.NewRequest("GET", srv.URL+"/api/v1/me", nil)
	staleReq.AddCookie(&http.Cookie{Name: "pms_session", Value: oldSession})
	if stale, err := http.DefaultClient.Do(staleReq); err != nil || stale.StatusCode != 401 {
		t.Fatalf("old session token must stop working: %v %v", stale, err)
	}
	resp, out = do("GET", "/me", nil, false)
	expect("own session survives change", resp, out, 200, "")

	// Age rows directly so timeout checks do not depend on sleeps or cookie-jar expiry.
	// expires_at is the sliding idle limit, absolute_expires_at the fixed 30-day limit.
	for _, tc := range []struct {
		name                          string
		idleExpiresIn, absoluteExpiry time.Duration
		status                        int
	}{
		{"within both limits", time.Hour, time.Hour, 200},
		{"idle limit passed", -time.Hour, 24 * time.Hour, 401},
		{"absolute limit passed", -2 * time.Hour, -time.Hour, 401},
	} {
		resp, out = do("POST", "/auth/login", map[string]string{"email": "admin@example.com", "password": "new password 123"}, true)
		expect(tc.name+" login", resp, out, 200, "")
		cookie := resp.Cookies()[0]
		now := time.Now()
		if _, err := pool.Exec(ctx, "UPDATE sessions SET expires_at=$2,absolute_expires_at=$3 WHERE token_hash=$1", security.TokenHash(cookie.Value), now.Add(tc.idleExpiresIn), now.Add(tc.absoluteExpiry)); err != nil {
			t.Fatal(err)
		}
		resp, out = do("GET", "/me", nil, false)
		code := ""
		if tc.status == 401 {
			code = "unauthenticated"
		}
		expect(tc.name, resp, out, tc.status, code)
		if tc.status == 401 {
			var exists bool
			if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash=$1)", security.TokenHash(cookie.Value)).Scan(&exists); err != nil || exists {
				t.Fatalf("%s: expired session remains: exists=%t err=%v", tc.name, exists, err)
			}
			if cookies := resp.Cookies(); len(cookies) != 1 || cookies[0].MaxAge >= 0 {
				t.Fatalf("%s: expired session cookie was not cleared: %v", tc.name, cookies)
			}
		}
	}
	// The idle limit slides forward on activity but never past the absolute limit.
	for _, tc := range []struct {
		name                 string
		absoluteIn, wantIdle time.Duration
	}{
		{"idle limit slides", 30 * 24 * time.Hour, config.IdleSessionTimeout},
		{"idle limit capped by absolute", 2 * 24 * time.Hour, 2 * 24 * time.Hour},
	} {
		resp, out = do("POST", "/auth/login", map[string]string{"email": "admin@example.com", "password": "new password 123"}, true)
		expect(tc.name+" login", resp, out, 200, "")
		cookie := resp.Cookies()[0]
		now := time.Now()
		if _, err := pool.Exec(ctx, "UPDATE sessions SET expires_at=$2,absolute_expires_at=$3 WHERE token_hash=$1", security.TokenHash(cookie.Value), now.Add(time.Hour), now.Add(tc.absoluteIn)); err != nil {
			t.Fatal(err)
		}
		resp, out = do("GET", "/me", nil, false)
		expect(tc.name, resp, out, 200, "")
		var idle time.Time
		if err := pool.QueryRow(ctx, "SELECT expires_at FROM sessions WHERE token_hash=$1", security.TokenHash(cookie.Value)).Scan(&idle); err != nil {
			t.Fatal(err)
		}
		if got := idle.Sub(now); (got - tc.wantIdle).Abs() > 10*time.Second {
			t.Fatalf("%s: idle limit is now %v ahead, want about %v", tc.name, got, tc.wantIdle)
		}
		// Once slid (or capped), further requests must not write to the sessions table again.
		var rowBefore, rowAfter string
		_ = pool.QueryRow(ctx, "SELECT xmin::text FROM sessions WHERE token_hash=$1", security.TokenHash(cookie.Value)).Scan(&rowBefore)
		resp, out = do("GET", "/me", nil, false)
		expect(tc.name+" second request", resp, out, 200, "")
		_ = pool.QueryRow(ctx, "SELECT xmin::text FROM sessions WHERE token_hash=$1", security.TokenHash(cookie.Value)).Scan(&rowAfter)
		if rowBefore == "" || rowBefore != rowAfter {
			t.Fatalf("%s: a second request rewrote the session row (xmin %q -> %q)", tc.name, rowBefore, rowAfter)
		}
	}

	// Only failed logins are limited: many correct logins from one IP (an office) must keep working.
	var adminSessions []*http.Cookie
	for i := 0; i < 12; i++ {
		resp, out = do("POST", "/auth/login", map[string]string{"email": "admin@example.com", "password": "new password 123"}, true)
		expect("repeated successful login", resp, out, 200, "")
		if i < 2 {
			adminSessions = append(adminSessions, resp.Cookies()[0])
		}
	}
	// Five failures lock that email for a while, even for the right password; other emails are unaffected.
	for i := 0; i < 5; i++ {
		resp, out = do("POST", "/auth/login", map[string]string{"email": "victim@example.com", "password": "wrong password"}, true)
		expect("failed login", resp, out, 401, "unauthenticated")
	}
	resp, out = do("POST", "/auth/login", map[string]string{"email": "Victim@example.com", "password": "wrong password"}, true)
	expect("locked email", resp, out, 429, "rate_limited")
	resp, out = do("POST", "/auth/login", map[string]string{"email": "admin@example.com", "password": "new password 123"}, true)
	expect("other email still works", resp, out, 200, "")
	// An unknown email must cost about as much time as a known one (argon2 runs either way).
	start := time.Now()
	do("POST", "/auth/login", map[string]string{"email": "ghost@example.com", "password": "wrong password"}, true)
	if took := time.Since(start); took < 5*time.Millisecond {
		t.Fatalf("unknown email answered in %v: login timing reveals which emails exist", took)
	}

	resp, out = do("POST", "/auth/password/forgot", map[string]string{"email": "admin@example.com"}, true)
	expect("forgot known", resp, out, 202, "")
	resp, out = do("POST", "/auth/password/forgot", map[string]string{"email": "nobody@example.com"}, true)
	expect("forgot unknown looks identical", resp, out, 202, "")

	// inviteUser stands in for the admin invite endpoint (S1.6.1): an `invited` user plus a single-use token.
	inviteUser := func(email string, validFor time.Duration) string {
		t.Helper()
		plain, hash, _ := security.Token()
		var id string
		if err := pool.QueryRow(ctx, "INSERT INTO users(email,user_type,status) VALUES ($1,'regular','invited') RETURNING id::text", email).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) VALUES ($1,$2,'invite',$3)", hash, id, time.Now().Add(validFor)); err != nil {
			t.Fatal(err)
		}
		return plain
	}
	invite := inviteUser("new@example.com", time.Hour)
	resp, out = do("GET", "/auth/invitations/bogus", nil, false)
	expect("invitation bad token", resp, out, 410, "token_invalid")
	logs := captureLogs(t)
	resp, out = do("GET", "/auth/invitations/"+url.PathEscape(invite), nil, false)
	expect("invitation info", resp, out, 200, "")
	if strings.Contains(logs.String(), invite) {
		t.Fatal("invitation token was written to the request log")
	}
	if out["email"] != "new@example.com" || out["expires_at"] == nil {
		t.Fatalf("invitation info: %v", out)
	}
	// A token only works for its own purpose: an invitation cannot reset a password and a reset link cannot accept an invitation.
	resp, out = do("POST", "/auth/password/reset", map[string]string{"token": invite, "password": "cross purpose 123"}, true)
	expect("invite token used as reset token", resp, out, 410, "token_invalid")
	crossReset, crossHash, _ := security.Token()
	if _, err := pool.Exec(ctx, "INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) SELECT $1,id,'reset',now()+interval '1 hour' FROM users WHERE email='new@example.com'", crossHash); err != nil {
		t.Fatal(err)
	}
	resp, out = do("POST", "/auth/invitations/accept", map[string]any{"token": crossReset, "first_name": "No", "last_name": "Way", "password": "cross purpose 123", "accept_terms": true}, true)
	expect("reset token used as invitation", resp, out, 410, "token_invalid")
	resp, out = do("GET", "/auth/invitations/"+url.PathEscape(crossReset), nil, false)
	expect("reset token used to read an invitation", resp, out, 410, "token_invalid")
	// A stale invitation token of an already active user must neither reset that user's password nor re-activate the account.
	staleInvite, staleHash, _ := security.Token()
	if _, err := pool.Exec(ctx, "INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) SELECT $1,id,'invite',now()+interval '1 hour' FROM users WHERE email='admin@example.com'", staleHash); err != nil {
		t.Fatal(err)
	}
	resp, out = do("POST", "/auth/password/reset", map[string]string{"token": staleInvite, "password": "stale invite 123"}, true)
	expect("active user's invite token used to reset", resp, out, 410, "token_invalid")
	resp, out = do("POST", "/auth/invitations/accept", map[string]any{"token": staleInvite, "first_name": "Evil", "last_name": "Twin", "password": "stale invite 123", "accept_terms": true}, true)
	expect("active user's invite token used to accept", resp, out, 410, "token_invalid")
	var adminFirstName string
	if err := pool.QueryRow(ctx, "SELECT first_name FROM users WHERE email='admin@example.com'").Scan(&adminFirstName); err != nil || adminFirstName != "Ada" {
		t.Fatalf("admin was overwritten through a stale invitation: first_name=%q (%v)", adminFirstName, err)
	}
	var staleLeft int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM user_tokens WHERE token_hash=$1 AND used_at IS NULL", staleHash).Scan(&staleLeft); err != nil || staleLeft != 1 {
		t.Fatalf("refused requests must not burn the token: unused=%d (%v)", staleLeft, err)
	}
	accept := map[string]any{"token": invite, "first_name": "Nia", "last_name": "Putri", "password": "invited password 1", "accept_terms": true}
	resp, out = do("POST", "/auth/invitations/accept", map[string]any{"token": invite, "first_name": "Nia", "last_name": "Putri", "password": "invited password 1", "accept_terms": false}, true)
	expect("accept without terms", resp, out, 422, "validation_failed")
	resp, out = do("POST", "/auth/invitations/accept", map[string]any{"token": invite, "first_name": strings.Repeat("N", 81), "last_name": "Putri", "password": "invited password 1", "accept_terms": true}, true)
	expect("accept with a name over 80 characters", resp, out, 422, "validation_failed")
	resp, out = do("POST", "/auth/invitations/accept", map[string]any{"token": invite, "first_name": "Nia", "last_name": "Putri", "password": "密码密码", "accept_terms": true}, true)
	expect("accept with 4 multibyte characters", resp, out, 422, "validation_failed")
	resp, out = do("POST", "/auth/invitations/accept", accept, true)
	expect("accept", resp, out, 200, "")
	if out["email"] != "new@example.com" || out["user_type"] != "regular" || out["last_name"] != "Putri" {
		t.Fatalf("accept body does not match Me: %v", out)
	}
	resp, out = do("POST", "/auth/invitations/accept", accept, true)
	expect("accept token reuse", resp, out, 410, "token_invalid")

	// A stolen session cookie must not allow unlimited guessing of the current password.
	for i := 0; i < 5; i++ {
		resp, out = do("POST", "/auth/password/change", map[string]string{"current_password": "guess number " + fmt.Sprint(i), "new_password": "another new password"}, true)
		expect("wrong current password", resp, out, 401, "unauthenticated")
	}
	resp, out = do("POST", "/auth/password/change", map[string]string{"current_password": "invited password 1", "new_password": "another new password"}, true)
	expect("change password after five failures", resp, out, 429, "rate_limited")

	// Seats are counted as invited + active users and guarded when inviting (S1.6.1). Accepting an invitation only turns
	// an already-counted seat from invited into active, so it must not change the number of seats in use.
	invite2 := inviteUser("second@example.com", time.Hour)
	inviteUser("third@example.com", time.Hour)
	// An invited account cannot log in and gets the same answer as a wrong password.
	resp, wrongInvited := do("POST", "/auth/login", map[string]string{"email": "third@example.com", "password": "whatever password"}, true)
	_, wrongAdmin := do("POST", "/auth/login", map[string]string{"email": "admin@example.com", "password": "whatever password"}, true)
	if resp.StatusCode != 401 || wrongInvited["detail"] != wrongAdmin["detail"] || wrongInvited["code"] != wrongAdmin["code"] {
		t.Fatalf("invited and wrong-password answers differ: %v vs %v", wrongInvited, wrongAdmin)
	}
	// Forgot password creates no token for an invited user.
	resp, out = do("POST", "/auth/password/forgot", map[string]string{"email": "third@example.com"}, true)
	expect("forgot for invited user", resp, out, 202, "")
	var resetTokens int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM user_tokens t JOIN users u ON u.id=t.user_id WHERE u.email='third@example.com' AND t.purpose='reset'").Scan(&resetTokens); err != nil || resetTokens != 0 {
		t.Fatalf("reset tokens for an invited user: %d (%v)", resetTokens, err)
	}
	accept2 := map[string]any{"token": invite2, "first_name": "Sam", "last_name": "Second", "password": "second password 1", "accept_terms": true}
	seatsInUse := func() (n int) {
		t.Helper()
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE status IN ('invited','active')").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	seatsBefore := seatsInUse()
	resp, out = do("POST", "/auth/invitations/accept", accept2, true)
	expect("accept at full capacity", resp, out, 200, "")
	if seatsInUse() != seatsBefore {
		t.Fatalf("accepting an invitation changed seats in use: %d -> %d", seatsBefore, seatsInUse())
	}

	otherUserSession := resp.Cookies()[0]
	serverURL, _ := url.Parse(srv.URL)
	for _, cookie := range adminSessions {
		jar.SetCookies(serverURL, []*http.Cookie{cookie})
		resp, out = do("GET", "/me", nil, false)
		expect("admin session before reset", resp, out, 200, "")
	}
	resetToken, resetHash, err := security.Token()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) SELECT $1,id,'reset',$2 FROM users WHERE email='admin@example.com'`, resetHash, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	resp, out = do("POST", "/auth/password/reset", map[string]string{"token": resetToken, "password": "reset password 456"}, true)
	expect("expired reset token", resp, out, 410, "token_invalid")
	resp, out = do("GET", "/me", nil, false)
	expect("expired reset leaves session valid", resp, out, 200, "")
	resp, out = do("POST", "/auth/login", map[string]string{"email": "admin@example.com", "password": "new password 123"}, true)
	expect("expired reset leaves password unchanged", resp, out, 200, "")
	if _, err := pool.Exec(ctx, "UPDATE user_tokens SET expires_at=$2 WHERE token_hash=$1", resetHash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	secondLink, secondHash, _ := security.Token()
	if _, err := pool.Exec(ctx, "INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) SELECT $1,id,'reset',now()+interval '1 hour' FROM users WHERE email='admin@example.com'", secondHash); err != nil {
		t.Fatal(err)
	}
	resp, out = do("POST", "/auth/password/reset", map[string]string{"token": resetToken, "password": "reset password 456"}, true)
	expect("reset with active sessions", resp, out, 204, "")
	resp, out = do("POST", "/auth/password/reset", map[string]string{"token": secondLink, "password": "second link 789"}, true)
	expect("older reset link after a reset", resp, out, 410, "token_invalid")
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sessions s JOIN users u ON u.id=s.user_id WHERE u.email='admin@example.com'`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("reset must revoke all admin sessions: remaining=%d err=%v", remaining, err)
	}
	resp, out = do("GET", "/me", nil, false)
	expect("reset revokes caller session", resp, out, 401, "unauthenticated")
	for _, cookie := range adminSessions {
		jar.SetCookies(serverURL, []*http.Cookie{cookie})
		resp, out = do("GET", "/me", nil, false)
		expect("reset revokes other admin session", resp, out, 401, "unauthenticated")
	}
	jar.SetCookies(serverURL, []*http.Cookie{otherUserSession})
	resp, out = do("GET", "/me", nil, false)
	expect("reset preserves another user's session", resp, out, 200, "")

	resp, out = do("POST", "/auth/logout", nil, true)
	expect("logout", resp, out, 204, "")
	resp, out = do("GET", "/me", nil, false)
	expect("me after logout", resp, out, 401, "unauthenticated")

	for _, path := range []string{"/healthz", "/readyz"} {
		resp, out = do("GET", path, nil, false)
		expect(path, resp, out, 200, "")
	}
	// ---- S0.2.2 / S0.2.3 / S0.1.2 closeout ----
	var lastLoginBody []byte
	login := func(c *http.Client, email, password string) *http.Response {
		b, _ := json.Marshal(map[string]string{"email": email, "password": password})
		req, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/login", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", publicURL)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		lastLoginBody, _ = io.ReadAll(resp.Body)
		return resp
	}
	mustLogin := func(c *http.Client, email, password string) {
		t.Helper()
		if r := login(c, email, password); r.StatusCode != 200 {
			t.Fatalf("login %s: status %d, want 200: %s", email, r.StatusCode, lastLoginBody)
		}
	}
	meStatus := func(c *http.Client) int {
		resp, err := c.Get(srv.URL + "/api/v1/me")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	newClient := func() *http.Client { j, _ := cookiejar.New(nil); return &http.Client{Jar: j} }
	const adminPassword = "reset password 456"

	// Changing the password revokes the user's other sessions but keeps the current one.
	a, b := newClient(), newClient()
	mustLogin(a, "admin@example.com", adminPassword)
	mustLogin(b, "admin@example.com", adminPassword)
	changeBody, _ := json.Marshal(map[string]string{"current_password": adminPassword, "new_password": "changed password 9"})
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/password/change", bytes.NewReader(changeBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", publicURL)
	changeResp, err := a.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	changeResp.Body.Close()
	if changeResp.StatusCode != 204 {
		t.Fatalf("change password: status %d, want 204", changeResp.StatusCode)
	}
	if meStatus(a) != 200 || meStatus(b) != 401 {
		t.Fatal("change password must keep the caller's session and revoke the others")
	}

	// A deactivated account gets exactly the same answer as a wrong password.
	if _, err := pool.Exec(ctx, "UPDATE users SET status='deactivated' WHERE email='new@example.com'"); err != nil {
		t.Fatal(err)
	}
	resp, wrong := do("POST", "/auth/login", map[string]string{"email": "admin@example.com", "password": "wrong password"}, true)
	_, gone := do("POST", "/auth/login", map[string]string{"email": "new@example.com", "password": "invited password 1"}, true)
	if resp.StatusCode != 401 || gone["status"] != float64(401) || gone["detail"] != wrong["detail"] || gone["code"] != wrong["code"] {
		t.Fatalf("deactivated and wrong-password answers differ: %v vs %v", gone, wrong)
	}

	// A reset link issued before deactivation must not change the password of a deactivated account.
	var hashBefore, hashAfter string
	_ = pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE email='new@example.com'").Scan(&hashBefore)
	deadLink, deadHash, _ := security.Token()
	if _, err := pool.Exec(ctx, "INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) SELECT $1,id,'reset',now()+interval '1 hour' FROM users WHERE email='new@example.com'", deadHash); err != nil {
		t.Fatal(err)
	}
	resp, out = do("POST", "/auth/password/reset", map[string]string{"token": deadLink, "password": "deactivated 456"}, true)
	expect("reset for a deactivated user", resp, out, 410, "token_invalid")
	_ = pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE email='new@example.com'").Scan(&hashAfter)
	if hashBefore == "" || hashBefore != hashAfter {
		t.Fatal("a reset link changed the password of a deactivated user")
	}

	// Seat checks above deleted nothing from sessions of the admin; now a missing migration must make /readyz fail.
	if _, err := pool.Exec(ctx, "DELETE FROM schema_migrations WHERE version_id=1"); err != nil {
		t.Fatal(err)
	}
	resp, out = do("GET", "/readyz", nil, false)
	expect("readyz with a migration missing", resp, out, 503, "")
	if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations(version_id,is_applied) VALUES (1,true)"); err != nil {
		t.Fatal(err)
	}
	resp, out = do("GET", "/readyz", nil, false)
	expect("readyz after migrations restored", resp, out, 200, "")

	// Email jobs retry at most 8 times.
	var attempts int
	if err := pool.QueryRow(ctx, "SELECT max_attempts FROM river_job WHERE kind='send_email' LIMIT 1").Scan(&attempts); err != nil || attempts != 8 {
		t.Fatalf("send_email max_attempts = %d (%v), want 8", attempts, err)
	}

	// Metrics must not be reachable on the public router (the SPA fallback answers instead), only on their own handler.
	pub, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	pubBody := new(bytes.Buffer)
	_, _ = pubBody.ReadFrom(pub.Body)
	pub.Body.Close()
	if strings.Contains(pubBody.String(), "pms_http_requests_total") {
		t.Fatal("/metrics is exposed on the public router")
	}
	internal := httptest.NewRecorder()
	httpserver.Metrics().ServeHTTP(internal, httptest.NewRequest("GET", "/metrics", nil))
	for _, name := range []string{"pms_http_requests_total", "pms_http_request_duration_seconds", "pms_jobs_available", "pms_jobs_failed_total", "pms_db_pool_acquired_conns", "pms_db_pool_max_conns", "pms_db_pool_empty_acquire_total"} {
		if !strings.Contains(internal.Body.String(), name) {
			t.Fatalf("internal metrics handler does not serve %s", name)
		}
	}

	// Bogus tokens must be rate limited so they cannot be used to burn CPU and memory.
	pinIP = "10.9.9.9"
	var limited bool
	for i := 0; i < 15 && !limited; i++ {
		resp, _ = do("POST", "/auth/password/reset", map[string]string{"token": "bogus", "password": "valid password 1"}, true)
		limited = resp.StatusCode == 429
	}
	if !limited {
		t.Fatal("reset endpoint is not rate limited")
	}
	resp, out = do("GET", "/nope", nil, false)
	if resp.StatusCode == 500 {
		t.Fatalf("unknown path: %v", out)
	}
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}
