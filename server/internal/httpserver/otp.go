package httpserver

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	mailqueue "github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Sign-in with a one-time code mailed to the person (AUTH-08, D-97). It replaces the password, not the second factor: a person with
// two-factor still answers it. Off until an administrator turns it on.
// The short code is stored as an HMAC under a server-only key, so a database copy is not an offline guessing oracle.

const (
	auditOTPRequested = "auth.otp_requested"
	otpLifetime       = 10 * time.Minute
	otpCooldown       = time.Minute
	otpMaxAttempts    = 5
)

var otpFormat = regexp.MustCompile(`^[0-9]{6}$`)

func (s *Server) otpRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/otp/request", s.otpRequest)
	mux.HandleFunc("POST /api/v1/auth/otp/verify", s.otpVerify)
}

func otpHash(key []byte, codeID, code string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(codeID + ":" + code))
	return mac.Sum(nil)
}

// otpGate answers whether the feature can be used from here; it writes the refusal itself.
func (s *Server) otpGate(w http.ResponseWriter, r *http.Request) bool {
	if !s.ipAllowed(r) {
		writeIPDenied(w)
		return false
	}
	p, err := s.loadPolicy(r.Context())
	if err != nil {
		writeError(w, 500, "could not check the sign-in policy")
		return false
	}
	if !p.OTP {
		writeError(w, http.StatusNotFound, "signing in with a code is not turned on")
		return false
	}
	if len(s.cfg.OTPKey) != 32 {
		writeError(w, http.StatusServiceUnavailable, "signing in with a code is temporarily unavailable")
		return false
	}
	return true
}

// otpRequest always answers 202, whether or not the address belongs to someone, so it cannot be used to find out who has an account.
func (s *Server) otpRequest(w http.ResponseWriter, r *http.Request) {
	if !s.otpGate(w, r) {
		return
	}
	if retry := s.forgotLimiter.take("otp:" + s.clientIP(r)); retry > 0 {
		writeRateLimit(w, retry)
		return
	}
	var in struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	var id, locale string
	if err := s.pool.QueryRow(r.Context(), "SELECT id::text,locale FROM users WHERE email=$1 AND status='active'", email).Scan(&id, &locale); err == nil {
		if err := s.issueCode(r, id, email, locale); err != nil {
			slog.ErrorContext(r.Context(), "issue sign-in code", "error", err)
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) issueCode(r *http.Request, userID, email, locale string) error {
	ctx := r.Context()
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// One request a minute per person: the code already on its way is the answer, and nobody can be flooded with mail.
	var recent bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM login_codes WHERE user_id=$1 AND created_at > now()-$2::interval)", userID, otpCooldown.String()).Scan(&recent); err != nil {
		return err
	}
	if recent {
		return nil
	}
	// A new code replaces every earlier one.
	if _, err := tx.Exec(ctx, "UPDATE login_codes SET used_at=now() WHERE user_id=$1 AND used_at IS NULL", userID); err != nil {
		return err
	}
	codeID := uuid.NewString() // the id salts the hash, so it is chosen before the row exists
	if _, err := tx.Exec(ctx, "INSERT INTO login_codes(id,user_id,code_hash,expires_at) VALUES($1,$2,$3,$4)", codeID, userID, otpHash(s.cfg.OTPKey, codeID, code), time.Now().Add(otpLifetime)); err != nil {
		return err
	}
	if err := s.mail.SendTx(ctx, tx, mailqueue.Args{To: email, Template: "login_code", Message: code, Locale: locale}); err != nil {
		return err
	}
	if err := s.auditIn(ctx, tx, r, userID, "", auditEvent{Action: auditOTPRequested, TargetType: "user", TargetID: userID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Server) otpVerify(w http.ResponseWriter, r *http.Request) {
	if !s.otpGate(w, r) {
		return
	}
	var in struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	ip, account := s.clientIP(r), strings.ToLower(strings.TrimSpace(in.Email))
	// The failure limits are the password sign-in's own: guessing codes and guessing passwords draw on the same budget.
	if retry := max(s.loginIPFails.wait(ip), s.loginAcctFails.wait(account)); retry > 0 {
		writeRateLimit(w, retry)
		return
	}
	fail := func(userID string) {
		s.loginIPFails.hit(ip)
		s.loginAcctFails.hit(account)
		s.audit(r, userID, account, auditEvent{Action: auditLoginFailed, TargetType: "user", TargetID: userID, Details: map[string]any{"reason": "otp"}})
		writeError(w, http.StatusUnauthorized, "the code is not valid or has expired")
	}
	userID, ok := s.checkCode(r, account, in.Code)
	if !ok {
		fail(userID)
		return
	}
	s.loginAcctFails.clear(account)
	s.completeSignIn(w, r, userID, "otp")
}

// checkCode counts the attempt before it looks at the code, so a wrong guess is spent even when the answer is slow or the connection drops.
// It returns the user it recognised (for the audit entry) and whether the code was right.
func (s *Server) checkCode(r *http.Request, email, code string) (string, bool) {
	ctx := r.Context()
	var userID string
	if err := s.pool.QueryRow(ctx, "SELECT id::text FROM users WHERE email=$1 AND status='active'", email).Scan(&userID); err != nil {
		return "", false
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return userID, false
	}
	defer tx.Rollback(ctx)
	var id string
	var hash []byte
	var attempts int
	err = tx.QueryRow(ctx, `UPDATE login_codes SET attempts=attempts+1 WHERE id=(
SELECT id FROM login_codes WHERE user_id=$1 AND used_at IS NULL AND expires_at>now() ORDER BY created_at DESC LIMIT 1 FOR UPDATE)
RETURNING id::text,code_hash,attempts`, userID).Scan(&id, &hash, &attempts)
	if err != nil {
		if err != pgx.ErrNoRows {
			slog.ErrorContext(r.Context(), "check sign-in code", "error", err)
		}
		return userID, false
	}
	match := otpFormat.MatchString(code) && subtle.ConstantTimeCompare(hash, otpHash(s.cfg.OTPKey, id, code)) == 1
	if match && attempts <= otpMaxAttempts {
		tag, err := tx.Exec(ctx, "UPDATE login_codes SET used_at=now() WHERE id=$1 AND used_at IS NULL", id)
		if err != nil || tag.RowsAffected() != 1 {
			return userID, false
		}
		return userID, tx.Commit(ctx) == nil
	}
	if attempts >= otpMaxAttempts { // the fifth wrong guess burns the code
		_, _ = tx.Exec(ctx, "UPDATE login_codes SET used_at=now() WHERE id=$1", id)
	}
	_ = tx.Commit(ctx)
	return userID, false
}
