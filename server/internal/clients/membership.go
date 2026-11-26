package clients

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Eligible user types and statuses. Staff are admin and regular, the same people policy.CanBrowseDirectory calls internal.
// External, collaborator and viewer users are partners and are out of scope until K-19 is decided.
// A user must also be active: invited and deactivated users cannot receive new eligibility.
func eligible(status, userType string) bool {
	return status == "active" && (userType == "admin" || userType == "regular")
}

const membershipColumns = `id::text, client_id::text, user_id::text, status, granted_at, granted_by::text, revoked_at, revoked_by::text`

func scanMembership(row pgx.Row) (Membership, error) {
	var m Membership
	err := row.Scan(&m.ID, &m.ClientID, &m.UserID, &m.Status, &m.GrantedAt, &m.GrantedBy, &m.RevokedAt, &m.RevokedBy)
	return m, err
}

// ListMembers returns the active memberships, or with history true every row including revoked ones.
// Revoked rows are never presented as current eligibility. Credentials and unrelated user fields are not selected.
func (s *Service) ListMembers(ctx context.Context, clientID string, history bool) ([]Membership, error) {
	if _, err := uuid.Parse(clientID); err != nil {
		return nil, ErrNotFound
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clients WHERE id=$1::uuid)`, clientID).Scan(&exists); err != nil {
		return nil, mapCustomerError(err)
	}
	if !exists {
		return nil, ErrNotFound
	}
	rows, err := s.pool.Query(ctx, `
SELECT m.id::text, m.client_id::text, m.user_id::text, m.status, m.granted_at, m.granted_by::text, m.revoked_at, m.revoked_by::text,
       COALESCE(NULLIF(btrim(u.first_name||' '||u.last_name),''), '')
FROM client_memberships m JOIN users u ON u.id = m.user_id
WHERE m.client_id=$1::uuid AND ($2::boolean OR m.status='active')
ORDER BY (m.status='active') DESC, m.granted_at, m.id
LIMIT $3`, clientID, history, maxMembers)
	if err != nil {
		return nil, mapCustomerError(err)
	}
	defer rows.Close()
	out := []Membership{}
	for rows.Next() {
		var m Membership
		if err := rows.Scan(&m.ID, &m.ClientID, &m.UserID, &m.Status, &m.GrantedAt, &m.GrantedBy, &m.RevokedAt, &m.RevokedBy, &m.UserName); err != nil {
			return nil, mapCustomerError(err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Grant makes a user eligible for a client. Repeating an active grant returns the existing row unchanged.
// Two concurrent grants race on the partial unique index; the loser returns the winner's row.
func (s *Service) Grant(ctx context.Context, actor, clientID, userID string) (Membership, error) {
	if _, err := uuid.Parse(clientID); err != nil {
		return Membership{}, ErrNotFound
	}
	if _, err := uuid.Parse(userID); err != nil {
		return Membership{}, ErrUserNotFound
	}
	if _, err := uuid.Parse(actor); err != nil {
		return Membership{}, ErrUserNotFound
	}
	// Two attempts: the second one only runs after a lost race, and then finds the active row the winner committed.
	for attempt := 0; attempt < 2; attempt++ {
		m, retry, err := s.grantOnce(ctx, actor, clientID, userID)
		if retry {
			continue
		}
		return m, err
	}
	return Membership{}, errors.New("clients: grant did not settle after a lost race")
}

func (s *Service) grantOnce(ctx context.Context, actor, clientID, userID string) (Membership, bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Membership{}, false, err
	}
	defer tx.Rollback(ctx)

	// Lock order: client (shared, so archive waits for us), then user (shared, so a deactivation waits for us).
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM clients WHERE id=$1::uuid FOR SHARE`, clientID).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, false, ErrNotFound
	} else if err != nil {
		return Membership{}, false, mapCustomerError(err)
	}
	if status == "archived" {
		return Membership{}, false, ErrArchived
	}
	var userStatus, userType string
	if err := tx.QueryRow(ctx, `SELECT status::text, user_type::text FROM users WHERE id=$1::uuid FOR SHARE`, userID).Scan(&userStatus, &userType); errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, false, ErrUserNotFound
	} else if err != nil {
		return Membership{}, false, mapCustomerError(err)
	}
	if !eligible(userStatus, userType) {
		return Membership{}, false, ErrIneligibleUser
	}

	// Idempotent: an active grant is returned as it is, keeping its original actor and time.
	if existing, err := scanMembership(tx.QueryRow(ctx,
		`SELECT `+membershipColumns+` FROM client_memberships WHERE client_id=$1::uuid AND user_id=$2::uuid AND status='active'`,
		clientID, userID)); err == nil {
		return existing, false, tx.Commit(ctx)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, false, err
	}

	m, err := scanMembership(tx.QueryRow(ctx,
		`INSERT INTO client_memberships(client_id,user_id,granted_by) VALUES ($1::uuid,$2::uuid,$3::uuid) RETURNING `+membershipColumns,
		clientID, userID, actor))
	if pg := pgErrorOf(err); pg != nil && pg.Code == "23505" && pg.ConstraintName == "client_memberships_one_active_idx" {
		return Membership{}, true, nil // another grant committed first; retry reads its row
	}
	if err != nil {
		return Membership{}, false, mapCustomerError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Membership{}, false, mapCustomerError(err)
	}
	return m, false, nil
}

// Revoke ends an active membership and keeps the row. A repeated revoke returns the revoked row unchanged.
// Revoking is allowed for an archived customer and for a user who is no longer eligible.
// ErrNotMember means the pair has never had a membership.
func (s *Service) Revoke(ctx context.Context, actor, clientID, userID string) (Membership, error) {
	if _, err := uuid.Parse(clientID); err != nil {
		return Membership{}, ErrNotFound
	}
	if _, err := uuid.Parse(userID); err != nil {
		return Membership{}, ErrNotMember
	}
	if _, err := uuid.Parse(actor); err != nil {
		return Membership{}, ErrUserNotFound
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Membership{}, err
	}
	defer tx.Rollback(ctx)

	// Lock order: client (shared), then the membership row.
	var one int
	if err := tx.QueryRow(ctx, `SELECT 1 FROM clients WHERE id=$1::uuid FOR SHARE`, clientID).Scan(&one); errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, ErrNotFound
	} else if err != nil {
		return Membership{}, mapCustomerError(err)
	}
	active, err := scanMembership(tx.QueryRow(ctx,
		`SELECT `+membershipColumns+` FROM client_memberships WHERE client_id=$1::uuid AND user_id=$2::uuid AND status='active' FOR UPDATE`,
		clientID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		// No active row: return the latest revoked one unchanged, or report that there was never a membership.
		last, err := scanMembership(tx.QueryRow(ctx,
			`SELECT `+membershipColumns+` FROM client_memberships WHERE client_id=$1::uuid AND user_id=$2::uuid ORDER BY granted_at DESC, id DESC LIMIT 1`,
			clientID, userID))
		if errors.Is(err, pgx.ErrNoRows) {
			return Membership{}, ErrNotMember
		}
		if err != nil {
			return Membership{}, mapCustomerError(err)
		}
		return last, tx.Commit(ctx)
	}
	if err != nil {
		return Membership{}, mapCustomerError(err)
	}
	revoked, err := scanMembership(tx.QueryRow(ctx,
		`UPDATE client_memberships SET status='revoked', revoked_at=now(), revoked_by=$2::uuid WHERE id=$1::uuid RETURNING `+membershipColumns,
		active.ID, actor))
	if err != nil {
		return Membership{}, mapCustomerError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Membership{}, mapCustomerError(err)
	}
	return revoked, nil
}
