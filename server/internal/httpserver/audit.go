package httpserver

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/reqid"
	"github.com/anissaefuddin/pms/server/internal/spaces"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Audit actions ("area.verb"). Keep this list and docs/architecture/DECISIONS.md D-90 in step.
const (
	auditLogin             = "auth.login"
	auditLoginFailed       = "auth.login_failed"
	auditLogout            = "auth.logout"
	auditPasswordChanged   = "auth.password_changed"
	auditPasswordReset     = "auth.password_reset"
	auditUserInvited       = "user.invited"
	auditUserType          = "user.type_changed"
	auditSpaceLocked       = "policy.space_locked"
	auditSpaceUnlocked     = "policy.space_unlocked"
	auditSpaceVisibility   = "space.visibility_changed"
	auditSpaceArchived     = "space.archived"
	auditSpaceRestored     = "space.restored"
	auditSpaceAdminGranted = "space.admin_granted"
	auditSpaceAdminRemoved = "space.admin_removed"
	auditSpaceOwnership    = "space.ownership_transferred"
	auditSpaceJoined       = "space.joined"
	auditOperationalRead   = "report.operational_read"
	auditOperationalDenied = "report.operational_denied"
	auditCreateMenu        = "space.create_menu_changed"
	auditUserDeactivated   = "user.deactivated"
	auditUserReactivated   = "user.reactivated"
	auditShareSpace        = "share.space_changed"
	auditShareItem         = "share.item_changed"
	auditExported          = "audit.exported"
	auditAPIKeyCreated     = "api_key.created"
	auditAPIKeyRotated     = "api_key.rotated"
	auditAPIKeyRevoked     = "api_key.revoked"
	auditWebhookCreated    = "webhook.created"
	auditWebhookUpdated    = "webhook.updated"
	auditWebhookDeleted    = "webhook.deleted"
	auditOAuthAppCreated   = "oauth.app_created"
	auditOAuthAppRemoved   = "oauth.app_removed"
	auditOAuthGrantRevoked = "oauth.grant_revoked"
	auditCustomFieldGrants = "custom_field.grants_changed"
)

type auditEvent struct {
	Action, TargetType, TargetID string
	Details                      map[string]any
}

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// auditIn records an event inside the caller's transaction, so a change and its trail succeed or fail together.
// label names an actor who is not a user (a failed login's email); a known actor's own email is stored instead.
func (s *Server) auditIn(ctx context.Context, db execer, r *http.Request, actor, label string, ev auditEvent) error {
	details := ev.Details
	if details == nil {
		details = map[string]any{}
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	ip := ""
	if r != nil {
		ip = s.clientIP(r)
	}
	_, err = db.Exec(ctx, `INSERT INTO audit_events(actor_id,actor_name,action,target_type,target_id,ip,details,request_id)
VALUES (NULLIF($1,'')::uuid, COALESCE((SELECT email::text FROM users WHERE id=NULLIF($1,'')::uuid), $2), $3, $4, $5, $6, $7, $8)`,
		actor, clip(label, 200), ev.Action, ev.TargetType, clip(ev.TargetID, 80), clip(ip, 64), raw, reqid.From(ctx))
	return err
}

// audit records an event outside any transaction (a login, a logout). A failure is logged and never blocks the request itself.
// shortcut: best effort, so a database hiccup can lose a login entry; move to the caller's transaction if login ever writes one.
func (s *Server) audit(r *http.Request, actor, label string, ev auditEvent) {
	if err := s.auditIn(r.Context(), s.pool, r, actor, label, ev); err != nil && r.Context().Err() == nil {
		slog.ErrorContext(r.Context(), "audit event lost", "action", ev.Action, "error", err)
	}
}

func (s *Server) auditRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/audit-events", s.requireUser(requireAdmin(s.listAudit)))
	mux.HandleFunc("GET /api/v1/audit-events/export", s.requireUser(requireAdmin(s.exportAudit)))
}

type auditRow struct {
	ID         int64           `json:"id"`
	At         time.Time       `json:"at"`
	ActorID    *string         `json:"actor_id"`
	ActorName  string          `json:"actor_name"`
	Action     string          `json:"action"`
	TargetType string          `json:"target_type"`
	TargetID   string          `json:"target_id"`
	IP         string          `json:"ip"`
	Details    json.RawMessage `json:"details"`
	RequestID  string          `json:"request_id"`
}

type auditFilter struct {
	action, category string
	requestID        string
	actor            *uuid.UUID
	from, to         *time.Time
	before           int64
}

