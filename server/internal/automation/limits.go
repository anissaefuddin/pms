package automation

import (
	"context"
	"encoding/json"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/reqid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Limits are the execution caps (decision D3). Each one counts completed actions in the last hour. Workspace limits are configurable
// by a workspace admin and every change is written to the trail.
type Limits struct {
	RulePerHour      int `json:"rule_per_hour"`
	WorkspacePerHour int `json:"workspace_per_hour"`
	SpacePerHour     int `json:"space_per_hour"`
}

// SpaceRuleQuotaPerHour is the per-rule cap for Space rules, unchanged from before workspace rules existed.
const SpaceRuleQuotaPerHour = RuleQuotaPerHour

// GetLimits returns the current caps to a workspace admin.
func GetLimits(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor) (Limits, error) {
	if actor.Type != policy.Admin {
		return Limits{}, items.ErrForbidden
	}
	var l Limits
	err := pool.QueryRow(ctx, "SELECT rule_per_hour, workspace_per_hour, space_per_hour FROM automation_limits WHERE id=1").Scan(&l.RulePerHour, &l.WorkspacePerHour, &l.SpacePerHour)
	return l, err
}

// SetLimits replaces the caps. The old and new values go to the trail in the same transaction.
func SetLimits(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, next Limits) (Limits, error) {
	if actor.Type != policy.Admin {
		return Limits{}, items.ErrForbidden
	}
	for _, v := range []int{next.RulePerHour, next.WorkspacePerHour, next.SpacePerHour} {
		if v < 1 || v > 100000 {
			return Limits{}, items.InvalidField{Field: "limits", Message: "each limit must be between 1 and 100000 an hour"}
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Limits{}, err
	}
	defer tx.Rollback(ctx)
	var before Limits
	if err := tx.QueryRow(ctx, "SELECT rule_per_hour, workspace_per_hour, space_per_hour FROM automation_limits WHERE id=1 FOR UPDATE").Scan(&before.RulePerHour, &before.WorkspacePerHour, &before.SpacePerHour); err != nil {
		return Limits{}, err
	}
	if _, err := tx.Exec(ctx, "UPDATE automation_limits SET rule_per_hour=$1, workspace_per_hour=$2, space_per_hour=$3, updated_by=$4::uuid, updated_at=now() WHERE id=1",
		next.RulePerHour, next.WorkspacePerHour, next.SpacePerHour, actor.ID); err != nil {
		return Limits{}, err
	}
	raw, _ := json.Marshal(map[string]any{"before": before, "after": next})
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(actor_id,actor_name,action,target_type,target_id,details,request_id)
VALUES ($1::uuid, COALESCE((SELECT email::text FROM users WHERE id=$1::uuid),''), 'automation_limits.updated', 'automation_limits', '1', $2, $3)`, actor.ID, raw, reqid.From(ctx)); err != nil {
		return Limits{}, err
	}
	return next, tx.Commit(ctx)
}

// GetLimitsInternal reads the caps for the engine, which has no actor: it is the system reading its own configuration.
func GetLimitsInternal(ctx context.Context, pool *pgxpool.Pool) (Limits, error) {
	var l Limits
	err := pool.QueryRow(ctx, "SELECT rule_per_hour, workspace_per_hour, space_per_hour FROM automation_limits WHERE id=1").Scan(&l.RulePerHour, &l.WorkspacePerHour, &l.SpacePerHour)
	return l, err
}
