package httpserver_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/anissaefuddin/pms/server/internal/twofactor"
)

const twoFactorPassword = "Correct-Horse-9"

// withPassword gives a fixture user a real password hash so the login endpoints can be used.
func (f *fixture) withPassword(p *person) *person {
	f.t.Helper()
	hash, err := security.HashPassword(twoFactorPassword)
	if err != nil {
		f.t.Fatal(err)
	}
	f.exec("UPDATE users SET password_hash=$2 WHERE id=$1", p.id, hash)
	return p
}

func (f *fixture) loginAs(email string) reply {
	return f.call(nil, "POST", "/auth/login", map[string]any{"email": email, "password": twoFactorPassword})
}

func (f *fixture) email(p *person) string {
	return f.queryID("SELECT email::text FROM users WHERE id=$1", p.id)
}

func sessionOf(r reply) *person {
	for _, c := range (&http.Response{Header: r.header}).Cookies() {
		if c.Name == "pms_session" && c.Value != "" {
			return &person{cookie: c}
		}
	}
	return nil
}

// codeAt is the TOTP code for the current step plus an offset; the server accepts one step of drift.
func codeAt(secret string, offset int64) string {
	c, _ := twofactor.Code(secret, twofactor.StepAt(time.Now())+offset)
	return c
}

func (f *fixture) enableTwoFactor(p *person) (secret string, backup []any) {
	f.t.Helper()
	setup := f.call(p, "POST", "/me/2fa/setup", nil)
	f.want(setup, 200, "setup")
	secret = setup.body["secret"].(string)
	enabled := f.call(p, "POST", "/me/2fa/enable", map[string]any{"code": codeAt(secret, 0)})
	f.want(enabled, 200, "enable")
	return secret, enabled.body["backup_codes"].([]any)
}

