package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/directory"
	mailqueue "github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/realtime"
	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/anissaefuddin/pms/server/internal/spaces"
	"github.com/anissaefuddin/pms/server/web"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	pool                *pgxpool.Pool
	cfg                 config.Config
	mail                *mailqueue.Queue
	spaces              *spaces.Service
	directory           *directory.Service
	loginIPFails        *limiter // failed logins per IP
	loginAcctFails      *limiter // failed logins per email: temporary lockout
	mfaFails            *limiter // wrong second-factor codes per user
	dummyHash           string   // checked when the email is unknown, so timing does not reveal which emails exist
	forgotLimiter       *limiter
	tokenLimiter        *limiter
	publicLimiter       *limiter // public form reads per IP
	publicSubmitLimiter *limiter // public form submissions per IP
	publicLinkLimiter   *limiter // public form submissions per link (D-42)
	requests            *prometheus.CounterVec
	duration            *prometheus.HistogramVec
	events              *realtime.Hub
	eventEpoch          string
	sseConnections      prometheus.Gauge
}

type contextKey string

const userKey contextKey = "user_id"
const userTypeKey contextKey = "user_type"

// Me matches the OpenAPI `Me` schema.
type me struct {
	AccountName              string     `json:"account_name"`
	ID                       string     `json:"id"`
	Email                    string     `json:"email"`
	FirstName                string     `json:"first_name"`
	LastName                 string     `json:"last_name"`
	UserType                 string     `json:"user_type"`
	Status                   string     `json:"status"`
	JobTitle                 *string    `json:"job_title"`
	Department               *string    `json:"department"`
	LastLoginAt              *time.Time `json:"last_login_at"`
	Locale                   string     `json:"locale"`
	Timezone                 string     `json:"timezone"`
	DateFormat               string     `json:"date_format"`
	TimeFormat               string     `json:"time_format"`
	WeekStart                string     `json:"week_start"`
	PersonalSpaceID          string     `json:"personal_space_id"`
	Company                  *string    `json:"company"`
	Country                  *string    `json:"country"`
	Location                 *string    `json:"location"`
	Phone                    *string    `json:"phone"`
	Density                  string     `json:"density"`
	Theme                    string     `json:"theme"`
	Accent                   string     `json:"accent"`
	CommentShortcut          string     `json:"comment_shortcut"`
	SidebarHidden            []string   `json:"sidebar_hidden"`
	EmailMuted               []string   `json:"email_muted"`
	OnboardingPending        bool       `json:"onboarding_pending"`
	DefaultTaskDurationDays  *int       `json:"default_task_duration_days"`
	DefaultTaskEffortMinutes *int       `json:"default_task_effort_minutes"`
	Capabilities             struct {
		ManageUsers bool `json:"manage_users"`
		CreateSpace bool `json:"create_space"`
	} `json:"capabilities"`
}

