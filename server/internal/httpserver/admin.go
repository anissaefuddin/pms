package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	stdmail "net/mail"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type adminError struct {
	status        int
	code, message string
}

func (e adminError) Error() string { return e.message }

var (
	noAdmin            = adminError{403, "forbidden", "administrator access is required"}
	missingAdminTarget = adminError{404, "not_found", "not found"}
	seatsExhausted     = adminError{422, "seats_exhausted", "no seats available"}
	lastAdmin          = adminError{409, "last_admin", "at least one active administrator is required"}
)

func adminWriteError(w http.ResponseWriter, err error) {
	var problem adminError
	switch {
	case errors.As(err, &problem):
		writeProblem(w, problem.status, problem.code, problem.message)
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, 404, "not found")
	default:
		slog.ErrorContext(logContext(w), "admin operation failed", "error", err)
		writeError(w, 500, "could not complete admin operation")
	}
}

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if actorOf(r).Type != policy.Admin {
			adminWriteError(w, noAdmin)
			return
		}
		next(w, r)
	}
}

func (s *Server) adminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/users/{userId}", s.requireUser(s.adminGetUser))
	mux.HandleFunc("POST /api/v1/users/invitations", s.requireUser(requireAdmin(s.adminInvite)))
	mux.HandleFunc("PATCH /api/v1/users/{userId}", s.requireUser(requireAdmin(s.adminChangeUser)))
	mux.HandleFunc("POST /api/v1/users/{userId}/deactivate", s.requireUser(requireAdmin(s.adminChangeUser)))
	mux.HandleFunc("POST /api/v1/users/{userId}/reactivate", s.requireUser(requireAdmin(s.adminChangeUser)))
	mux.HandleFunc("POST /api/v1/users/{userId}/invitation/resend", s.requireUser(requireAdmin(s.adminChangeUser)))
	mux.HandleFunc("GET /api/v1/seats", s.requireUser(requireAdmin(s.adminSeats)))
	mux.HandleFunc("GET /api/v1/users/types", s.requireUser(requireAdmin(func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, policy.Definition()) })))
	mux.HandleFunc("POST /api/v1/groups", s.requireUser(requireAdmin(s.adminGroup)))
	mux.HandleFunc("PUT /api/v1/groups/{groupId}", s.requireUser(requireAdmin(s.adminGroup)))
	mux.HandleFunc("DELETE /api/v1/groups/{groupId}", s.requireUser(requireAdmin(s.adminGroup)))
}

