package automation

import (
	"context"
	"errors"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Usage is the rolling hour of executions held against each cap (decision: same counter as enforcement). A cap is exhausted at 100%,
// warns at 80%, and is ok below that. A cap that does not apply to a rule is nil.
type Usage struct {
	Window    string   `json:"window"`
	Rule      *Counter `json:"rule"`
	Workspace *Counter `json:"workspace"`
	Space     *Counter `json:"space"`
}

// Counter is one cap: how many actions ran in the window, the limit, and its state.
type Counter struct {
	Used  int    `json:"used"`
	Limit int    `json:"limit"`
	State string `json:"state"`
}

func newCounter(used, limit int) *Counter {
	state := "ok"
	switch {
	case used >= limit:
		state = "exhausted"
	case used*100 >= limit*80:
		state = "warning"
	}
	return &Counter{Used: used, Limit: limit, State: state}
}

// ThrottledBy names the first exhausted cap, or "" when a run may go ahead. Order: rule, workspace, Space.
func (u Usage) ThrottledBy() string {
	switch {
	case u.Rule != nil && u.Rule.State == "exhausted":
		return "rule_quota"
	case u.Workspace != nil && u.Workspace.State == "exhausted":
		return "workspace_quota"
	case u.Space != nil && u.Space.State == "exhausted":
		return "space_quota"
	}
	return ""
}

// usageOf counts completed actions in the last hour for a rule of the given scope, in the given Space. A Space rule's Space and a
// workspace rule's execution Space are both checked; the workspace total counts every workspace rule.
func usageOf(ctx context.Context, pool *pgxpool.Pool, ruleID uuid.UUID, scope string, spaceID uuid.UUID) (Usage, error) {
	limits, err := GetLimitsInternal(ctx, pool)
	if err != nil {
		return Usage{}, err
	}
	ruleCap := SpaceRuleQuotaPerHour
	if scope == "workspace" {
		ruleCap = limits.RulePerHour
	}
	var rule, workspace, space int
	err = pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE a.rule_id=$1), count(*) FILTER (WHERE r.scope='workspace'), count(*) FILTER (WHERE a.space_id=$2)
FROM automation_runs a JOIN automation_rules r ON r.id=a.rule_id
WHERE a.action_index>=0 AND a.status='done' AND a.created_at > now() - interval '1 hour'
 AND (a.rule_id=$1 OR (r.scope='workspace' AND $3) OR a.space_id=$2)`, ruleID, spaceID, scope == "workspace").Scan(&rule, &workspace, &space)
	if err != nil {
		return Usage{}, err
	}
	u := Usage{Window: "1h", Space: newCounter(space, limits.SpacePerHour)}
	if ruleID != uuid.Nil {
		u.Rule = newCounter(rule, ruleCap)
	}
	if scope == "workspace" {
		u.Workspace = newCounter(workspace, limits.WorkspacePerHour)
	}
	return u, nil
}

// UsageFor is what a person may see of the usage: a rule's counters for those who can manage the rule, the workspace counter for
// workspace admins, and a Space's counter for those who can edit the Space. Nothing else is shown (decision D4).
func UsageFor(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ruleID, spaceID string) (Usage, error) {
	if ruleID != "" {
		id, err := uuid.Parse(ruleID)
		if err != nil {
			return Usage{}, items.ErrNotFound
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return Usage{}, err
		}
		defer tx.Rollback(ctx)
		rule, err := manage(ctx, tx, actor, ruleID, false)
		if err != nil {
			return Usage{}, err
		}
		space := uuid.Nil
		if rule.SpaceID != "" {
			space = uuid.MustParse(rule.SpaceID)
		} else if len(rule.TargetSpaceIDs) > 0 {
			space = uuid.MustParse(rule.TargetSpaceIDs[0])
		}
		u, err := usageOf(ctx, pool, id, rule.Scope, space)
		if err != nil {
			return Usage{}, err
		}
		if rule.Scope != "workspace" {
			u.Workspace = nil
		} else if actor.Type != policy.Admin {
			u.Workspace = nil
		}
		if rule.Scope == "workspace" {
			u.Space = nil // a workspace rule has no one Space; each execution Space is seen through its runs
		}
		return u, nil
	}
	if spaceID != "" {
		sid, err := uuid.Parse(spaceID)
		if err != nil {
			return Usage{}, items.ErrNotFound
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return Usage{}, err
		}
		defer tx.Rollback(ctx)
		decision, _, err := policy.CanOnSpace(ctx, tx, actor, policy.Edit, spaceID)
		if err != nil {
			return Usage{}, err
		}
		if decision != policy.Allowed {
			return Usage{}, items.ErrNotFound
		}
		u, err := usageOf(ctx, pool, uuid.Nil, "space", sid)
		if err != nil {
			return Usage{}, err
		}
		u.Rule, u.Workspace = nil, nil
		return u, nil
	}
	if actor.Type != policy.Admin {
		return Usage{}, items.ErrForbidden
	}
	u, err := usageOf(ctx, pool, uuid.Nil, "workspace", uuid.Nil)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Usage{}, err
	}
	u.Rule, u.Space = nil, nil
	return u, nil
}