func parseAuditFilter(r *http.Request) (auditFilter, string) {
	q := r.URL.Query()
	f := auditFilter{action: q.Get("action"), category: q.Get("category"), requestID: q.Get("request_id")}
	if len(f.action) > 80 || len(f.category) > 40 || len(f.requestID) > 64 {
		return f, "action, category or request_id is too long"
	}
	if raw := q.Get("actor_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return f, "actor_id must be a UUID"
		}
		f.actor = &id
	}
	for key, dst := range map[string]**time.Time{"from": &f.from, "to": &f.to} {
		if raw := q.Get(key); raw != "" {
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return f, key + " must be an RFC 3339 time"
			}
			*dst = &t
		}
	}
	if raw := q.Get("cursor"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 1 {
			return f, "cursor is not valid"
		}
		f.before = n
	}
	return f, ""
}

func (s *Server) queryAudit(ctx context.Context, f auditFilter, limit int) ([]auditRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, at, actor_id::text, actor_name, action, target_type, target_id, ip, details, request_id FROM audit_events
WHERE ($1::bigint = 0 OR id < $1)
  AND ($2 = '' OR action = $2)
  AND ($3 = '' OR action LIKE replace(replace(replace($3,'\','\\'),'%','\%'),'_','\_') || '.%')
  AND ($4::uuid IS NULL OR actor_id = $4)
  AND ($5::timestamptz IS NULL OR at >= $5)
  AND ($6::timestamptz IS NULL OR at <= $6)
  AND ($8 = '' OR request_id = $8)
ORDER BY id DESC LIMIT $7`, f.before, f.action, f.category, f.actor, f.from, f.to, limit, f.requestID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[auditRow])
}

const auditPage, auditExportMax = 50, 10000

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	f, bad := parseAuditFilter(r)
	if bad != "" {
		writeError(w, http.StatusUnprocessableEntity, bad)
		return
	}
	limit := auditPage
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 {
			writeError(w, http.StatusUnprocessableEntity, "limit must be between 1 and 200")
			return
		}
		limit = n
	}
	rows, err := s.queryAudit(r.Context(), f, limit+1)
	if err != nil {
		slog.ErrorContext(r.Context(), "list audit", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		c := strconv.FormatInt(rows[limit-1].ID, 10)
		next = &c
	}
	if rows == nil {
		rows = []auditRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rows, "next_cursor": next})
}

// exportAudit streams up to 10,000 matching events as CSV. Reading the trail in bulk is itself recorded.
func (s *Server) exportAudit(w http.ResponseWriter, r *http.Request) {
	f, bad := parseAuditFilter(r)
	if bad != "" {
		writeError(w, http.StatusUnprocessableEntity, bad)
		return
	}
	rows, err := s.queryAudit(r.Context(), f, auditExportMax)
	if err != nil {
		slog.ErrorContext(r.Context(), "export audit", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.audit(r, actorOf(r).ID, "", auditEvent{Action: auditExported, TargetType: "audit", Details: map[string]any{"rows": len(rows)}})
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="audit-events.csv"`)
	out := csv.NewWriter(w)
	_ = out.Write([]string{"time", "actor", "action", "target_type", "target_id", "ip", "details", "request_id"})
	for _, e := range rows {
		_ = out.Write([]string{e.At.UTC().Format(time.RFC3339), csvCell(e.ActorName), e.Action, e.TargetType, csvCell(e.TargetID), e.IP, csvCell(string(e.Details)), csvCell(e.RequestID)})
	}
	out.Flush()
}

// auditShares records what a share replacement changed: who was added, removed, or given another role. Inherited rows are not changes.
func (s *Server) auditShares(r *http.Request, action, targetType, targetID string, before, after []spaces.Share) {
	type entry struct {
		PrincipalType string `json:"principal_type"`
		PrincipalID   string `json:"principal_id"`
		Role          string `json:"role"`
		From          string `json:"from,omitempty"`
	}
	key := func(x spaces.Share) string { return x.PrincipalType + ":" + x.PrincipalID }
	old := map[string]spaces.Share{}
	for _, x := range before {
		if x.InheritedFrom == nil {
			old[key(x)] = x
		}
	}
	var added, changed, removed []entry
	seen := map[string]bool{}
	for _, x := range after {
		if x.InheritedFrom != nil {
			continue
		}
		seen[key(x)] = true
		switch prev, ok := old[key(x)]; {
		case !ok:
			added = append(added, entry{x.PrincipalType, x.PrincipalID, x.Role, ""})
		case prev.Role != x.Role:
			changed = append(changed, entry{x.PrincipalType, x.PrincipalID, x.Role, prev.Role})
		}
	}
	for k, x := range old {
		if !seen[k] {
			removed = append(removed, entry{x.PrincipalType, x.PrincipalID, x.Role, ""})
		}
	}
	if len(added)+len(changed)+len(removed) == 0 {
		return
	}
	s.audit(r, actorOf(r).ID, "", auditEvent{Action: action, TargetType: targetType, TargetID: targetID,
		Details: map[string]any{"added": added, "changed": changed, "removed": removed}})
}
