package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/reqid"
	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/anissaefuddin/pms/server/internal/twofactor"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	twoFactorIssuer    = "PMS"
	challengeLifetime  = 5 * time.Minute
	errTwoFactorOff    = "two-factor authentication is not configured on this server"
	errInvalidSecond   = "invalid code"
	errNeedsPasswordFA = "password and a current code are required"
)

func (s *Server) twoFactorRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login/2fa", s.loginSecondStep)
	mux.HandleFunc("GET /api/v1/me/2fa", s.requireUser(s.twoFactorStatus))
	mux.HandleFunc("POST /api/v1/me/2fa/setup", s.requireUser(s.twoFactorSetup))
	mux.HandleFunc("POST /api/v1/me/2fa/enable", s.requireUser(s.twoFactorEnable))
	mux.HandleFunc("POST /api/v1/me/2fa/disable", s.requireUser(s.twoFactorDisable))
	mux.HandleFunc("POST /api/v1/me/2fa/backup-codes", s.requireUser(s.twoFactorNewBackupCodes))
	mux.HandleFunc("POST /api/v1/users/{userId}/2fa/reset", s.requireUser(requireAdmin(s.twoFactorAdminReset)))
}

func (s *Server) twoFactorKeys() twofactor.Keys {
	return twofactor.Keys{Current: s.cfg.TwoFactorKey, Previous: s.cfg.TwoFactorKeyPrevious}
}

func (s *Server) twoFactorOn() bool { return len(s.cfg.TwoFactorKey) == 32 }

// securityEvent notes a security-relevant change. It is part of the caller's transaction, so it cannot be skipped.
func securityEvent(ctx context.Context, tx pgx.Tx, actor, target, action string) error {
	// The trail lives in audit_events (migration 00300); security_events only keeps what was recorded before it.
	_, err := tx.Exec(ctx, `INSERT INTO audit_events(actor_id,actor_name,action,target_type,target_id,request_id)
VALUES($1::uuid, COALESCE((SELECT email::text FROM users WHERE id=$1::uuid),''), 'auth.'||$3, 'user', $2, $4)`, actor, target, action, reqid.From(ctx))
	return err
}

// loadTOTP returns the user's decrypted secret, whether setup was confirmed, and the last used step.
func (s *Server) loadTOTP(ctx context.Context, tx pgx.Tx, userID string, lock bool) (secret string, confirmed bool, last int64, found bool, err error) {
	var stored string
	var at *time.Time
	query := "SELECT secret_enc,confirmed_at,last_step FROM user_totp WHERE user_id=$1::uuid"
	if lock {
		query += " FOR UPDATE"
	}
	if err = tx.QueryRow(ctx, query, userID).Scan(&stored, &at, &last); errors.Is(err, pgx.ErrNoRows) {
		return "", false, 0, false, nil
	} else if err != nil {
		return "", false, 0, false, err
	}
	plain, old, err := s.twoFactorKeys().Decrypt(stored, userID)
	if err != nil {
		return "", false, 0, true, err
	}
	if old && s.twoFactorOn() { // written with the previous key: move it to the current one
		if fresh, encErr := s.twoFactorKeys().Encrypt(plain, userID); encErr == nil {
			if _, execErr := tx.Exec(ctx, "UPDATE user_totp SET secret_enc=$2 WHERE user_id=$1::uuid", userID, fresh); execErr != nil {
				return "", false, 0, true, execErr
			}
		}
	}
	return plain, at != nil, last, true, nil
}

// checkSecondFactor accepts a TOTP code or an unused backup code. Each accepted code works once.
func (s *Server) checkSecondFactor(ctx context.Context, tx pgx.Tx, userID, code string) (bool, error) {
	secret, confirmed, last, found, err := s.loadTOTP(ctx, tx, userID, true)
	if err != nil || !found || !confirmed {
		return false, err
	}
	digits := strings.NewReplacer(" ", "", "-", "").Replace(code)
	if step, ok := twofactor.Verify(secret, digits, time.Now(), last); ok {
		tag, err := tx.Exec(ctx, "UPDATE user_totp SET last_step=$2 WHERE user_id=$1::uuid AND last_step<$2", userID, step)
		return err == nil && tag.RowsAffected() == 1, err
	}
	if len(twofactor.NormalizeBackup(code)) != 10 {
		return false, nil
	}
	tag, err := tx.Exec(ctx, "UPDATE user_totp_backup_codes SET used_at=now() WHERE user_id=$1::uuid AND code_hash=$2 AND used_at IS NULL", userID, twofactor.HashBackup(code))
	return err == nil && tag.RowsAffected() == 1, err
}

