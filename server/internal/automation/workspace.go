package automation

import (
	"context"
	"encoding/json"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Workspace automation (AUT-02, design approved: D1 to D4). A workspace rule has no Space of its own; it lists the Spaces it may run in.
// Only a workspace admin creates, changes, or lists them. A run still needs the rule's owner to be allowed on the item and its Space,
// so the admin's role never opens a Space's content.

// enableWorkspace checks what must hold before a workspace rule may run: at least one target, an active owner, and references that
// exist in every target (D2 and D1). Owner access per Space is checked at each run, so one Space refusing does not stop the others.
func enableWorkspace(ctx context.Context, tx pgx.Tx, rule Rule, def Definition) error {
	if len(rule.TargetSpaceIDs) == 0 {
		return items.InvalidField{Field: "target_space_ids", Message: "a workspace rule needs at least one target Space to be enabled"}
	}
	var active bool
	if err := tx.QueryRow(ctx, "SELECT status='active' FROM users WHERE id=$1::uuid", rule.OwnerID).Scan(&active); err != nil {
		return err
	}
	if !active {
		return items.InvalidField{Field: "enabled", Message: "the rule cannot run until this is fixed: " + ReasonOwnerInactive}
	}
	single := len(rule.TargetSpaceIDs) == 1
	for _, target := range rule.TargetSpaceIDs {
		code, err := dependencyProblem(ctx, tx, "workspace", target, nil, single, def)
		if err != nil {
			return err
		}
		if code != "" {
			return items.InvalidField{Field: "enabled", Message: "the rule cannot run in a target Space until this is fixed: " + code}
		}
	}
	return nil
}

// CreateWorkspace makes a workspace rule with a first draft. Only a workspace admin may create one; targets come later.
func CreateWorkspace(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, name string, raw json.RawMessage) (Rule, error) {
	if actor.Type != policy.Admin {
		return Rule{}, items.ErrForbidden
	}
	name, err := cleanName(name)
	if err != nil {
		return Rule{}, err
	}
	def, err := ParseDefinition(raw)
	if err != nil {
		return Rule{}, items.InvalidField{Field: "definition", Message: err.Error()}
	}
	if err := validateForScope(def, "workspace"); err != nil {
		return Rule{}, items.InvalidField{Field: "definition", Message: err.Error()}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Rule{}, err
	}
	defer tx.Rollback(ctx)
	id, vid := uuid.New(), uuid.New()
	r, err := scanRule(tx.QueryRow(ctx, `INSERT INTO automation_rules(id,space_id,container_id,name,owner_id,scope) VALUES ($1,NULL,NULL,$2,$3::uuid,'workspace') RETURNING `+ruleColumns,
		id, name, actor.ID))
	if err != nil {
		return Rule{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO automation_rule_versions(id,rule_id,version,definition,status,created_by) VALUES ($1,$2,1,$3,'draft',$4::uuid)`, vid, id, raw, actor.ID); err != nil {
		return Rule{}, err
	}
	if err := audit(ctx, tx, actor, "automation_rule.created", id.String(), map[string]any{"scope": "workspace"}); err != nil {
		return Rule{}, err
	}
	return r, tx.Commit(ctx)
}

// SetTargets replaces the Spaces a workspace rule may run in. The list is a snapshot: a Space created later is not added (D2).
// An enabled rule must keep at least one target. Every change is written to the trail with the old and new lists.
func SetTargets(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ruleID string, spaceIDs []string) (Rule, error) {
	if actor.Type != policy.Admin {
		return Rule{}, items.ErrForbidden
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Rule{}, err
	}
	defer tx.Rollback(ctx)
	rule, err := manage(ctx, tx, actor, ruleID, true)
	if err != nil {
		return Rule{}, err
	}
	if rule.Scope != "workspace" {
		return Rule{}, items.Invalid("targets belong to workspace rules; a Space rule runs in its own Space")
	}
	seen := map[string]bool{}
	var next []uuid.UUID
	for _, s := range spaceIDs {
		id, perr := uuid.Parse(s)
		if perr != nil {
			return Rule{}, items.InvalidField{Field: "space_ids", Message: "each target must be a Space id"}
		}
		if !seen[id.String()] {
			seen[id.String()] = true
			next = append(next, id)
		}
	}
	if rule.Enabled && len(next) == 0 {
		return Rule{}, items.InvalidField{Field: "space_ids", Message: "an enabled workspace rule keeps at least one target Space"}
	}
	if len(next) > 0 {
		var found int
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM spaces WHERE id=ANY($1::uuid[])", next).Scan(&found); err != nil {
			return Rule{}, err
		}
		if found != len(next) {
			return Rule{}, items.InvalidField{Field: "space_ids", Message: "a target Space does not exist"}
		}
	}
	nextStrings := make([]string, 0, len(next))
	for _, id := range next {
		nextStrings = append(nextStrings, id.String())
	}
	out, err := scanRule(tx.QueryRow(ctx, "UPDATE automation_rules SET target_space_ids=$2::uuid[], updated_at=now() WHERE id=$1::uuid RETURNING "+ruleColumns, rule.ID, next))
	if err != nil {
		return Rule{}, err
	}
	if err := audit(ctx, tx, actor, "automation_rule.targets_updated", rule.ID, map[string]any{"before": rule.TargetSpaceIDs, "after": nextStrings}); err != nil {
		return Rule{}, err
	}
	return out, tx.Commit(ctx)
}

// ListWorkspace lists the workspace rules for a workspace admin: metadata only (decision D4). Target ids are listed; Space names and
// item content are not, so a confidential Space is not revealed by the registry.
func ListWorkspace(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor) ([]Rule, error) {
	if actor.Type != policy.Admin {
		return nil, items.ErrForbidden
	}
	return listRegistry(ctx, pool, "r.scope='workspace'")
}

// SpaceWorkspaceRule is what a Space editor sees of a workspace rule that runs in their Space: its name and whether it is on. The owner,
// the definition, and the outcomes stay with the workspace admin (decision D4).
type SpaceWorkspaceRule struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Health  string `json:"health"`
}

// WorkspaceRulesForSpace lists the enabled workspace rules that target a Space, for people who can edit that Space.
func WorkspaceRulesForSpace(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID string) ([]SpaceWorkspaceRule, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	decision, _, err := policy.CanOnSpace(ctx, tx, actor, policy.Edit, spaceID)
	if err != nil {
		return nil, err
	}
	if decision == policy.NotFound {
		return nil, items.ErrNotFound
	}
	if decision != policy.Allowed {
		return nil, items.ErrForbidden
	}
	rows, err := tx.Query(ctx, `SELECT id::text, name, enabled, health FROM automation_rules
WHERE scope='workspace' AND archived_at IS NULL AND $1::uuid = ANY(target_space_ids) ORDER BY name, id`, spaceID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[SpaceWorkspaceRule])
}