func TestTwoFactorLifecycle(t *testing.T) {
	f := newFixture(t)
	alice := f.withPassword(f.person("regular"))
	email := f.email(alice)

	if st := f.call(alice, "GET", "/me/2fa", nil); st.status != 200 || st.body["configured"] != true || st.body["enabled"] != false {
		t.Fatalf("initial status: %v", st.body)
	}
	other := sessionOf(f.loginAs(email)) // a second signed-in browser
	if other == nil {
		t.Fatal("login without 2FA must still work")
	}

	// Setup stores an encrypted secret that is not active yet.
	setup := f.call(alice, "POST", "/me/2fa/setup", nil)
	secret := setup.body["secret"].(string)
	if setup.status != 200 || !strings.HasPrefix(setup.body["otpauth_uri"].(string), "otpauth://totp/PMS:") || setup.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("setup: %d %v", setup.status, setup.body)
	}
	if stored := f.queryID("SELECT secret_enc FROM user_totp WHERE user_id=$1", alice.id); !strings.HasPrefix(stored, "v1:") || strings.Contains(stored, secret) {
		t.Fatalf("the secret must be stored encrypted, got %q", stored)
	}
	if r := f.loginAs(email); r.status != 200 {
		t.Fatalf("an unconfirmed setup must not change login: %d", r.status)
	}
	f.want(f.call(alice, "POST", "/me/2fa/enable", map[string]any{"code": "000000"}), 422, "wrong code does not enable")
	if st := f.call(alice, "GET", "/me/2fa", nil); st.body["enabled"] != false {
		t.Fatalf("still off: %v", st.body)
	}
	enabled := f.call(alice, "POST", "/me/2fa/enable", map[string]any{"code": codeAt(secret, 0)})
	f.want(enabled, 200, "enable")
	backup := enabled.body["backup_codes"].([]any)
	if len(backup) != 10 {
		t.Fatalf("ten backup codes expected: %v", backup)
	}
	st := f.call(alice, "GET", "/me/2fa", nil)
	if st.body["enabled"] != true || st.body["enabled_at"] == nil || st.body["backup_codes_remaining"] != float64(10) {
		t.Fatalf("enabled status: %v", st.body)
	}
	f.want(f.call(other, "GET", "/me", nil), 401, "other sessions are revoked when 2FA is turned on")
	f.want(f.call(alice, "GET", "/me", nil), 200, "the session that turned it on stays")
	f.want(f.call(alice, "POST", "/me/2fa/setup", nil), 409, "no second setup while on")
	f.want(f.call(alice, "POST", "/me/2fa/enable", map[string]any{"code": codeAt(secret, 1)}), 409, "enable twice")

	// Login now needs the second step; the password alone gives no session.
	first := f.loginAs(email)
	if first.status != 202 || first.body["two_factor_required"] != true || sessionOf(first) != nil {
		t.Fatalf("password only: %d %v cookie=%v", first.status, first.body, sessionOf(first))
	}
	challenge := first.body["challenge"].(string)
	second := func(code string) reply {
		return f.call(nil, "POST", "/auth/login/2fa", map[string]any{"challenge": challenge, "code": code})
	}
	f.want(second("000000"), 401, "wrong code")
	gone := f.call(nil, "POST", "/auth/login/2fa", map[string]any{"challenge": "nope", "code": codeAt(secret, 1)})
	if gone.status != 401 || gone.body["code"] != "challenge_expired" {
		t.Fatalf("an unknown or expired challenge has its own code: %d %v", gone.status, gone.body)
	}
	// The enable step already used the current step, so the same code must not log in (replay).
	f.want(second(codeAt(secret, 0)), 401, "a code that was already used")
	done := second(codeAt(secret, 1))
	if done.status != 200 || sessionOf(done) == nil || done.body["email"] != email {
		t.Fatalf("second step: %d %v", done.status, done.body)
	}
	f.want(second(codeAt(secret, 1)), 401, "challenge is single use")
	f.want(f.call(sessionOf(done), "GET", "/me", nil), 200, "the new session works")

	// Backup codes work once each, and the count shows it.
	login := func(code string) reply {
		start := f.loginAs(email)
		f.want(start, 202, "password step")
		return f.call(nil, "POST", "/auth/login/2fa", map[string]any{"challenge": start.body["challenge"], "code": code})
	}
	f.want(login(strings.ToLower(backup[0].(string))), 200, "backup code (any case) signs in")
	f.want(login(backup[0].(string)), 401, "a backup code works once")
	f.want(login("AAAAA-AAAAA"), 401, "unknown backup code")
	if st := f.call(alice, "GET", "/me/2fa", nil); st.body["backup_codes_remaining"] != float64(9) {
		t.Fatalf("one backup code used: %v", st.body)
	}

	// Turning it off needs the password and a code, and signs the other browsers out.
	off := map[string]any{"password": twoFactorPassword, "code": backup[1]}
	f.want(f.call(alice, "POST", "/me/2fa/disable", map[string]any{"password": "wrong-password-1", "code": backup[1]}), 401, "wrong password")
	f.want(f.call(alice, "POST", "/me/2fa/disable", map[string]any{"password": twoFactorPassword, "code": "000000"}), 401, "wrong code")
	f.want(f.call(alice, "POST", "/me/2fa/disable", map[string]any{"code": backup[1]}), 401, "missing password")
	f.want(f.call(alice, "POST", "/me/2fa/disable", off), 204, "disable")
	if n := f.queryID("SELECT count(*)::text FROM user_totp WHERE user_id=$1", alice.id); n != "0" {
		t.Fatalf("disabling removes the secret, got %s rows", n)
	}
	if n := f.queryID("SELECT count(*)::text FROM user_totp_backup_codes WHERE user_id=$1", alice.id); n != "0" {
		t.Fatalf("disabling removes the backup codes, got %s rows", n)
	}
	if r := f.loginAs(email); r.status != 200 || sessionOf(r) == nil {
		t.Fatalf("login is single-step again: %d", r.status)
	}
	f.want(f.call(sessionOf(done), "GET", "/me", nil), 401, "other sessions are revoked when 2FA is turned off")
	if n := f.queryID("SELECT count(*)::text FROM audit_events WHERE target_id=$1 AND action IN ('auth.2fa_enabled','auth.2fa_disabled')", alice.id); n != "2" {
		t.Fatalf("enable and disable are recorded, got %s", n)
	}
}

