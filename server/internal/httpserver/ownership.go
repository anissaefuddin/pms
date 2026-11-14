package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Account ownership (SET-09, D-97). The owner answers for the account: the person other administrators cannot demote or switch off,
// and the only one who can hand the role to someone else. Handing it over changes nobody's administrator access.

const (
	auditOwnership      = "account.ownership_transferred"
	recentSignInWindow  = 15 * time.Minute
	errNotOwner         = "not_owner"
	errRecentSignInCode = "recent_sign_in_required"
)

var ownerProtected = adminError{409, "account_owner", "the account owner cannot be demoted or deactivated; transfer ownership first"}

func (s *Server) ownershipRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/account/ownership", s.requireUser(requireAdmin(s.getOwnership)))
	mux.HandleFunc("POST /api/v1/account/ownership", s.requireUser(requireAdmin(s.transferOwnership)))
}

type ownerView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *Server) getOwnership(w http.ResponseWriter, r *http.Request) {
	ctx, actor := r.Context(), actorOf(r)
	var owner *ownerView
	var o ownerView
	if err := s.pool.QueryRow(ctx, `SELECT u.id::text,btrim(u.first_name||' '||u.last_name),u.email::text FROM account a JOIN users u ON u.id=a.owner_id WHERE a.id=1`).Scan(&o.ID, &o.Name, &o.Email); err == nil {
		owner = &o
	} else if err != pgx.ErrNoRows {
		adminWriteError(w, err)
		return
	}
	isOwner := owner != nil && owner.ID == actor.ID
	candidates := []ownerView{}
	if isOwner { // only the owner needs the list: it is who the role can go to
		rows, err := s.pool.Query(ctx, `SELECT id::text,btrim(first_name||' '||last_name),email::text FROM users
WHERE status='active' AND user_type='admin' AND id<>$1::uuid AND id<>$2::uuid ORDER BY first_name,last_name,email`, actor.ID, externalRequesterID)
		if err != nil {
			adminWriteError(w, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var c ownerView
			if err := rows.Scan(&c.ID, &c.Name, &c.Email); err != nil {
				adminWriteError(w, err)
				return
			}
			candidates = append(candidates, c)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"owner": owner, "is_owner": isOwner, "candidates": candidates})
}

func (s *Server) transferOwnership(w http.ResponseWriter, r *http.Request) {
	var in struct {
		NewOwnerID string `json:"new_owner_id"`
		Password   string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	target, err := uuid.Parse(in.NewOwnerID)
	actor := actorOf(r)
	if err != nil || target.String() == actor.ID {
		writeError(w, 422, "choose another administrator to hand the account to")
		return
	}
	var email string
	if err := s.pool.QueryRow(r.Context(), "SELECT email::text FROM users WHERE id=$1", actor.ID).Scan(&email); err != nil {
		adminWriteError(w, err)
		return
	}
	if retry := s.loginAcctFails.wait(email); retry > 0 { // guessing the password here draws on the same budget as signing in
		writeRateLimit(w, retry)
		return
	}
	err = s.adminTx(r, func(ctx context.Context, tx pgx.Tx) error {
		var owner *string
		if err := tx.QueryRow(ctx, "SELECT owner_id::text FROM account WHERE id=1 FOR UPDATE").Scan(&owner); err != nil {
			return err
		}
		if owner == nil || *owner != actor.ID {
			return adminError{403, errNotOwner, "only the account owner can transfer ownership"}
		}
		var ok bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active' AND user_type='admin' AND id<>$2::uuid)", target, externalRequesterID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return adminError{422, "validation_failed", "the new owner must be an active administrator"}
		}
		if err := s.confirmOwner(ctx, tx, r, actor.ID, email, in.Password); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "UPDATE account SET owner_id=$1 WHERE id=1", target); err != nil {
			return err
		}
		return s.auditIn(ctx, tx, r, actor.ID, "", auditEvent{Action: auditOwnership, TargetType: "account", TargetID: "1", Details: map[string]any{"from": actor.ID, "to": target.String()}})
	})
	if err != nil {
		adminWriteError(w, err)
		return
	}
	s.getOwnership(w, r)
}

// confirmOwner asks the owner to prove it is them at the keyboard: their password, or, for someone who has none (single sign-on,
// emailed codes), a sign-in from the last quarter hour.
func (s *Server) confirmOwner(ctx context.Context, tx pgx.Tx, r *http.Request, userID, email, password string) error {
	var hash string
	if err := tx.QueryRow(ctx, "SELECT COALESCE(password_hash,'') FROM users WHERE id=$1", userID).Scan(&hash); err != nil {
		return err
	}
	if hash != "" {
		if !security.CheckPassword(hash, password) {
			s.loginAcctFails.hit(email)
			return adminError{401, "invalid_password", "the password is not correct"}
		}
		s.loginAcctFails.clear(email)
		return nil
	}
	cookie, err := r.Cookie("pms_session")
	if err != nil {
		return noAdmin
	}
	var created time.Time
	if err := tx.QueryRow(ctx, "SELECT created_at FROM sessions WHERE token_hash=$1", security.TokenHash(cookie.Value)).Scan(&created); err != nil {
		return err
	}
	if time.Since(created) > recentSignInWindow {
		return adminError{403, errRecentSignInCode, "sign in again, then transfer ownership within 15 minutes"}
	}
	return nil
}