func New(pool *pgxpool.Pool, cfg config.Config, mail *mailqueue.Queue) http.Handler {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "pms_http_requests_total", Help: "HTTP requests."}, []string{"method", "route", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "pms_http_request_duration_seconds", Help: "HTTP latency."}, []string{"method", "route"})
	prometheus.MustRegister(requests, duration)
	stat := func(f func(*pgxpool.Stat) float64) func() float64 { return func() float64 { return f(pool.Stat()) } }
	prometheus.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "pms_db_pool_acquired_conns", Help: "Connections in use."}, stat(func(s *pgxpool.Stat) float64 { return float64(s.AcquiredConns()) })),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "pms_db_pool_idle_conns", Help: "Idle connections."}, stat(func(s *pgxpool.Stat) float64 { return float64(s.IdleConns()) })),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "pms_db_pool_total_conns", Help: "Open connections."}, stat(func(s *pgxpool.Stat) float64 { return float64(s.TotalConns()) })),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "pms_db_pool_max_conns", Help: "Pool size limit."}, stat(func(s *pgxpool.Stat) float64 { return float64(s.MaxConns()) })),
		prometheus.NewCounterFunc(prometheus.CounterOpts{Name: "pms_db_pool_acquire_total", Help: "Connection acquisitions."}, stat(func(s *pgxpool.Stat) float64 { return float64(s.AcquireCount()) })),
		prometheus.NewCounterFunc(prometheus.CounterOpts{Name: "pms_db_pool_empty_acquire_total", Help: "Acquisitions that had to wait because no connection was idle."}, stat(func(s *pgxpool.Stat) float64 { return float64(s.EmptyAcquireCount()) })),
	)
	prometheus.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "pms_jobs_available", Help: "River jobs waiting to run."}, func() float64 { return jobCount(pool, "available") }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "pms_jobs_failed_total", Help: "River jobs discarded after exhausting retries."}, func() float64 { return jobCount(pool, "discarded") }),
	)
	dummyHash, err := security.HashPassword("timing-only-dummy-password")
	if err != nil {
		panic(err) // only fails if the system random source does
	}
	s := &Server{pool: pool, cfg: cfg, mail: mail, spaces: spaces.New(pool), directory: directory.New(pool), loginIPFails: newLimiter(20, 10*time.Minute), loginAcctFails: newLimiter(5, 15*time.Minute), mfaFails: newLimiter(5, 15*time.Minute), dummyHash: dummyHash, forgotLimiter: newLimiter(5, time.Minute), tokenLimiter: newLimiter(10, time.Minute), publicLimiter: newLimiter(120, time.Minute), publicSubmitLimiter: newLimiter(30, time.Hour), publicLinkLimiter: newLimiter(300, time.Hour), requests: requests, duration: duration}
	mux := http.NewServeMux()
	s.events = realtime.New()
	s.eventEpoch = uuid.NewString()
	s.sseConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pms_sse_connections", Help: "Open authenticated event streams.",
	})
	prometheus.MustRegister(s.sseConnections)
	// Paths follow api/openapi.yaml (servers.url = /api/v1). Keep them in sync with the contract.
	mux.HandleFunc("GET /api/v1/healthz", s.healthz)
	mux.HandleFunc("GET /api/v1/readyz", s.readyz)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.requireUser(s.logout))
	mux.HandleFunc("GET /api/v1/me", s.requireUser(s.me))
	mux.HandleFunc("PATCH /api/v1/me", s.requireUser(s.updateMe))
	mux.HandleFunc("GET /api/v1/account", s.requireUser(s.accountInfo))
	mux.HandleFunc("PATCH /api/v1/account", s.requireUser(requireAdmin(s.renameAccount)))
	mux.HandleFunc("POST /api/v1/onboarding/complete", s.requireUser(requireAdmin(s.completeOnboarding)))
	mux.HandleFunc("GET /api/v1/me/shared", s.requireUser(s.listSharedItems))
	mux.HandleFunc("GET /api/v1/me/todo", s.requireUser(s.listMyTodo))
	mux.HandleFunc("GET /api/v1/stream", s.requireUser(s.listStream))
	mux.HandleFunc("GET /api/v1/search", s.requireUser(s.search))
	mux.HandleFunc("GET /api/v1/events", s.requireUser(s.streamEvents))
	mux.HandleFunc("POST /api/v1/auth/password/forgot", s.forgotPassword)
	mux.HandleFunc("POST /api/v1/auth/password/reset", s.resetPassword)
	mux.HandleFunc("POST /api/v1/auth/password/change", s.requireUser(s.changePassword))
	mux.HandleFunc("GET /api/v1/auth/invitations/{token}", s.getInvitation)
	mux.HandleFunc("POST /api/v1/auth/invitations/accept", s.acceptInvitation)
	mux.HandleFunc("GET /api/v1/notifications", s.requireUser(s.listNotifications))
	mux.HandleFunc("PATCH /api/v1/notifications/{notificationId}", s.requireUser(s.updateNotification))
	mux.HandleFunc("POST /api/v1/notifications/read-all", s.requireUser(func(w http.ResponseWriter, r *http.Request) { s.bulkNotifications(w, r, false) }))
	mux.HandleFunc("POST /api/v1/notifications/archive-all", s.requireUser(func(w http.ResponseWriter, r *http.Request) { s.bulkNotifications(w, r, true) }))
	s.adminRoutes(mux)
	s.spaceRoutes(mux)
	s.viewRoutes(mux)
	s.dashboardRoutes(mux)
	s.reportRoutes(mux)
	s.starRoutes(mux)
	s.commentRoutes(mux)
	s.workScheduleRoutes(mux)
	s.workflowRoutes(mux)
	s.approvalRoutes(mux)
	s.formRoutes(mux)
	s.twoFactorRoutes(mux)
	s.blueprintRoutes(mux)
	s.dependencyRoutes(mux)
	s.customFieldRoutes(mux)
	s.attachmentRoutes(mux)
	s.timeEntryRoutes(mux)
	s.projectExcelRoutes(mux)
	mux.HandleFunc("GET /api/v1/items/{itemId}/shares", s.requireUser(s.itemShares))
	mux.HandleFunc("PUT /api/v1/items/{itemId}/shares", s.requireUser(s.itemShares))
	mux.HandleFunc("POST /api/v1/items", s.requireUser(s.createItem))
	mux.HandleFunc("GET /api/v1/workflows", s.requireUser(s.listWorkflows))
	mux.HandleFunc("GET /api/v1/items/{itemId}", s.requireUser(s.getItem))
	mux.HandleFunc("GET /api/v1/items/{itemId}/children", s.requireUser(s.listChildItems))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/items", s.requireUser(s.listRootItems))
	mux.HandleFunc("PATCH /api/v1/items/{itemId}", s.requireUser(s.patchItem))
	mux.HandleFunc("DELETE /api/v1/items/{itemId}", s.requireUser(s.deleteItem))
	mux.HandleFunc("GET /api/v1/recycle-bin", s.requireUser(s.listRecycleBin))
	mux.HandleFunc("DELETE /api/v1/recycle-bin", s.requireUser(s.emptyRecycleBin))
	mux.HandleFunc("POST /api/v1/recycle-bin/{itemId}/restore", s.requireUser(s.restoreItem))
	mux.HandleFunc("POST /api/v1/items/{itemId}/reorder", s.requireUser(s.reorderItem))
	mux.HandleFunc("PUT /api/v1/items/{itemId}/locations", s.requireUser(s.replaceItemLocations))
	mux.Handle("/", static())
	return s.observe(s.requestID(recoverPanic(s.csrf(mux))))
}