// Lock the singleton account before checking a seat or administrator invariant.
func (s *Server) adminTx(r *http.Request, work func(context.Context, pgx.Tx) error) error {
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT 1 FROM account WHERE id=1 FOR UPDATE"); err != nil {
		return err
	}
	var ok bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active' AND user_type='admin')", actorOf(r).ID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return noAdmin
	}
	if err = work(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type adminQueryRow interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type seatUse struct {
	Max       int `json:"max"`
	Used      int `json:"used"`
	Pending   int `json:"pending"`
	Available int `json:"available"`
}

func (s *Server) seatUsage(ctx context.Context, db adminQueryRow) (seatUse, error) {
	out := seatUse{Max: s.cfg.MaxSeats}
	err := db.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='active'),count(*) FILTER (WHERE status='invited') FROM users`).Scan(&out.Used, &out.Pending)
	out.Available = max(0, out.Max-out.Used-out.Pending)
	return out, err
}

func (s *Server) ensureSeat(ctx context.Context, tx pgx.Tx) error {
	seats, err := s.seatUsage(ctx, tx)
	if err != nil {
		return err
	}
	if seats.Available == 0 {
		return seatsExhausted
	}
	return nil
}

func (s *Server) adminSeats(w http.ResponseWriter, r *http.Request) {
	seats, err := s.seatUsage(r.Context(), s.pool)
	if err != nil {
		adminWriteError(w, err)
		return
	}
	writeJSON(w, 200, seats)
}

func adminUser(ctx context.Context, db adminQueryRow, id string, all bool) (json.RawMessage, error) {
	var user json.RawMessage
	err := db.QueryRow(ctx, `SELECT jsonb_build_object('id',id,'email',email,'first_name',first_name,'last_name',last_name,
'user_type',user_type,'status',status,'job_title',NULLIF(job_title,''),'department',NULLIF(department,''),'last_login_at',last_login_at)
FROM users WHERE id=$1::uuid AND ($2 OR status='active')`, id, all).Scan(&user)
	return user, err
}

func (s *Server) adminGetUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("userId"))
	actor := actorOf(r)
	if err != nil || (!policy.CanBrowseDirectory(actor.Type) && id.String() != actor.ID) {
		adminWriteError(w, missingAdminTarget)
		return
	}
	user, err := adminUser(r.Context(), s.pool, id.String(), actor.Type == policy.Admin)
	if err != nil {
		adminWriteError(w, err)
		return
	}
	writeJSON(w, 200, user)
}

type inviteShare struct {
	Type string `json:"target_type"`
	ID   string `json:"target_id"`
	Role string `json:"role"`
}

// mayInviteInto is the rule of the share endpoints: Share on the target, and nothing beyond one's own rights there.
func mayInviteInto(ctx context.Context, tx pgx.Tx, actor policy.Actor, share inviteShare) error {
	var caps policy.Caps
	var err error
	if share.Type == "space" {
		var access policy.SpaceAccess
		access, _, err = policy.SpaceAccessFor(ctx, tx, actor, share.ID)
		caps = access.Caps
	} else {
		caps, err = policy.ItemCaps(ctx, tx, actor, share.ID)
	}
	if err != nil {
		return err
	}
	role := policy.RoleCaps(policy.Role(share.Role))
	ok := caps.Has(policy.Share) && caps.Contains(role)
	if ok && share.Type == "item" && role.Has(policy.Share) {
		ok, err = items.CanReassign(ctx, tx, actor, share.ID)
	}
	if err == nil && !ok {
		err = adminError{403, "forbidden", "you can only invite people into what you may share yourself"}
	}
	return err
}

func (s *Server) adminInvite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string          `json:"email"`
		UserType policy.UserType `json:"user_type"`
		Message  string          `json:"message"`
		Share    []inviteShare   `json:"share"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	address, err := stdmail.ParseAddress(in.Email)
	if err != nil || address.Address != in.Email || len(in.Email) > 254 || !slices.Contains(policy.UserTypes, in.UserType) || utf8.RuneCountInString(in.Message) > 500 || len(in.Share) > 500 {
		writeError(w, 422, "valid email, user type and invitation message are required")
		return
	}
	for i := range in.Share {
		entry := &in.Share[i]
		id, err := uuid.Parse(entry.ID)
		if err != nil || !policy.ValidRole(entry.Role) || (entry.Type != "space" && entry.Type != "item") {
			writeError(w, 422, "invalid invitation share")
			return
		}
		entry.ID = id.String()
	}
	var user json.RawMessage
	err = s.adminTx(r, func(ctx context.Context, tx pgx.Tx) error {
		var duplicate bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)", in.Email).Scan(&duplicate); err != nil {
			return err
		}
		if duplicate {
			return adminError{409, "conflict", "email already exists"}
		}
		if err := s.ensureSeat(ctx, tx); err != nil {
			return err
		}
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO users(email,user_type,invitation_message,invited_by)
VALUES($1,$2,$3,$4) RETURNING id::text`, in.Email, in.UserType, in.Message, actorOf(r).ID).Scan(&id); err != nil {
			return err
		}
		// An invitation grants no more than the inviter could grant by sharing (D-17.1, DEF-045). Checked before any share is written.
		for _, share := range in.Share {
			if err := mayInviteInto(ctx, tx, actorOf(r), share); err != nil {
				return err
			}
		}
		for _, share := range in.Share {
			if _, err := tx.Exec(ctx, `INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,created_by)
