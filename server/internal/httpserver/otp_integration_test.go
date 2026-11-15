package httpserver_test

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
)

var sixDigits = regexp.MustCompile(`^[0-9]{6}$`)

// setPolicy saves the access policy the way an administrator does: read the version, then replace it.
func (f *fixture) setPolicy(admin *person, otp bool, allowlist []string, headers ...string) reply {
	f.t.Helper()
	cur := f.call(admin, "GET", "/access-policy", nil, headers...)
	f.want(cur, 200, "read policy")
	if allowlist == nil {
		allowlist = []string{}
	}
	return f.call(admin, "PUT", "/access-policy", map[string]any{"ip_allowlist_enabled": len(allowlist) > 0, "ip_allowlist": allowlist, "otp_login_enabled": otp},
		append([]string{"If-Match", cur.header.Get("ETag")}, headers...)...)
}

// lastCode reads the code out of the queued email, the way a person reads it from their mailbox.
func (f *fixture) lastCode() (to, code string) {
	f.t.Helper()
	to = f.queryID("SELECT args->>'to' FROM river_job WHERE args->>'template'='login_code' ORDER BY id DESC LIMIT 1")
	code = f.queryID("SELECT args->>'message' FROM river_job WHERE args->>'template'='login_code' ORDER BY id DESC LIMIT 1")
	return to, code
}

func (f *fixture) codesSent() int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, "SELECT count(*) FROM river_job WHERE args->>'template'='login_code'").Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func (f *fixture) otpRequest(email string, headers ...string) reply {
	return f.call(nil, "POST", "/auth/otp/request", map[string]any{"email": email}, headers...)
}

func (f *fixture) otpVerify(email, code string, headers ...string) reply {
	return f.call(nil, "POST", "/auth/otp/verify", map[string]any{"email": email, "code": code}, headers...)
}

func TestOTPIsOffUntilAnAdministratorTurnsItOn(t *testing.T) {
	f := newFixture(t)
	admin, alice := f.person("admin"), f.withPassword(f.person("regular"))
	email := f.email(alice)
	if r := f.call(nil, "GET", "/auth/sso/providers", nil); r.body["otp_login"] != false {
		t.Fatalf("providers: %v", r.body)
	}
	f.want(f.otpRequest(email), 404, "request while off")
	f.want(f.otpVerify(email, "123456"), 404, "verify while off")
	if f.codesSent() != 0 {
		t.Fatal("nothing may be sent while the feature is off")
	}
	f.want(f.call(alice, "PUT", "/access-policy", map[string]any{"ip_allowlist_enabled": false, "ip_allowlist": []string{}, "otp_login_enabled": true}, "If-Match", `"1"`), 403, "a regular user cannot turn it on")

	f.want(f.setPolicy(admin, true, nil), 200, "turn on")
	if r := f.call(nil, "GET", "/auth/sso/providers", nil); r.body["otp_login"] != true {
		t.Fatalf("providers after: %v", r.body)
	}
	if e := lastAudit(t, f, admin, "policy.ip_allowlist_changed"); e["details"].(map[string]any)["otp_login"] != true {
		t.Fatalf("the change must be audited: %v", e)
	}
	f.want(f.setPolicy(admin, false, nil), 200, "turn off")
	f.want(f.otpRequest(email), 404, "off again")
}

func TestOTPCannotBeEnabledWithoutAHashKey(t *testing.T) {
	f := newFixtureWith(t, 25, func(cfg *config.Config) { cfg.OTPKey = nil })
	admin := f.person("admin")
	r := f.setPolicy(admin, true, nil)
	if r.status != 422 || r.body["code"] != "key_not_configured" {
		t.Fatalf("enable without PMS_OTP_KEY: %d %v", r.status, r.body)
	}
	f2 := newFixtureWith(t, 25, func(cfg *config.Config) { cfg.OTPKey = nil })
	f2.exec("UPDATE access_policy SET otp_login_enabled=true WHERE id=1") // an enabled policy carried across an upgrade
	f2.want(f2.otpRequest(f2.email(f2.person("regular"))), 503, "enabled policy without key")
}