// Metrics serves /metrics. It must run on its own internal port, never behind the public router.
func Metrics() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	return mux
}

func jobCount(pool *pgxpool.Pool, state string) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var count float64
	if err := pool.QueryRow(ctx, "SELECT count(*)::float8 FROM river_job WHERE state=$1", state).Scan(&count); err != nil {
		slog.Error("collect job metric", "state", state, "error", err)
		return 0
	}
	return count
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := database.Ready(ctx, s.pool); err != nil {
		slog.Error("not ready", "error", err)
		writeError(w, http.StatusServiceUnavailable, "database unavailable or migrations pending")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// loadMe builds the Me payload from the users row.
func loadMe(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID string) (me, error) {
	var m me
	var jobTitle, department string
	err := db.QueryRow(ctx, `SELECT u.id::text,u.email::text,u.first_name,u.last_name,u.user_type::text,u.status::text,u.job_title,u.department,u.last_login_at,u.locale,u.timezone,u.date_format,u.time_format,u.week_start,COALESCE((SELECT id::text FROM spaces WHERE owner_id=u.id AND kind='personal'),''),a.name,u.company,u.country,u.location,u.phone,u.density,u.theme,u.accent,u.comment_shortcut,u.sidebar_hidden,u.email_muted,u.default_task_duration_days,u.default_task_effort_minutes,u.user_type='admin' AND NOT COALESCE((a.settings->>'onboarding_done')::boolean,false) FROM users u CROSS JOIN account a WHERE u.id=$1`, userID).
		Scan(&m.ID, &m.Email, &m.FirstName, &m.LastName, &m.UserType, &m.Status, &jobTitle, &department, &m.LastLoginAt, &m.Locale, &m.Timezone, &m.DateFormat, &m.TimeFormat, &m.WeekStart, &m.PersonalSpaceID, &m.AccountName, &m.Company, &m.Country, &m.Location, &m.Phone, &m.Density, &m.Theme, &m.Accent, &m.CommentShortcut, &m.SidebarHidden, &m.EmailMuted, &m.DefaultTaskDurationDays, &m.DefaultTaskEffortMinutes, &m.OnboardingPending)
	if err != nil {
		return me{}, err
	}
	if department != "" {
		m.Department = &department
	}
	if jobTitle != "" {
		m.JobTitle = &jobTitle
	}
	m.Capabilities.ManageUsers = m.UserType == "admin"
	m.Capabilities.CreateSpace = m.UserType == "admin" || m.UserType == "regular"
	return m, nil
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID string) bool {
	plain, tokenHash, err := security.Token()
	now := time.Now()
	if err == nil {
		_, err = s.pool.Exec(r.Context(), "INSERT INTO sessions(token_hash,user_id,expires_at,absolute_expires_at) VALUES($1,$2,$3,$4)", tokenHash, userID, now.Add(config.IdleSessionTimeout), now.Add(config.AbsoluteSessionTimeout))
	}
	if err != nil {
		writeError(w, 500, "could not create session")
		return false
	}
	s.setSessionCookie(w, plain, now.Add(config.AbsoluteSessionTimeout))
	return true
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	// Only failures count, so a whole office behind one IP can log in; the per-email limit is a 15-minute lockout.
	// shortcut: in-memory and per process, fine for one replica; an attacker can lock a known email for 15 minutes.
	ip, account := s.clientIP(r), strings.ToLower(strings.TrimSpace(input.Email))
	if retry := max(s.loginIPFails.wait(ip), s.loginAcctFails.wait(account)); retry > 0 {
		writeRateLimit(w, retry)
		return
	}
	var id, hash string
	err := s.pool.QueryRow(r.Context(), `SELECT id::text,COALESCE(password_hash,'') FROM users WHERE email=$1 AND status='active'`, account).Scan(&id, &hash)
	if err != nil || hash == "" {
		hash, id = s.dummyHash, ""
	}
	if !security.CheckPassword(hash, input.Password) || id == "" {
		s.loginIPFails.hit(ip)
		s.loginAcctFails.hit(account)
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	s.loginAcctFails.clear(account)
	if on, err := s.hasTwoFactor(r.Context(), id); err != nil {
		writeError(w, 500, "could not sign in")
		return
	} else if on {
		// The password is right but the sign-in is not finished: no session until the second step succeeds.
		challenge, err := s.issueChallenge(r.Context(), id)
		if err != nil {
			writeError(w, 500, "could not sign in")
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"two_factor_required": true, "challenge": challenge})
		return
	}
	// SEC-04: a fresh login replaces the session the browser was holding instead of leaving it alive until it expires.
	if presented, err := r.Cookie("pms_session"); err == nil {
		_, _ = s.pool.Exec(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", security.TokenHash(presented.Value))
	}
	if !s.startSession(w, r, id) {
		return
	}
	_, _ = s.pool.Exec(r.Context(), "UPDATE users SET last_login_at=now() WHERE id=$1", id)
	s.writeMe(w, r, id)
}

func (s *Server) writeMe(w http.ResponseWriter, r *http.Request, userID string) {
	m, err := loadMe(r.Context(), s.pool, userID)
	if err != nil {
		if errors.Is(r.Context().Err(), context.Canceled) {
			return
		}
		writeError(w, 500, "could not load user")
		return
	}
	writeJSON(w, 200, m)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("pms_session"); err == nil {
		_, _ = s.pool.Exec(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", security.TokenHash(cookie.Value))
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	s.writeMe(w, r, r.Context().Value(userKey).(string))
}

func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	if retry := s.forgotLimiter.take(s.clientIP(r)); retry > 0 {
		writeRateLimit(w, retry)
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &input) {
		return
	}
	var id, locale string
	if err := s.pool.QueryRow(r.Context(), "SELECT id,locale FROM users WHERE email=$1 AND status='active'", input.Email).Scan(&id, &locale); err == nil {
		plain, hash, err := security.Token()
		if err == nil {
			tx, beginErr := s.pool.Begin(r.Context())
			if beginErr != nil {
				err = beginErr
			} else {
				defer tx.Rollback(r.Context())
				_, err = tx.Exec(r.Context(), "INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) VALUES($1,$2,'reset',$3)", hash, id, time.Now().Add(time.Hour))
			}
			if err == nil {
				err = s.mail.SendTx(r.Context(), tx, mailqueue.Args{To: strings.ToLower(input.Email), Template: "reset_password", URL: fmt.Sprintf("%s/reset-password?token=%s", s.cfg.PublicURL, url.QueryEscape(plain)), Locale: locale})
			}
			if err == nil {
				err = tx.Commit(r.Context())
			}
			if err != nil {
				slog.Error("queue reset email", "error", err)
			}
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	if retry := s.tokenLimiter.take(s.clientIP(r)); retry > 0 {
		writeRateLimit(w, retry)
		return
	}
	if err := security.ValidatePassword(input.Password); err != nil {
		writeError(w, 422, err.Error())
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "could not reset password")
		return
	}
	defer tx.Rollback(r.Context())
	var userID string
	err = tx.QueryRow(r.Context(), "UPDATE user_tokens SET used_at=now() WHERE token_hash=$1 AND purpose='reset' AND used_at IS NULL AND expires_at>now() RETURNING user_id", security.TokenHash(input.Token)).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusGone, "reset link is invalid or expired")
		return
	}
	if err != nil {
		writeError(w, 500, "could not reset password")
		return
	}
	// Hash only after the token proved valid, so bogus tokens never cost an argon2 run.
	hash, err := security.HashPassword(input.Password)
	if err != nil {
		writeError(w, 500, "could not reset password")
		return
	}
	tag, err := tx.Exec(r.Context(), "UPDATE users SET password_hash=$1 WHERE id=$2 AND status='active'", hash, userID)
	if err != nil {
		writeError(w, 500, "could not reset password")
		return
	}
	if tag.RowsAffected() == 0 { // deactivated after the link was issued
		writeError(w, http.StatusGone, "reset link is invalid or expired")
		return
	}
	if _, err = tx.Exec(r.Context(), "DELETE FROM user_tokens WHERE user_id=$1 AND purpose='reset'", userID); err != nil { // older emailed links die too
		writeError(w, 500, "could not reset password")
		return
	}
	if _, err = tx.Exec(r.Context(), "DELETE FROM sessions WHERE user_id=$1", userID); err != nil {
		writeError(w, 500, "could not reset password")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "could not reset password")
		return
	}
	w.WriteHeader(204)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decode(w, r, &input) {
		return
	}
	userID := r.Context().Value(userKey).(string)
	// A stolen session cookie must not allow unlimited guessing of the current password.
	guard := "user:" + userID
	if retry := s.loginAcctFails.wait(guard); retry > 0 {
		writeRateLimit(w, retry)
		return
	}
	var current string
	if err := s.pool.QueryRow(r.Context(), "SELECT COALESCE(password_hash,'') FROM users WHERE id=$1", userID).Scan(&current); err != nil || !security.CheckPassword(current, input.CurrentPassword) {
		s.loginAcctFails.hit(guard)
		writeError(w, 401, "current password is incorrect")
		return
	}
	s.loginAcctFails.clear(guard)
	if err := security.ValidatePassword(input.NewPassword); err != nil {
		writeError(w, 422, err.Error())
		return
	}
	hash, err := security.HashPassword(input.NewPassword)
	if err != nil {
		writeError(w, 500, "could not change password")
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "could not change password")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), "UPDATE users SET password_hash=$1 WHERE id=$2", hash, userID); err != nil {
		writeError(w, 500, "could not change password")
		return
	}
	if _, err = tx.Exec(r.Context(), "DELETE FROM user_tokens WHERE user_id=$1 AND purpose='reset'", userID); err != nil { // emailed links from before the change die
		writeError(w, 500, "could not change password")
		return
	}
	// Decision D-14: revoke the other sessions and rotate the caller's token, keeping its absolute expiry.
	cookie, _ := r.Cookie("pms_session")
	oldHash := security.TokenHash(cookie.Value)
	if _, err = tx.Exec(r.Context(), "DELETE FROM sessions WHERE user_id=$1 AND token_hash<>$2", userID, oldHash); err != nil {
		writeError(w, 500, "could not change password")
		return
	}
	newToken, newHash, err := security.Token()
	if err != nil {
		writeError(w, 500, "could not change password")
		return
	}
	var expires time.Time
	if err = tx.QueryRow(r.Context(), "UPDATE sessions SET token_hash=$1 WHERE token_hash=$2 RETURNING absolute_expires_at", newHash, oldHash).Scan(&expires); err != nil {
		writeError(w, 500, "could not change password")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "could not change password")
		return
	}
	s.setSessionCookie(w, newToken, expires)
	w.WriteHeader(204)
}