VALUES($1,$2,'user',$3,$4,$5)`, share.Type, share.ID, id, share.Role, actorOf(r).ID); err != nil {
				var pg *pgconn.PgError
				if errors.As(err, &pg) && (pg.Code == "23503" || pg.Code == "23505") {
					return adminError{422, "validation_failed", "invalid or duplicate invitation share"}
				}
				return err
			}
		}
		// The locked-Space rule of the share endpoints (D-95), read after every share of this invitation exists (DEF-047).
		for _, share := range in.Share {
			if share.Type != "item" {
				continue
			}
			ok, err := policy.LockedAllows(ctx, tx, share.ID, "user", id)
			if err != nil {
				return err
			}
			if !ok {
				return adminError{403, "forbidden", policy.ErrLockedSpace.Error()}
			}
		}
		if err := s.mail.InviteTx(ctx, tx, id); err != nil {
			return err
		}
		if err := s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditUserInvited, TargetType: "user", TargetID: id, Details: map[string]any{"user_type": in.UserType}}); err != nil {
			return err
		}
		var err error
		user, err = adminUser(ctx, tx, id, true)
		return err
	})
	if err != nil {
		adminWriteError(w, err)
		return
	}
	writeJSON(w, 201, user)
}

func (s *Server) adminChangeUser(w http.ResponseWriter, r *http.Request) {
	parsed, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		adminWriteError(w, missingAdminTarget)
		return
	}
	id := parsed.String()
	action := "type"
	if r.Method == http.MethodPost {
		_, action, _ = strings.Cut(r.URL.Path, "/users/"+r.PathValue("userId")+"/")
	}
	var in struct {
		UserType policy.UserType `json:"user_type"`
	}
	if action == "type" {
		if !decode(w, r, &in) {
			return
		}
		if !slices.Contains(policy.UserTypes, in.UserType) {
			writeError(w, 422, "invalid user type")
			return
		}
	}
	var user json.RawMessage
	err = s.adminTx(r, func(ctx context.Context, tx pgx.Tx) error {
		var status string
		var current policy.UserType
		var password bool
		if err := tx.QueryRow(ctx, "SELECT status::text,user_type::text,password_hash IS NOT NULL FROM users WHERE id=$1 FOR UPDATE", id).Scan(&status, &current, &password); err != nil {
			return err
		}
		if status == "active" && current == policy.Admin && (action == "deactivate" || (action == "type" && in.UserType != policy.Admin)) {
			var other bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE status='active' AND user_type='admin' AND id<>$1)", id).Scan(&other); err != nil {
				return err
			}
			if !other {
				return lastAdmin
			}
		}
		if action == "deactivate" || (action == "type" && in.UserType != policy.Admin) {
			var isOwner bool
			if err := tx.QueryRow(ctx, "SELECT COALESCE(owner_id=$1::uuid,false) FROM account WHERE id=1", id).Scan(&isOwner); err != nil {
				return err
			}
			if isOwner && status == "active" {
				return ownerProtected
			}
		}
		revoke := false
		switch action {
		case "type":
			if in.UserType != current {
				if _, err := tx.Exec(ctx, "UPDATE users SET user_type=$2 WHERE id=$1", id, in.UserType); err != nil {
					return err
				}
				if err := s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditUserType, TargetType: "user", TargetID: id, Details: map[string]any{"from": current, "to": in.UserType}}); err != nil {
					return err
				}
				revoke = true
			}
		case "deactivate":
			if status != "deactivated" {
				if _, err := tx.Exec(ctx, "UPDATE users SET status='deactivated',invitation_generation=invitation_generation+1 WHERE id=$1", id); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, "DELETE FROM user_tokens WHERE user_id=$1 AND used_at IS NULL", id); err != nil {
					return err
				}
				if err := s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditUserDeactivated, TargetType: "user", TargetID: id}); err != nil {
					return err
				}
				revoke = true
			}
		case "reactivate":
			if status == "deactivated" {
				if err := s.ensureSeat(ctx, tx); err != nil {
					return err
				}
				newStatus := "active"
				if !password {
					newStatus = "invited"
				}
				if _, err := tx.Exec(ctx, "UPDATE users SET status=$2 WHERE id=$1", id, newStatus); err != nil {
					return err
				}
				if !password {
					if err := s.mail.InviteTx(ctx, tx, id); err != nil {
						return err
					}
				}
				if err := s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditUserReactivated, TargetType: "user", TargetID: id}); err != nil {
					return err
				}

			}
		case "invitation/resend":
			if status != "invited" {
				return adminError{409, "conflict", "only pending invitations can be resent"}
			}
			return s.mail.InviteTx(ctx, tx, id)
		default:
			return missingAdminTarget
		}
		if revoke {
			if _, err := tx.Exec(ctx, "DELETE FROM sessions WHERE user_id=$1", id); err != nil {
				return err
			}
		}
		var err error
		user, err = adminUser(ctx, tx, id, true)
		return err
	})
	if err != nil {
		adminWriteError(w, err)
		return
	}
	if action == "invitation/resend" {
		w.WriteHeader(204)
		return
	}
	writeJSON(w, 200, user)
}

func (s *Server) adminGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("groupId")
	if r.Method != http.MethodPost {
		parsed, err := uuid.Parse(id)
		if err != nil {
			adminWriteError(w, missingAdminTarget)
			return
		}
		id = parsed.String()
	}
	var in struct {
		Name    string      `json:"name"`
		Members []uuid.UUID `json:"member_ids"`
	}
	if r.Method != http.MethodDelete {
		if !decode(w, r, &in) {
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		if n := utf8.RuneCountInString(in.Name); n < 1 || n > 80 || (r.Method == http.MethodPut && in.Members == nil) {
			writeError(w, 422, "a group name and member list are required")
			return
		}
	}
	var group json.RawMessage
	err := s.adminTx(r, func(ctx context.Context, tx pgx.Tx) error {
		switch r.Method {
		case http.MethodDelete:
			if _, err := tx.Exec(ctx, "DELETE FROM shares WHERE principal_type='group' AND principal_id=$1", id); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, "DELETE FROM groups WHERE id=$1", id)
			if err == nil && tag.RowsAffected() == 0 {
				return missingAdminTarget
			}
			return err
		case http.MethodPost:
			if err := tx.QueryRow(ctx, "INSERT INTO groups(name) VALUES($1) RETURNING id::text", in.Name).Scan(&id); err != nil {
				return err
			}
		case http.MethodPut:
			tag, err := tx.Exec(ctx, "UPDATE groups SET name=$2 WHERE id=$1", id, in.Name)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return missingAdminTarget
			}
			if _, err := tx.Exec(ctx, "DELETE FROM group_members WHERE group_id=$1", id); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO group_members(group_id,user_id) SELECT $1::uuid,m FROM unnest($2::uuid[]) m ON CONFLICT DO NOTHING`, id, in.Members); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT jsonb_build_object('id',g.id,'name',g.name,'member_ids',COALESCE(
  (SELECT jsonb_agg(m.user_id ORDER BY m.user_id) FROM group_members m WHERE m.group_id=g.id),'[]'::jsonb))
FROM groups g WHERE g.id=$1`, id).Scan(&group)
	})
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) {
			switch pg.Code {
			case "23505":
				writeError(w, 409, "group name already exists")
				return
			case "23503":
				writeError(w, 422, "group member does not exist")
				return
			}
		}
		adminWriteError(w, err)
		return
	}
	switch r.Method {
	case http.MethodDelete:
		w.WriteHeader(204)
	case http.MethodPost:
		writeJSON(w, 201, group)
	default:
		writeJSON(w, 200, group)
	}
}
