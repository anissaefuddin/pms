package automation

import (
	"context"
	"encoding/json"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// A rule runs with its owner's rights, so changing what another person's rule assigns, or handing a rule that assigns to a new
// owner, would borrow that person's rights. Doing it needs the right to assign in every Space the rule acts in (DEF-028).
func requireAssignAuthority(ctx context.Context, tx pgx.Tx, actor policy.Actor, rule Rule, definitions ...json.RawMessage) error {
	assigns := false
	for _, raw := range definitions {
		var def Definition
		if err := json.Unmarshal(raw, &def); err != nil {
			return err
		}
		for _, action := range def.Then {
			assigns = assigns || action.Type == "assign"
		}
	}
	if !assigns {
		return nil
	}
	spaces := rule.TargetSpaceIDs
	if rule.Scope != "workspace" {
		spaces = []string{rule.SpaceID}
	}
	var locations []items.Location
	for _, id := range spaces {
		parsed, err := uuid.Parse(id)
		if err != nil {
			return items.ErrAssignForbidden
		}
		locations = append(locations, items.Location{SpaceID: parsed})
	}
	ok, err := items.CanAssignOthers(ctx, tx, actor, locations)
	if err == nil && !ok {
		err = items.ErrAssignForbidden
	}
	return err
}

// ruleDefinitions are the versions of a rule that run or are about to: the published one and the draft.
func ruleDefinitions(ctx context.Context, tx pgx.Tx, ruleID string) ([]json.RawMessage, error) {
	rows, err := tx.Query(ctx, "SELECT definition FROM automation_rule_versions WHERE rule_id=$1::uuid AND status IN ('draft','published')", ruleID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[json.RawMessage])
}
