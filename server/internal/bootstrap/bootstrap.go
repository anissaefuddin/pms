package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InviteValidity is how long the first administrator's invitation link works.
const InviteValidity = 7 * 24 * time.Hour

type Result struct {
	AdminID     string
	InviteToken string
}

type EnqueueInvite func(ctx context.Context, tx pgx.Tx, to, token string) error

// Run creates the single account row, an administrator in state `invited`, the General space and the default workflow.
// The Personal space is created when the administrator accepts the invitation. A second run fails without changing data,
// except for one recovery case: same email while that administrator is still `invited` re-issues the invitation.
// The account row is a singleton (id = 1), so concurrent first runs cannot both succeed.
func Run(ctx context.Context, pool *pgxpool.Pool, accountName, adminEmail string, enqueue EnqueueInvite) (Result, error) {
	accountName, adminEmail = strings.TrimSpace(accountName), strings.ToLower(strings.TrimSpace(adminEmail))
	address, err := mail.ParseAddress(adminEmail)
	if accountName == "" || err != nil || strings.ToLower(address.Address) != adminEmail {
		return Result{}, fmt.Errorf("valid --account and --admin-email are required")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "INSERT INTO account(id,name) VALUES (1,$1) ON CONFLICT (id) DO NOTHING", accountName)
	if err != nil {
		return Result{}, err
	}
	if _, err := tx.Exec(ctx, "SELECT 1 FROM account WHERE id=1 FOR UPDATE"); err != nil {
		return Result{}, err
	}
	var adminID string
	if tag.RowsAffected() == 0 {
		// Already bootstrapped. The only allowed repeat is recovery: the same administrator never accepted the
		// invitation (for example it expired), so issue a fresh link instead of leaving the instance locked.
		// FOR UPDATE serializes concurrent re-issues so exactly one live token remains.
		err := tx.QueryRow(ctx, "SELECT id::text FROM users WHERE email=$1 AND user_type='admin' AND status='invited' FOR UPDATE", adminEmail).Scan(&adminID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, errors.New("instance is already bootstrapped")
		}
		if err != nil {
			return Result{}, err
		}
		if _, err := tx.Exec(ctx, "DELETE FROM user_tokens WHERE user_id=$1 AND purpose='invite'", adminID); err != nil {
			return Result{}, err
		}
	} else {
		var workflowID string
		if err := tx.QueryRow(ctx, "INSERT INTO users(email,user_type,status) VALUES ($1,'admin','invited') RETURNING id::text", adminEmail).Scan(&adminID); err != nil {
			return Result{}, err
		}
		if _, err := tx.Exec(ctx, "UPDATE account SET owner_id=$1 WHERE id=1", adminID); err != nil {
			return Result{}, err
		}
		// The first administrator owns General explicitly (D-17.1, D-SA1): without a share and an admin nobody could ever open it. General is internal by explicit choice (B3-SC): a fresh instance has no client to own it.
		if _, err := tx.Exec(ctx, `WITH general AS (INSERT INTO spaces(name,kind,scope_state) VALUES ('General','shared','internal') RETURNING id),
owner AS (INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,created_by) SELECT 'space',id,'user',$1::uuid,'full',$1::uuid FROM general)
INSERT INTO space_admins(space_id,user_id,granted_by) SELECT id,$1::uuid,$1::uuid FROM general`, adminID); err != nil {
			return Result{}, err
		}
		if err := tx.QueryRow(ctx, "INSERT INTO workflows(name,is_default) VALUES ('Default workflow',true) RETURNING id::text").Scan(&workflowID); err != nil {
			return Result{}, err
		}
		statuses := []struct{ name, group, color string }{{"New", "active", "slate"}, {"In Progress", "active", "blue"}, {"Completed", "completed", "green"}, {"On Hold", "deferred", "amber"}, {"Cancelled", "cancelled", "red"}}
		for i, status := range statuses {
			if _, err := tx.Exec(ctx, "INSERT INTO statuses(workflow_id,name,grp,color,position) VALUES ($1,$2,$3::status_group,$4,$5)", workflowID, status.name, status.group, status.color, i); err != nil {
				return Result{}, err
			}
		}
	}
	plain, hash, err := security.Token()
	if err != nil {
		return Result{}, err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) VALUES ($1,$2,'invite',$3)", hash, adminID, time.Now().Add(InviteValidity)); err != nil {
		return Result{}, err
	}
	if enqueue != nil {
		if err := enqueue(ctx, tx, adminEmail, plain); err != nil {
			return Result{}, fmt.Errorf("queue invitation email: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{AdminID: adminID, InviteToken: plain}, nil
}