func TestTwoFactorBackupCodesAndLockout(t *testing.T) {
	f := newFixture(t)
	bob := f.withPassword(f.person("regular"))
	email := f.email(bob)
	secret, old := f.enableTwoFactor(bob)

	// New codes replace the old ones; re-authentication needs the password and a current code.
	f.want(f.call(bob, "POST", "/me/2fa/backup-codes", map[string]any{"password": twoFactorPassword, "code": "000000"}), 401, "wrong code")
	renewed := f.call(bob, "POST", "/me/2fa/backup-codes", map[string]any{"password": twoFactorPassword, "code": codeAt(secret, 1)})
	f.want(renewed, 200, "renew with a TOTP code")
	fresh := renewed.body["backup_codes"].([]any)
	if len(fresh) != 10 || fresh[0] == old[0] {
		t.Fatalf("new codes: %v", fresh)
	}
	step := func(code string) reply {
		start := f.loginAs(email)
		f.want(start, 202, "password step")
		return f.call(nil, "POST", "/auth/login/2fa", map[string]any{"challenge": start.body["challenge"], "code": code})
	}
	f.want(step(old[0].(string)), 401, "old backup codes stop working")
	f.want(step(fresh[0].(string)), 200, "new backup codes work")

	// Five wrong codes lock the second step for this user, even for the right code.
	for i := 0; i < 5; i++ {
		f.want(step("111111"), 401, "wrong code")
	}
	limited := step(fresh[1].(string))
	if limited.status != 429 || limited.header.Get("Retry-After") == "" {
		t.Fatalf("too many wrong codes must be rate limited: %d %v", limited.status, limited.body)
	}
}

func TestTwoFactorAdminReset(t *testing.T) {
	f := newFixture(t)
	admin, carol, member := f.person("admin"), f.withPassword(f.person("regular")), f.person("regular")
	email := f.email(carol)
	f.enableTwoFactor(carol)

	f.want(f.call(member, "POST", "/users/"+carol.id+"/2fa/reset", nil), 403, "only admins reset")
	f.want(f.call(admin, "POST", "/users/00000000-0000-0000-0000-000000000000/2fa/reset", nil), 404, "unknown user")
	f.want(f.call(admin, "POST", "/users/"+member.id+"/2fa/reset", nil), 204, "a user without 2FA is a no-op")
	if n := f.queryID("SELECT count(*)::text FROM audit_events WHERE action='auth.2fa_reset_by_admin'"); n != "0" {
		t.Fatalf("nothing to reset must not be recorded, got %s", n)
	}
	f.want(f.call(admin, "POST", "/users/"+carol.id+"/2fa/reset", nil), 204, "reset")
	f.want(f.call(carol, "GET", "/me", nil), 401, "the user is signed out everywhere")
	if r := f.loginAs(email); r.status != 200 {
		t.Fatalf("after a reset the user logs in with the password only: %d", r.status)
	}
	if got := f.queryID("SELECT actor_id::text FROM audit_events WHERE target_id=$1 AND action='auth.2fa_reset_by_admin'", carol.id); got != admin.id {
		t.Fatalf("the reset records who did it: %q", got)
	}
}

func TestTwoFactorConfiguration(t *testing.T) {
	// Without a key, 2FA cannot be turned on and says so.
	keep := testTwoFactorKeys
	testTwoFactorKeys = twofactor.Keys{}
	f := newFixture(t)
	testTwoFactorKeys = keep
	dave := f.person("regular")
	if st := f.call(dave, "GET", "/me/2fa", nil); st.status != 200 || st.body["configured"] != false {
		t.Fatalf("status without a key: %v", st.body)
	}
	if r := f.call(dave, "POST", "/me/2fa/setup", nil); r.status != 503 || r.body["code"] != "not_configured" {
		t.Fatalf("setup without a key: %d %v", r.status, r.body)
	}

	// Rotation: a secret written with the previous key still works and moves to the current key on use.
	oldKey, newKey := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	testTwoFactorKeys = twofactor.Keys{Current: newKey, Previous: oldKey}
	g := newFixture(t)
	testTwoFactorKeys = keep
	erin := g.withPassword(g.person("regular"))
	secret, _ := twofactor.NewSecret()
	enc, err := twofactor.Keys{Current: oldKey}.Encrypt(secret, erin.id)
	if err != nil {
		t.Fatal(err)
	}
	g.exec("INSERT INTO user_totp(user_id,secret_enc,confirmed_at) VALUES($1,$2,now())", erin.id, enc)
	start := g.loginAs(g.email(erin))
	g.want(start, 202, "password step")
	g.want(g.call(nil, "POST", "/auth/login/2fa", map[string]any{"challenge": start.body["challenge"], "code": codeAt(secret, 0)}), 200, "login with a secret from the previous key")
	stored := g.queryID("SELECT secret_enc FROM user_totp WHERE user_id=$1", erin.id)
	if _, old, err := (twofactor.Keys{Current: newKey}).Decrypt(stored, erin.id); err != nil || old {
		t.Fatalf("the secret must now be readable with the current key alone: old=%v err=%v", old, err)
	}
}