func (s *Server) getInvitation(w http.ResponseWriter, r *http.Request) {
	var result struct {
		Email         string    `json:"email"`
		InvitedByName string    `json:"invited_by_name,omitempty"`
		ExpiresAt     time.Time `json:"expires_at"`
	}
	err := s.pool.QueryRow(r.Context(), `SELECT u.email::text,COALESCE(NULLIF(trim(i.first_name||' '||i.last_name),''),''),t.expires_at FROM user_tokens t JOIN users u ON u.id=t.user_id LEFT JOIN users i ON i.id=t.created_by WHERE t.token_hash=$1 AND t.purpose='invite' AND t.used_at IS NULL AND t.expires_at>now() AND u.status='invited'`, security.TokenHash(r.PathValue("token"))).Scan(&result.Email, &result.InvitedByName, &result.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusGone, "invitation not found or expired")
		return
	}
	if err != nil {
		writeError(w, 500, "could not load invitation")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token       string `json:"token"`
		FirstName   string `json:"first_name"`
		LastName    string `json:"last_name"`
		Password    string `json:"password"`
		Department  string `json:"department"`
		JobTitle    string `json:"job_title"`
		AcceptTerms bool   `json:"accept_terms"`
	}
	if retry := s.tokenLimiter.take(s.clientIP(r)); retry > 0 {
		writeRateLimit(w, retry)
		return
	}
	if !decode(w, r, &input) {
		return
	}
	first, last := strings.TrimSpace(input.FirstName), strings.TrimSpace(input.LastName)
	if first == "" || last == "" || !input.AcceptTerms {
		writeError(w, 422, "first name, last name and terms acceptance are required")
		return
	}
	for _, field := range []string{first, last, input.Department, input.JobTitle} { // openapi maxLength: 80
		if utf8.RuneCountInString(field) > 80 {
			writeError(w, 422, "name, department and job title are limited to 80 characters")
			return
		}
	}
	if err := security.ValidatePassword(input.Password); err != nil {
		writeError(w, 422, err.Error())
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "could not accept invitation")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err := tx.Exec(r.Context(), "SELECT 1 FROM account WHERE id=1 FOR UPDATE"); err != nil {
		writeError(w, 500, "could not accept invitation")
		return
	}
	// Seats are guarded when inviting or reactivating (S1.6.1): an invited user already holds one, so accepting adds none.
	var userID string
	err = tx.QueryRow(r.Context(), `UPDATE user_tokens SET used_at=now() WHERE token_hash=$1 AND purpose='invite' AND used_at IS NULL AND expires_at>now() RETURNING user_id::text`, security.TokenHash(input.Token)).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusGone, "invitation is invalid or expired")
		return
	}
	if err != nil {
		writeError(w, 500, "could not accept invitation")
		return
	}
	// Hash only after the invitation proved valid, so bogus tokens never cost an argon2 run.
	passwordHash, err := security.HashPassword(input.Password)
	if err != nil {
		writeError(w, 500, "could not accept invitation")
		return
	}
	tag, err := tx.Exec(r.Context(), `UPDATE users SET first_name=$2,last_name=$3,password_hash=$4,department=$5,job_title=$6,status='active' WHERE id=$1 AND status='invited'`, userID, first, last, passwordHash, input.Department, input.JobTitle)
	if err != nil {
		writeError(w, 500, "could not accept invitation")
		return
	}
	if tag.RowsAffected() == 0 { // the user was deactivated or already active
		writeError(w, http.StatusGone, "invitation is invalid or expired")
		return
	}
	if _, err = tx.Exec(r.Context(), "INSERT INTO spaces(name,kind,owner_id) VALUES ('Personal','personal',$1)", userID); err != nil {
		writeError(w, 500, "could not create personal space")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "could not accept invitation")
		return
	}
	if !s.startSession(w, r, userID) {
		return
	}
	s.writeMe(w, r, userID)
}