// issueChallenge is called by login once the password is right and the user has 2FA on.
func (s *Server) issueChallenge(ctx context.Context, userID string) (string, error) {
	plain, hash, err := security.Token()
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx, "INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) VALUES($1,$2::uuid,'mfa',$3)", hash, userID, time.Now().Add(challengeLifetime))
	return plain, err
}

func (s *Server) hasTwoFactor(ctx context.Context, userID string) (bool, error) {
	var on bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM user_totp WHERE user_id=$1::uuid AND confirmed_at IS NOT NULL)", userID).Scan(&on)
	return on, err
}

func (s *Server) loginSecondStep(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Challenge string `json:"challenge"`
		Code      string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	ip := s.clientIP(r)
	if retry := s.loginIPFails.wait(ip); retry > 0 {
		writeRateLimit(w, retry)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		writeError(w, 500, "could not check the code")
		return
	}
	defer tx.Rollback(ctx)
	var userID string
	err = tx.QueryRow(ctx, "SELECT user_id::text FROM user_tokens WHERE token_hash=$1 AND purpose='mfa' AND used_at IS NULL AND expires_at>now() FOR UPDATE", security.TokenHash(in.Challenge)).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		s.loginIPFails.hit(ip)
		writeProblem(w, http.StatusUnauthorized, "challenge_expired", "the sign-in expired; enter your password again")
		return
	}
	if err != nil {
		writeError(w, 500, "could not check the code")
		return
	}
	guard := "2fa:" + userID
	if retry := s.mfaFails.wait(guard); retry > 0 {
		writeRateLimit(w, retry)
		return
	}
	ok, err := s.checkSecondFactor(ctx, tx, userID, in.Code)
	if err != nil {
		slog.ErrorContext(r.Context(), "second factor", "error", err)
		writeError(w, 500, "could not check the code")
		return
	}
	if !ok {
		s.mfaFails.hit(guard)
		s.loginIPFails.hit(ip)
		s.audit(r, userID, "", auditEvent{Action: auditLoginFailed, TargetType: "user", TargetID: userID, Details: map[string]any{"reason": "second_factor"}})
		writeError(w, http.StatusUnauthorized, errInvalidSecond)
		return
	}
	if _, err := tx.Exec(ctx, "UPDATE user_tokens SET used_at=now() WHERE token_hash=$1", security.TokenHash(in.Challenge)); err != nil {
		writeError(w, 500, "could not check the code")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, 500, "could not check the code")
		return
	}
	s.mfaFails.clear(guard)
	if presented, err := r.Cookie("pms_session"); err == nil { // same as a plain login: the held session is replaced
		_, _ = s.pool.Exec(ctx, "DELETE FROM sessions WHERE token_hash=$1", security.TokenHash(presented.Value))
	}
	if !s.startSession(w, r, userID) {
		return
	}
	_, _ = s.pool.Exec(ctx, "UPDATE users SET last_login_at=now() WHERE id=$1", userID)
	s.audit(r, userID, "", auditEvent{Action: auditLogin, TargetType: "user", TargetID: userID, Details: map[string]any{"method": "password+2fa"}})
	s.writeMe(w, r, userID)
}

func (s *Server) twoFactorStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userKey).(string)
	out := map[string]any{"configured": s.twoFactorOn(), "enabled": false, "enabled_at": nil, "backup_codes_remaining": 0}
	var at *time.Time
	if err := s.pool.QueryRow(r.Context(), "SELECT confirmed_at FROM user_totp WHERE user_id=$1::uuid", userID).Scan(&at); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 500, "could not read the setting")
		return
	}
	if at != nil {
		out["enabled"], out["enabled_at"] = true, at
		var left int
		if err := s.pool.QueryRow(r.Context(), "SELECT count(*) FROM user_totp_backup_codes WHERE user_id=$1::uuid AND used_at IS NULL", userID).Scan(&left); err != nil {
			writeError(w, 500, "could not read the setting")
			return
		}
		out["backup_codes_remaining"] = left
	}
	writeJSON(w, 200, out)
}