func TestOTPSignInWorksOnceAndIsAudited(t *testing.T) {
	f := newFixture(t)
	admin, alice := f.person("admin"), f.withPassword(f.person("regular"))
	email := f.email(alice)
	f.want(f.setPolicy(admin, true, nil), 200, "turn on")

	f.want(f.otpRequest(strings.ToUpper(email)), 202, "request")
	to, code := f.lastCode()
	if to != email || !sixDigits.MatchString(code) {
		t.Fatalf("mail to %q with code %q", to, code)
	}
	if f.queryID("SELECT count(*)::text FROM login_codes WHERE encode(code_hash,'hex') LIKE '%'||$1||'%'", code) != "0" {
		t.Fatal("the code itself must not be stored")
	}
	id := f.queryID("SELECT id::text FROM login_codes WHERE user_id=$1", alice.id)
	plain := sha256.Sum256([]byte(id + ":" + code))
	if stored := f.queryID("SELECT encode(code_hash,'hex') FROM login_codes WHERE id=$1", id); stored == hex.EncodeToString(plain[:]) {
		t.Fatal("a database-only salted hash permits offline guessing; the stored value must be keyed")
	}
	if r := f.otpVerify(email, wrongCode(code)); r.status != 401 || r.body["code"] != "unauthenticated" {
		t.Fatalf("a wrong code: %d %v", r.status, r.body)
	}
	good := f.otpVerify(strings.ToUpper(email), code)
	f.want(good, 200, "the right code")
	session := sessionOf(good)
	if session == nil || good.body["id"] != alice.id {
		t.Fatalf("a session for alice: %v", good.body)
	}
	if me := f.call(session, "GET", "/me", nil); me.status != 200 || me.body["id"] != alice.id {
		t.Fatalf("the session works: %d", me.status)
	}
	if e := lastAudit(t, f, admin, "auth.login"); e["actor_id"] != alice.id || e["details"].(map[string]any)["method"] != "otp" {
		t.Fatalf("audit: %v", e)
	}
	if lastAudit(t, f, admin, "auth.otp_requested")["actor_id"] != alice.id {
		t.Fatal("the request must be audited")
	}
	if r := f.otpVerify(email, code); r.status != 401 || sessionOf(r) != nil {
		t.Fatalf("a code works once: %d", r.status)
	}
	// The password still works beside it.
	f.want(f.loginAs(email), 200, "password sign-in is unchanged")
}

func TestOTPConcurrentVerificationSucceedsOnce(t *testing.T) {
	f := newFixture(t)
	admin, alice := f.person("admin"), f.withPassword(f.person("regular"))
	email := f.email(alice)
	f.want(f.setPolicy(admin, true, nil), 200, "turn on")
	f.want(f.otpRequest(email), 202, "request")
	_, code := f.lastCode()

	before := f.queryID("SELECT count(*)::text FROM sessions WHERE user_id=$1", alice.id) // the fixture already holds one session for alice
	const requests = 5
	results := make(chan reply, requests)
	var ready, start sync.WaitGroup
	ready.Add(requests)
	start.Add(1)
	for i := 0; i < requests; i++ {
		go func() {
			ready.Done()
			start.Wait()
			results <- f.otpVerify(email, code, "X-Forwarded-For", "203.0.113.42")
		}()
	}
	ready.Wait()
	start.Done()
	var success int
	for i := 0; i < requests; i++ {
		r := <-results
		if r.status == 200 {
			success++
		} else if r.status != 401 {
			t.Errorf("concurrent verify status %d: %v", r.status, r.body)
		}
	}
	if success != 1 {
		t.Fatalf("concurrent verification succeeded %d times, want once", success)
	}
	if n, _ := strconv.Atoi(before); f.queryID("SELECT count(*)::text FROM sessions WHERE user_id=$1", alice.id) != strconv.Itoa(n+1) {
		t.Fatal("one code must create exactly one session")
	}
}

func wrongCode(code string) string {
	if code == "000000" {
		return "000001"
	}
	return "000000"
}