func (s *Server) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("pms_session")
		if err != nil {
			writeError(w, 401, "authentication required")
			return
		}
		var userID, userType string
		var idleExpires, absoluteExpires time.Time
		err = s.pool.QueryRow(r.Context(), `SELECT u.id::text,u.user_type::text,s.expires_at,s.absolute_expires_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND u.status='active'`, security.TokenHash(cookie.Value)).Scan(&userID, &userType, &idleExpires, &absoluteExpires)
		if r.Context().Err() != nil { // the browser gave up (navigation, abort): nobody reads an answer, and it is not a 5xx
			return
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) { // a database hiccup must not log everyone out
			writeError(w, 500, "could not check session")
			return
		}
		now := time.Now()
		if err != nil || now.After(idleExpires) || now.After(absoluteExpires) {
			_, _ = s.pool.Exec(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", security.TokenHash(cookie.Value))
			s.clearSessionCookie(w)
			writeError(w, 401, "session expired")
			return
		}
		// Slide the idle limit at most once per 5 minutes, never past the absolute limit.
		target := now.Add(config.IdleSessionTimeout)
		if absoluteExpires.Before(target) {
			target = absoluteExpires
		}
		if idleExpires.Before(target.Add(-5 * time.Minute)) {
			_, _ = s.pool.Exec(r.Context(), "UPDATE sessions SET expires_at=$2 WHERE token_hash=$1", security.TokenHash(cookie.Value), target)
		}
		next(w, r.WithContext(context.WithValue(context.WithValue(r.Context(), userKey, userID), userTypeKey, userType)))
	}
}