// twoFactorSetup starts (or restarts) enrolment: it stores a fresh secret that is not active until enable confirms it.
func (s *Server) twoFactorSetup(w http.ResponseWriter, r *http.Request) {
	if !s.twoFactorOn() {
		writeProblem(w, http.StatusServiceUnavailable, "not_configured", errTwoFactorOff)
		return
	}
	userID := r.Context().Value(userKey).(string)
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		writeError(w, 500, "could not start setup")
		return
	}
	defer tx.Rollback(ctx)
	var email string
	var enabled bool
	if err := tx.QueryRow(ctx, "SELECT u.email::text,EXISTS(SELECT 1 FROM user_totp t WHERE t.user_id=u.id AND t.confirmed_at IS NOT NULL) FROM users u WHERE u.id=$1::uuid FOR UPDATE OF u", userID).Scan(&email, &enabled); err != nil {
		writeError(w, 500, "could not start setup")
		return
	}
	if enabled {
		writeProblem(w, http.StatusConflict, "already_enabled", "two-factor authentication is already on; turn it off first to set it up again")
		return
	}
	secret, err := twofactor.NewSecret()
	var enc string
	if err == nil {
		enc, err = s.twoFactorKeys().Encrypt(secret, userID)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO user_totp(user_id,secret_enc) VALUES($1::uuid,$2)
ON CONFLICT (user_id) DO UPDATE SET secret_enc=EXCLUDED.secret_enc,last_step=0,created_at=now()`, userID, enc)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeError(w, 500, "could not start setup")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]string{"secret": secret, "otpauth_uri": twofactor.URI(twoFactorIssuer, email, secret)})
}

func (s *Server) newBackupCodes(ctx context.Context, tx pgx.Tx, userID string) ([]string, error) {
	codes, err := twofactor.BackupCodes()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM user_totp_backup_codes WHERE user_id=$1::uuid", userID); err != nil {
		return nil, err
	}
	for _, code := range codes {
		if _, err := tx.Exec(ctx, "INSERT INTO user_totp_backup_codes(user_id,code_hash) VALUES($1::uuid,$2)", userID, twofactor.HashBackup(code)); err != nil {
			return nil, err
		}
	}
	return codes, nil
}

// revokeOtherSessions signs the user out everywhere except the browser that made the change.
func revokeOtherSessions(ctx context.Context, tx pgx.Tx, r *http.Request, userID string) error {
	cookie, err := r.Cookie("pms_session")
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM sessions WHERE user_id=$1::uuid AND token_hash<>$2", userID, security.TokenHash(cookie.Value))
	return err
}

func (s *Server) twoFactorEnable(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	userID := r.Context().Value(userKey).(string)
	guard := "2fa:" + userID
	if retry := s.mfaFails.wait(guard); retry > 0 {
		writeRateLimit(w, retry)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		writeError(w, 500, "could not turn on two-factor authentication")
		return
	}
	defer tx.Rollback(ctx)
	secret, confirmed, last, found, err := s.loadTOTP(ctx, tx, userID, true)
	switch {
	case err != nil:
		writeError(w, 500, "could not turn on two-factor authentication")
		return
	case !found:
		writeProblem(w, http.StatusConflict, "setup_required", "start the setup first")
		return
	case confirmed:
		writeProblem(w, http.StatusConflict, "already_enabled", "two-factor authentication is already on")
		return
	}
	step, ok := twofactor.Verify(secret, in.Code, time.Now(), last)
	if !ok {
		s.mfaFails.hit(guard)
		writeError(w, http.StatusUnprocessableEntity, "the code is not valid; check the time on your phone and try again")
		return
	}
	s.mfaFails.clear(guard)
	var at time.Time
	codes, err := s.newBackupCodes(ctx, tx, userID)
	if err == nil {
		err = tx.QueryRow(ctx, "UPDATE user_totp SET confirmed_at=now(),last_step=$2 WHERE user_id=$1::uuid RETURNING confirmed_at", userID, step).Scan(&at)
	}
	if err == nil {
		err = revokeOtherSessions(ctx, tx, r, userID)
	}
	if err == nil {
		err = securityEvent(ctx, tx, userID, userID, "2fa_enabled")
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeError(w, 500, "could not turn on two-factor authentication")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"enabled_at": at, "backup_codes": codes})
}

// reauthenticate checks the password and a current code for changes that weaken or reset 2FA, so a stolen session is not enough.
func (s *Server) reauthenticate(w http.ResponseWriter, r *http.Request, tx pgx.Tx, userID, password, code string) bool {
	guard := "2fa:" + userID
	if retry := max(s.mfaFails.wait(guard), s.loginAcctFails.wait("user:"+userID)); retry > 0 {
		writeRateLimit(w, retry)
		return false
	}
	var hash string
	if err := tx.QueryRow(r.Context(), "SELECT COALESCE(password_hash,'') FROM users WHERE id=$1::uuid", userID).Scan(&hash); err != nil {
		writeError(w, 500, "could not check the password")
		return false
	}
	if password == "" || code == "" || !security.CheckPassword(hash, password) {
		s.loginAcctFails.hit("user:" + userID)
		writeError(w, http.StatusUnauthorized, errNeedsPasswordFA)
		return false
	}
	ok, err := s.checkSecondFactor(r.Context(), tx, userID, code)
	if err != nil {
		writeError(w, 500, "could not check the code")
		return false
	}
	if !ok {
		s.mfaFails.hit(guard)
		writeError(w, http.StatusUnauthorized, errNeedsPasswordFA)
		return false
	}
	s.mfaFails.clear(guard)
	return true
}

type reauthInput struct {
	Password string `json:"password"`
	Code     string `json:"code"`
}

func (s *Server) twoFactorDisable(w http.ResponseWriter, r *http.Request) {
	var in reauthInput
	if !decode(w, r, &in) {
		return
	}
	userID := r.Context().Value(userKey).(string)
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		writeError(w, 500, "could not turn off two-factor authentication")
		return
	}
	defer tx.Rollback(ctx)
	if !s.reauthenticate(w, r, tx, userID, in.Password, in.Code) {
		return
	}
	err = removeTwoFactor(ctx, tx, userID)
	if err == nil {
		err = revokeOtherSessions(ctx, tx, r, userID)
	}
	if err == nil {
		err = securityEvent(ctx, tx, userID, userID, "2fa_disabled")
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeError(w, 500, "could not turn off two-factor authentication")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) twoFactorNewBackupCodes(w http.ResponseWriter, r *http.Request) {
	var in reauthInput
	if !decode(w, r, &in) {
		return
	}
	userID := r.Context().Value(userKey).(string)
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		writeError(w, 500, "could not make new backup codes")
		return
	}
	defer tx.Rollback(ctx)
	if !s.reauthenticate(w, r, tx, userID, in.Password, in.Code) {
		return
	}
	codes, err := s.newBackupCodes(ctx, tx, userID)
	if err == nil {
		err = securityEvent(ctx, tx, userID, userID, "2fa_backup_codes_renewed")
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeError(w, 500, "could not make new backup codes")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"backup_codes": codes})
}

// removeTwoFactor deletes the secret and the backup codes together; the codes belong to the user, not to the secret row.
func removeTwoFactor(ctx context.Context, tx pgx.Tx, userID string) error {
	if _, err := tx.Exec(ctx, "DELETE FROM user_totp_backup_codes WHERE user_id=$1::uuid", userID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "DELETE FROM user_totp WHERE user_id=$1::uuid", userID)
	return err
}

func hasTOTPRow(ctx context.Context, tx pgx.Tx, userID string) (bool, error) {
	var found bool
	err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM user_totp WHERE user_id=$1::uuid)", userID).Scan(&found)
	return found, err
}

// twoFactorAdminReset is the recovery procedure for a user who lost the app and the backup codes: an administrator
// removes 2FA, the user is signed out everywhere, and the action is recorded with the administrator's id.
func (s *Server) twoFactorAdminReset(w http.ResponseWriter, r *http.Request) {
	target, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		adminWriteError(w, missingAdminTarget)
		return
	}
	actor := actorOf(r)
	if actor.Type != policy.Admin {
		adminWriteError(w, noAdmin)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		writeError(w, 500, "could not reset two-factor authentication")
		return
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)", target).Scan(&exists); err != nil || !exists {
		if err != nil {
			writeError(w, 500, "could not reset two-factor authentication")
			return
		}
		adminWriteError(w, missingAdminTarget)
		return
	}
	had, err := hasTOTPRow(ctx, tx, target.String())
	if err == nil {
		err = removeTwoFactor(ctx, tx, target.String())
	}
	if err == nil && had {
		if _, err = tx.Exec(ctx, "DELETE FROM sessions WHERE user_id=$1", target); err == nil {
			err = securityEvent(ctx, tx, actor.ID, target.String(), "2fa_reset_by_admin")
		}
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeError(w, 500, "could not reset two-factor authentication")
		return
	}
	slog.WarnContext(r.Context(), "two-factor authentication reset by an administrator", "actor", actor.ID, "target", target.String(), "had_2fa", had)
	w.WriteHeader(http.StatusNoContent)
}