func TestOTPRefusesWhatIsNotAProvenActiveUser(t *testing.T) {
	f := newFixture(t)
	admin, alice, gone := f.person("admin"), f.withPassword(f.person("regular")), f.person("regular")
	email := f.email(alice)
	f.want(f.setPolicy(admin, true, nil), 200, "turn on")

	// Unknown and deactivated addresses look exactly like known ones: 202 on request, 401 on verify, and nothing is sent.
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", gone.id)
	for _, who := range []string{"nobody@example.com", f.email(gone)} {
		f.want(f.otpRequest(who), 202, "request for "+who)
		if r := f.otpVerify(who, "123456"); r.status != 401 || r.body["detail"] != "the code is not valid or has expired" {
			t.Errorf("verify for %s: %d %v", who, r.status, r.body)
		}
	}
	if f.codesSent() != 0 || f.queryID("SELECT count(*)::text FROM login_codes") != "0" {
		t.Fatal("no code may be made for an address that cannot sign in")
	}

	// Format: anything but six digits is refused without touching the database's code.
	f.want(f.otpRequest(email), 202, "request")
	_, code := f.lastCode()
	for _, bad := range []string{"", "12345", "abcdef", code + " "} { // four refusals: the fifth would trip the shared limit
		if r := f.otpVerify(email, bad); r.status != 401 {
			t.Errorf("code %q: %d", bad, r.status)
		}
	}
	// A code is also for one person only.
	other := f.withPassword(f.person("regular"))
	if r := f.otpVerify(f.email(other), code); r.status != 401 {
		t.Fatalf("someone else's code: %d", r.status)
	}
}

func TestOTPCodeIsBurnedAfterFiveWrongGuessesAndExpires(t *testing.T) {
	f := newFixture(t)
	admin, alice := f.person("admin"), f.withPassword(f.person("regular"))
	email := f.email(alice)
	f.want(f.setPolicy(admin, true, nil), 200, "turn on")
	f.want(f.otpRequest(email), 202, "request")
	_, code := f.lastCode()
	for i := 0; i < 5; i++ {
		f.want(f.otpVerify(email, wrongCode(code)), 401, "wrong guess")
	}
	// The fifth wrong guess burned the code, and the shared failure limit now holds this address back.
	if r := f.otpVerify(email, code); r.status != 429 {
		t.Fatalf("after five failures the right code still gets %d, want 429", r.status)
	}
	if f.queryID("SELECT (used_at IS NOT NULL)::text FROM login_codes WHERE user_id=$1", alice.id) != "true" {
		t.Fatal("five wrong guesses must burn the code")
	}

	// Expiry, on a fresh person so the limit above does not interfere.
	bob := f.person("regular")
	f.want(f.otpRequest(f.email(bob)), 202, "request")
	_, code = f.lastCode()
	f.exec("UPDATE login_codes SET expires_at=now()-interval '1 second' WHERE user_id=$1", bob.id)
	if r := f.otpVerify(f.email(bob), code); r.status != 401 {
		t.Fatalf("an expired code: %d", r.status)
	}
}

func TestOTPOneCodeAtATimeAndOneRequestAMinute(t *testing.T) {
	f := newFixture(t)
	admin, alice := f.person("admin"), f.withPassword(f.person("regular"))
	email := f.email(alice)
	f.want(f.setPolicy(admin, true, nil), 200, "turn on")

	f.want(f.otpRequest(email), 202, "first")
	_, first := f.lastCode()
	f.want(f.otpRequest(email), 202, "again at once")
	if f.codesSent() != 1 {
		t.Fatalf("a second request inside a minute must not send another mail, sent %d", f.codesSent())
	}
	f.exec("UPDATE login_codes SET created_at=created_at-interval '2 minutes' WHERE user_id=$1", alice.id)
	f.want(f.otpRequest(email), 202, "after the cooldown")
	_, second := f.lastCode()
	if f.codesSent() != 2 {
		t.Fatalf("sent %d", f.codesSent())
	}
	if first != second { // (one in a million they match; then the older-code check below cannot tell them apart)
		if r := f.otpVerify(email, first); r.status != 401 {
			t.Fatalf("the earlier code must stop working once a new one is sent: %d", r.status)
		}
	}
	f.want(f.otpVerify(email, second), 200, "the new code")
}

