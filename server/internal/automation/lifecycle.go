package automation

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/reqid"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Health states (A1). A rule that is not ok does nothing until someone resumes it, and the reason code says why.
// Paused: its owner can no longer run it here. Invalid: something its published version points at is gone.
const (
	healthOK      = "ok"
	healthPaused  = "paused"
	healthInvalid = "invalid"
)

// errStopped ends a rule's actions after one of them is refused by policy. The refusal is already recorded; the rest do not run.
var errStopped = errors.New("a refused action stopped the rule")

// revalidate is what a rule needs before it may run again: a usable owner and everything its published version points at.
// It returns the reason code of the first problem, or "".
func revalidate(ctx context.Context, tx pgx.Tx, scope, ownerID, spaceID string, container *string, d Definition) (string, error) {
	if code, err := ownerProblem(ctx, tx, ownerID, spaceID); code != "" || err != nil {
		return code, err
	}
	return dependencyProblem(ctx, tx, scope, spaceID, container, true, d)
}

// healthFor maps a reason code to the state it puts the rule in: owner problems pause it, anything else makes it invalid.
func healthFor(code string) string {
	if code == ReasonOwnerInactive || code == ReasonOwnerNoAccess {
		return healthPaused
	}
	return healthInvalid
}

// setHealth moves a rule out of ok, once, and records why in the trail. Only resuming (PATCH enabled=true) moves it back.
// The engine calls it with a zero actor, so the trail names the Automation Bot acting for the rule's owner.
func setHealth(ctx context.Context, pool *pgxpool.Pool, ruleID, ownerID, health, reason string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "UPDATE automation_rules SET health=$2, health_reason=$3, updated_at=now() WHERE id=$1::uuid AND health='ok'", ruleID, health, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		action := "automation_rule.paused"
		if health == healthInvalid {
			action = "automation_rule.invalidated"
		}
		if err := audit(ctx, tx, policy.Actor{}, action, ruleID, map[string]any{"reason": reason, "owner_id": ownerID}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// audit writes one line of the trail inside the caller's transaction, so the change and its record commit or roll back together.
// A zero actor is the Automation Bot; a person's line carries their email as a snapshot.
func audit(ctx context.Context, tx pgx.Tx, actor policy.Actor, action, ruleID string, details map[string]any) error {
	var id *string
	if actor.ID != "" {
		id = &actor.ID
	}
	if details == nil {
		details = map[string]any{}
	}
	raw, _ := json.Marshal(details)
	_, err := tx.Exec(ctx, `INSERT INTO audit_events(actor_id,actor_name,action,target_type,target_id,details,request_id)
VALUES ($1::uuid, COALESCE((SELECT email::text FROM users WHERE id=$1::uuid), 'Automation Bot'), $2, 'automation_rule', $3, $4, $5)`, id, action, ruleID, raw, reqid.From(ctx))
	return err
}

// publishedDefinition reads the definition of the version a rule runs.
func publishedDefinition(ctx context.Context, tx pgx.Tx, ruleID string) (Definition, error) {
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT v.definition FROM automation_rule_versions v JOIN automation_rules r ON r.current_version_id=v.id WHERE r.id=$1::uuid`, ruleID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Definition{}, items.Invalid("publish a version first")
	}
	if err != nil {
		return Definition{}, err
	}
	return ParseDefinition(raw)
}

// Archive takes a rule out of use and keeps its versions and history. Deleting a rule is archiving it (A1 delete policy).
func Archive(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ruleID string) (Rule, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Rule{}, err
	}
	defer tx.Rollback(ctx)
	rule, err := manage(ctx, tx, actor, ruleID, true)
	if err != nil {
		return Rule{}, err
	}
	if rule.ArchivedAt != nil {
		return rule, nil
	}
	out, err := scanRule(tx.QueryRow(ctx, "UPDATE automation_rules SET archived_at=now(), enabled=false, updated_at=now() WHERE id=$1::uuid RETURNING "+ruleColumns, rule.ID))
	if err != nil {
		return Rule{}, err
	}
	if err := audit(ctx, tx, actor, "automation_rule.archived", rule.ID, nil); err != nil {
		return Rule{}, err
	}
	return out, tx.Commit(ctx)
}

// Restore brings an archived rule back as a disabled rule. Enabling it again revalidates it first.
func Restore(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ruleID string) (Rule, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Rule{}, err
	}
	defer tx.Rollback(ctx)
	rule, err := manage(ctx, tx, actor, ruleID, true)
	if err != nil {
		return Rule{}, err
	}
	if rule.ArchivedAt == nil {
		return rule, nil
	}
	out, err := scanRule(tx.QueryRow(ctx, "UPDATE automation_rules SET archived_at=NULL, updated_at=now() WHERE id=$1::uuid RETURNING "+ruleColumns, rule.ID))
	if err != nil {
		return Rule{}, err
	}
	if err := audit(ctx, tx, actor, "automation_rule.restored", rule.ID, nil); err != nil {
		return Rule{}, err
	}
	return out, tx.Commit(ctx)
}

// TransferOwner gives a rule to another user. Only a workspace admin may do it, and the new owner must be able to run it here.
// The rule is revalidated under the new owner: it becomes ok when nothing is missing, otherwise it is invalid with the reason.
func TransferOwner(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ruleID, newOwner string) (Rule, error) {
	if actor.Type != policy.Admin {
		return Rule{}, items.ErrForbidden
	}
	ownerID, err := uuid.Parse(newOwner)
	if err != nil {
		return Rule{}, items.InvalidField{Field: "owner_id", Message: "owner_id must be a user id"}
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
	// A workspace rule has no Space of its own: its new owner must be active, and each run still checks access to its Space.
	if rule.Scope == "workspace" {
		var active bool
		if err := tx.QueryRow(ctx, "SELECT status='active' FROM users WHERE id=$1::uuid", ownerID).Scan(&active); err != nil || !active {
			return Rule{}, errors.Join(err, items.InvalidField{Field: "owner_id", Message: "the new owner must be an active user"})
		}
	} else if code, err := ownerProblem(ctx, tx, ownerID.String(), rule.SpaceID); code != "" || err != nil {
		return Rule{}, errors.Join(err, items.InvalidField{Field: "owner_id", Message: "the new owner cannot run rules in this Space: " + code})
	}
	health, reason := healthOK, ""
	if rule.CurrentVersionID != nil && rule.Scope != "workspace" { // a workspace rule's references are checked per target at enable
		def, err := publishedDefinition(ctx, tx, rule.ID)
		if err != nil {
			return Rule{}, err
		}
		code, err := dependencyProblem(ctx, tx, rule.Scope, rule.SpaceID, rule.ContainerID, true, def)
		if err != nil {
			return Rule{}, err
		}
		if code != "" {
			health, reason = healthInvalid, code
		}
	}
	out, err := scanRule(tx.QueryRow(ctx, "UPDATE automation_rules SET owner_id=$2::uuid, health=$3, health_reason=$4, updated_at=now() WHERE id=$1::uuid RETURNING "+ruleColumns,
		rule.ID, ownerID, health, reason))
	if err != nil {
		return Rule{}, err
	}
	if err := audit(ctx, tx, actor, "automation_rule.owner_transferred", rule.ID, map[string]any{"from": rule.OwnerID, "to": ownerID.String()}); err != nil {
		return Rule{}, err
	}
	return out, tx.Commit(ctx)
}