func (s *Server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin == "" || strings.TrimRight(origin, "/") != s.cfg.PublicURL {
			writeError(w, 403, "invalid request origin")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: "pms_session", Value: value, Path: "/", HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: expires})
}
func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "pms_session", Path: "/", HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, 400, "invalid request body")
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeError(w, 400, "request body must contain one JSON object")
		return false
	}
	return true
}

// problemCodes maps HTTP status to the stable error codes in WORKFLOWS §10.
var problemCodes = map[int]string{400: "validation_failed", 401: "unauthenticated", 403: "forbidden", 404: "not_found", 410: "token_invalid", 422: "validation_failed", 409: "version_conflict", 428: "precondition_required", 429: "rate_limited"}

func writeError(w http.ResponseWriter, status int, message string) {
	code := problemCodes[status]
	if code == "" {
		code = "internal_error"
	}
	writeProblem(w, status, code, message)
}

func writeProblem(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": http.StatusText(status), "status": status, "code": code, "detail": message, "request_id": w.Header().Get("X-Request-ID")})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (s *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		s.requests.WithLabelValues(r.Method, route, fmt.Sprint(sw.status)).Inc()
		s.duration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
		slog.Info("request", "request_id", w.Header().Get("X-Request-ID"), "method", r.Method, "route", route, "status", sw.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

// recoverPanic turns a handler panic into a problem+json 500 and a log line instead of a dropped connection.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("handler panic", "request_id", w.Header().Get("X-Request-ID"), "route", r.Pattern, "panic", fmt.Sprint(v), "stack", string(debug.Stack()))
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			plain, _, _ := security.Token()
			id = plain[:16]
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

func static() http.Handler {
	assets, _ := fs.Sub(web.Dist, "dist")
	return staticHandler(assets)
}

// staticHandler serves the built SPA. Hashed files under /assets/ never change, so they are cached for a year;
// index.html must be revalidated so a new release is picked up. A missing /assets/ file is a real 404: answering with
// index.html would give the browser HTML where it expects a script, which surfaces as a confusing MIME error.
func staticHandler(assets fs.FS) http.Handler {
	files := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, 404, "not found")
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := assets.Open(path); err == nil {
				f.Close()
				if strings.HasPrefix(path, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(path, "assets/") {
				http.NotFound(w, r)
				return
			}
		}
		data, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			http.Error(w, "web build missing", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(data)
	})
}

func (s *Server) clientIP(r *http.Request) string {
	// Only behind a reverse proxy that overwrites the header; otherwise any client could pick its own IP.
	if s.cfg.TrustProxy {
		if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
			return forwarded
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type limiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	entries map[string]*limitEntry
}
type limitEntry struct {
	count int
	reset time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, entries: map[string]*limitEntry{}}
}

// hit records a failed attempt.
func (l *limiter) hit(key string) { l.record(key) }

func (l *limiter) record(key string) (int, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.entries) > 10_000 {
		for k, candidate := range l.entries {
			if now.After(candidate.reset) {
				delete(l.entries, k)
			}
		}
		// Still full of live entries (an IP flood): drop arbitrary ones so memory stays bounded.
		for k := range l.entries {
			if len(l.entries) <= 10_000 {
				break
			}
			delete(l.entries, k)
		}
	}
	entry := l.entries[key]
	if entry == nil || now.After(entry.reset) {
		l.entries[key] = &limitEntry{count: 1, reset: now.Add(l.window)}
		return 1, l.window
	}
	entry.count++
	return entry.count, entry.reset.Sub(now)
}

func (l *limiter) take(key string) time.Duration {
	count, remaining := l.record(key)
	if count > l.max {
		return remaining
	}
	return 0
}

func (l *limiter) wait(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if entry := l.entries[key]; entry != nil && entry.count >= l.max {
		return time.Until(entry.reset)
	}
	return 0
}

func writeRateLimit(w http.ResponseWriter, retry time.Duration) {
	seconds := int64((retry + time.Second - 1) / time.Second)
	w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
	writeError(w, http.StatusTooManyRequests, "too many attempts, try again later")
}

func (l *limiter) clear(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}