func TestOTPStillAsksForTwoFactorAndFollowsTheIPAllowlist(t *testing.T) {
	f := newFixture(t)
	admin, alice := f.person("admin"), f.withPassword(f.person("regular"))
	email := f.email(alice)
	f.want(f.setPolicy(admin, true, nil), 200, "turn on")
	f.exec("INSERT INTO user_totp(user_id,secret_enc,confirmed_at) VALUES ($1,'x',now())", alice.id)
	f.want(f.otpRequest(email), 202, "request")
	_, code := f.lastCode()
	r := f.otpVerify(email, code)
	if r.status != 202 || r.body["two_factor_required"] != true || sessionOf(r) != nil || r.body["challenge"] == "" {
		t.Fatalf("a person with two-factor gets the second step, not a session: %d %v", r.status, r.body)
	}

	// The address list guards the code endpoints too.
	here := []string{"X-Forwarded-For", "203.0.113.7"}
	f.want(f.setPolicy(admin, true, []string{"203.0.113.7"}, here...), 200, "allowlist on")
	denied := f.otpRequest(email, "X-Forwarded-For", "192.0.2.50")
	if denied.status != 403 || denied.body["code"] != "ip_not_allowed" {
		t.Fatalf("request from outside: %d %v", denied.status, denied.body)
	}
	if r := f.otpVerify(email, "123456", "X-Forwarded-For", "192.0.2.50"); r.status != 403 {
		t.Fatalf("verify from outside: %d", r.status)
	}
	f.want(f.otpRequest(email, here...), 202, "request from inside")
}

func TestOwnershipTransferKeepsAdministratorAccessAndProtectsTheOwner(t *testing.T) {
	f := newFixture(t)
	owner, other, third := f.withPassword(f.person("admin")), f.person("admin"), f.person("admin")
	regular := f.person("regular")
	f.exec("UPDATE account SET owner_id=$1", owner.id)
	ownerEmail := f.email(owner)

	// Reading: any administrator sees who the owner is; only the owner sees who can receive the role.
	view := f.call(other, "GET", "/account/ownership", nil)
	f.want(view, 200, "read as another admin")
	if v := view.body["owner"].(map[string]any); v["id"] != owner.id || view.body["is_owner"] != false || len(view.body["candidates"].([]any)) != 0 {
		t.Fatalf("view: %v", view.body)
	}
	mine := f.call(owner, "GET", "/account/ownership", nil)
	if mine.body["is_owner"] != true || len(mine.body["candidates"].([]any)) != 2 {
		t.Fatalf("owner's view: %v", mine.body)
	}
	f.want(f.call(nil, "GET", "/account/ownership", nil), 401, "anonymous")
	f.want(f.call(regular, "GET", "/account/ownership", nil), 403, "regular")
	f.want(f.call(regular, "POST", "/account/ownership", map[string]any{"new_owner_id": other.id}), 403, "regular transfer")

	// The owner cannot be demoted or switched off by another administrator.
	for name, call := range map[string]func() reply{
		"demote": func() reply {
			return f.call(other, "PATCH", "/users/"+owner.id, map[string]any{"user_type": "regular"})
		},
		"deactivate": func() reply { return f.call(other, "POST", "/users/"+owner.id+"/deactivate", nil) },
	} {
		if r := call(); r.status != 409 || r.body["code"] != "account_owner" {
			t.Errorf("%s the owner: %d %v", name, r.status, r.body)
		}
	}
	if f.queryID("SELECT status::text||'/'||user_type::text FROM users WHERE id=$1", owner.id) != "active/admin" {
		t.Fatal("the owner must be unchanged")
	}

	// Only the owner can hand it over, and only with proof.
	f.want(f.call(other, "POST", "/account/ownership", map[string]any{"new_owner_id": third.id, "password": "x"}), 403, "not the owner")
	for name, body := range map[string]map[string]any{
		"to yourself":          {"new_owner_id": owner.id, "password": twoFactorPassword},
		"not an id":            {"new_owner_id": "someone", "password": twoFactorPassword},
		"to a regular user":    {"new_owner_id": regular.id, "password": twoFactorPassword},
		"to an unknown person": {"new_owner_id": "00000000-0000-4000-8000-000000000042", "password": twoFactorPassword},
	} {
		if r := f.call(owner, "POST", "/account/ownership", body); r.status != 422 {
			t.Errorf("%s: %d %v", name, r.status, r.body)
		}
	}
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", third.id)
	f.want(f.call(owner, "POST", "/account/ownership", map[string]any{"new_owner_id": third.id, "password": twoFactorPassword}), 422, "to a deactivated admin")
	f.exec("UPDATE users SET status='active' WHERE id=$1", third.id)
	if r := f.call(owner, "POST", "/account/ownership", map[string]any{"new_owner_id": other.id}); r.status != 401 || r.body["code"] != "invalid_password" {
		t.Fatalf("no password: %d %v", r.status, r.body)
	}
	if r := f.call(owner, "POST", "/account/ownership", map[string]any{"new_owner_id": other.id, "password": "wrong"}); r.status != 401 {
		t.Fatalf("wrong password: %d", r.status)
	}
	if f.queryID("SELECT owner_id::text FROM account") != owner.id {
		t.Fatal("a refused transfer must change nothing")
	}

	done := f.call(owner, "POST", "/account/ownership", map[string]any{"new_owner_id": other.id, "password": twoFactorPassword})
	f.want(done, 200, "transfer")
	if done.body["is_owner"] != false || done.body["owner"].(map[string]any)["id"] != other.id {
		t.Fatalf("after the transfer: %v", done.body)
	}
	if f.queryID("SELECT owner_id::text FROM account") != other.id {
		t.Fatal("the owner must have moved")
	}
	// Nobody loses administrator access, and the old owner is now an ordinary administrator.
	if f.queryID("SELECT status::text||'/'||user_type::text FROM users WHERE id=$1", owner.id) != "active/admin" || f.call(owner, "GET", "/audit-events?limit=1", nil).status != 200 {
		t.Fatal("the previous owner must keep administrator access")
	}
	if e := lastAudit(t, f, other, "account.ownership_transferred"); e["actor_id"] != owner.id || e["details"].(map[string]any)["to"] != other.id {
		t.Fatalf("audit: %v", e)
	}
	f.want(f.call(owner, "POST", "/account/ownership", map[string]any{"new_owner_id": third.id, "password": twoFactorPassword}), 403, "the previous owner can no longer hand it over")
	f.want(f.call(other, "PATCH", "/users/"+owner.id, map[string]any{"user_type": "regular"}), 200, "the old owner can now be demoted")
	_ = ownerEmail
}

func TestOwnershipWithoutAPasswordNeedsARecentSignIn(t *testing.T) {
	f := newFixture(t)
	owner, other := f.person("admin"), f.person("admin")
	f.exec("UPDATE users SET password_hash=NULL, scim_provisioned=true WHERE id=$1", owner.id) // signs in through single sign-on or a code
	f.exec("UPDATE account SET owner_id=$1", owner.id)
	f.exec("UPDATE sessions SET created_at=now()-interval '2 hours' WHERE user_id=$1", owner.id)
	r := f.call(owner, "POST", "/account/ownership", map[string]any{"new_owner_id": other.id})
	if r.status != 403 || r.body["code"] != "recent_sign_in_required" {
		t.Fatalf("an old session: %d %v", r.status, r.body)
	}
	f.exec("UPDATE sessions SET created_at=now() WHERE user_id=$1", owner.id)
	f.want(f.call(owner, "POST", "/account/ownership", map[string]any{"new_owner_id": other.id}), 200, "a fresh sign-in is proof enough")
}

func TestBootstrapMakesTheFirstAdministratorTheOwner(t *testing.T) {
	f := newFixture(t) // newFixture runs bootstrap for boot@example.com
	if got := f.queryID("SELECT u.email::text FROM account a JOIN users u ON u.id=a.owner_id"); got != "boot@example.com" {
		t.Fatalf("owner = %s", got)
	}
}
