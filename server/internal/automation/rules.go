package automation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Version struct {
	ID          string          `json:"id"`
	Version     int             `json:"version"`
	Status      string          `json:"status"`
	Definition  json.RawMessage `json:"definition"`
	CreatedAt   string          `json:"created_at"`
	PublishedAt *string         `json:"published_at"`
}

type Rule struct {
	ID               string  `json:"id"`
	SpaceID          string  `json:"space_id"`
	ContainerID      *string `json:"container_id"`
	Name             string  `json:"name"`
	OwnerID          string  `json:"owner_id"`
	Enabled          bool    `json:"enabled"`
	CurrentVersionID *string `json:"current_version_id"`
	// Health is ok, paused (the owner cannot run it) or invalid (something it depends on is gone); HealthReason is the reason code.
	Health       string  `json:"health"`
	HealthReason string  `json:"health_reason"`
	ArchivedAt   *string `json:"archived_at"`
	// Scope is space, or workspace (then SpaceID is empty and TargetSpaceIDs lists the Spaces it may run in).
	Scope          string   `json:"scope"`
	TargetSpaceIDs []string `json:"target_space_ids"`
	// The registry fields below are filled by List (A2): what the rule is, who owns it, and how it has been doing.
	OwnerName      string    `json:"owner_name"`
	ContainerTitle *string   `json:"container_title"`
	Trigger        string    `json:"trigger"`
	ActionCount    int       `json:"action_count"`
	HasDraft       bool      `json:"has_draft"`
	LastRunAt      *string   `json:"last_run_at"`
	LastRunStatus  *string   `json:"last_run_status"`
	ErrorCount     int       `json:"error_count"`
	Versions       []Version `json:"versions,omitempty"`
}

const ruleColumns = "id::text,COALESCE(space_id::text,''),container_id::text,name,owner_id::text,enabled,current_version_id::text,health,health_reason,archived_at::text,scope,target_space_ids::text[]"

func scanRule(row pgx.Row) (Rule, error) {
	var r Rule
	err := row.Scan(&r.ID, &r.SpaceID, &r.ContainerID, &r.Name, &r.OwnerID, &r.Enabled, &r.CurrentVersionID, &r.Health, &r.HealthReason, &r.ArchivedAt, &r.Scope, &r.TargetSpaceIDs)
	return r, err
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 120 || strings.ContainsRune(name, 0) {
		return "", items.InvalidField{Field: "name", Message: "name must be 1 to 120 characters"}
	}
	return name, nil
}

// Reason codes for a rule that cannot run. They are stored on the rule and shown to people, so keep them stable.
const (
	ReasonOwnerInactive = "owner_inactive"
	ReasonOwnerNoAccess = "owner_no_access"
	ReasonStatusMissing = "status_missing"
	ReasonTypeMissing   = "item_type_missing"
	ReasonUserInactive  = "user_inactive"
	ReasonFieldMissing  = "field_missing"
	ReasonContainerGone = "container_missing"
)

// dependencyProblem returns the reason code of the first thing a definition points at that no longer exists in this Space, or "".
// It runs when a rule is saved, published, resumed, and before each run, so a deleted status or type is noticed without a hook.
// Whether the owner may use them is decided separately (ownerProblem, and the actor checks in each action).
func dependencyProblem(ctx context.Context, tx pgx.Tx, scope, spaceID string, container *string, singleTarget bool, d Definition) (string, error) {
	exists := func(sql string, args ...any) (bool, error) {
		var ok bool
		err := tx.QueryRow(ctx, sql, args...).Scan(&ok)
		return ok, err
	}
	check := func(code string, ok bool, err error) (string, error) {
		if err != nil {
			return "", err
		}
		if !ok {
			return code, nil
		}
		return "", nil
	}
	if container != nil {
		ok, err := exists("SELECT EXISTS(SELECT 1 FROM items i JOIN item_locations l ON l.item_id=i.id WHERE i.id=$1::uuid AND l.space_id=$2::uuid AND i.deleted_at IS NULL AND l.deleted_root_id IS NULL)", *container, spaceID)
		if code, err := check(ReasonContainerGone, ok, err); code != "" || err != nil {
			return code, err
		}
	}
	if d.When.ToStatusID != "" {
		ok, err := exists("SELECT EXISTS(SELECT 1 FROM statuses WHERE id=$1::uuid)", d.When.ToStatusID)
		if code, err := check(ReasonStatusMissing, ok, err); code != "" || err != nil {
			return code, err
		}
	}
	users := append([]string{}, d.When.UserID)
	fieldIDs := []string{}
	for _, c := range d.If {
		if strings.HasPrefix(c.Field, "fields.") {
			fieldIDs = append(fieldIDs, strings.TrimPrefix(c.Field, "fields."))
		}
	}
	for _, a := range d.Then {
		users = append(users, a.UserIDs...)
		if a.TypeKey != "" {
			ok, err := exists("SELECT EXISTS(SELECT 1 FROM item_type_defs WHERE key=$1 AND active)", a.TypeKey)
			if code, err := check(ReasonTypeMissing, ok, err); code != "" || err != nil {
				return code, err
			}
		}
		if a.Type == "set_field" && a.Field == "status_id" {
			var id string
			if json.Unmarshal(a.Value, &id) != nil || !isUUID(id) {
				return "", items.InvalidField{Field: "definition", Message: "set_field status_id needs a status id"}
			}
			ok, err := exists("SELECT EXISTS(SELECT 1 FROM statuses WHERE id=$1::uuid)", id)
			if code, err := check(ReasonStatusMissing, ok, err); code != "" || err != nil {
				return code, err
			}
		}
		if a.Type == "set_field" && strings.HasPrefix(a.Field, "fields.") {
			fieldIDs = append(fieldIDs, strings.TrimPrefix(a.Field, "fields."))
		}
	}
	for _, id := range fieldIDs {
		// A Space rule reads its Space's fields. A workspace rule reads Workspace fields (space_id NULL), which exist in every Space;
		// a Space field is usable only when the rule has exactly one target, since field ids differ between Spaces (decision D1).
		var ok bool
		var err error
		if scope == "workspace" {
			ok, err = exists("SELECT EXISTS(SELECT 1 FROM custom_fields WHERE id=$1::uuid AND archived_at IS NULL AND (space_id IS NULL OR (space_id=$2::uuid AND $3)))", id, spaceID, singleTarget)
		} else {
			ok, err = exists("SELECT EXISTS(SELECT 1 FROM custom_fields WHERE id=$1::uuid AND space_id=$2::uuid AND archived_at IS NULL)", id, spaceID)
		}
		if code, err := check(ReasonFieldMissing, ok, err); code != "" || err != nil {
			return code, err
		}
	}
	for _, id := range users {
		if id == "" {
			continue
		}
		ok, err := exists("SELECT EXISTS(SELECT 1 FROM users WHERE id=$1::uuid AND status='active')", id)
		if code, err := check(ReasonUserInactive, ok, err); code != "" || err != nil {
			return code, err
		}
	}
	return "", nil
}

// ownerProblem says whether the rule's owner may still run it in this Space: an active user with Edit on the Space.
func ownerProblem(ctx context.Context, tx pgx.Tx, ownerID, spaceID string) (string, error) {
	var active bool
	if err := tx.QueryRow(ctx, "SELECT status='active' FROM users WHERE id=$1::uuid", ownerID).Scan(&active); err != nil || !active {
		return ReasonOwnerInactive, err
	}
	decision, _, err := policy.CanOnSpace(ctx, tx, policy.Actor{ID: ownerID, Type: ownerUserType(ctx, tx, ownerID)}, policy.Edit, spaceID)
	if err != nil {
		return "", err
	}
	if decision != policy.Allowed {
		return ReasonOwnerNoAccess, nil
	}
	return "", nil
}

func ownerUserType(ctx context.Context, tx pgx.Tx, id string) policy.UserType {
	var t string
	_ = tx.QueryRow(ctx, "SELECT user_type::text FROM users WHERE id=$1::uuid", id).Scan(&t)
	return policy.UserType(t)
}

// checkReferences refuses a definition that points at something missing, with a message for the person who saved it.
func checkReferences(ctx context.Context, tx pgx.Tx, scope, spaceID string, container *string, singleTarget bool, d Definition) error {
	code, err := dependencyProblem(ctx, tx, scope, spaceID, container, singleTarget, d)
	if err != nil || code == "" {
		return err
	}
	return items.InvalidField{Field: "definition", Message: "a reference is missing or not allowed: " + code}
}

func errIf(cond bool, err error) error {
	if cond {
		return err
	}
	return nil
}

// manage loads a rule and checks the actor may change it: its owner or an administrator who can still see the Space.
func manage(ctx context.Context, tx pgx.Tx, actor policy.Actor, id string, lock bool) (Rule, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return Rule{}, items.ErrNotFound
	}
	q := "SELECT " + ruleColumns + " FROM automation_rules WHERE id=$1"
	if lock {
		q += " FOR UPDATE"
	}
	r, err := scanRule(tx.QueryRow(ctx, q, parsed))
	if errors.Is(err, pgx.ErrNoRows) {
		return Rule{}, items.ErrNotFound
	}
	if err != nil {
		return Rule{}, err
	}
	// Workspace rules: a workspace admin, or the rule's owner (the matrix in the design), may manage it; anyone else gets not found.
	if r.Scope == "workspace" {
		if actor.Type != policy.Admin && r.OwnerID != actor.ID {
			return Rule{}, items.ErrNotFound
		}
		return r, nil
	}
	decision, _, err := policy.CanOnSpace(ctx, tx, actor, policy.Edit, r.SpaceID)
	if err != nil {
		return Rule{}, err
	}
	switch {
	case decision == policy.NotFound:
		return Rule{}, items.ErrNotFound
	case decision != policy.Allowed || (r.OwnerID != actor.ID && actor.Type != policy.Admin):
		return Rule{}, items.ErrForbidden
	}
	return r, nil
}

// Create makes a rule with a first draft. The creator becomes its owner: the rule runs with their rights.
func Create(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID, name string, container *string, raw json.RawMessage) (Rule, error) {
	name, err := cleanName(name)
	if err != nil {
		return Rule{}, err
	}
	def, err := ParseDefinition(raw)
	if err != nil {
		return Rule{}, items.InvalidField{Field: "definition", Message: err.Error()}
	}
	if err := validateForScope(def, "space"); err != nil {
		return Rule{}, items.InvalidField{Field: "definition", Message: err.Error()}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Rule{}, err
	}
	defer tx.Rollback(ctx)
	decision, _, err := policy.CanOnSpace(ctx, tx, actor, policy.Edit, spaceID)
	if err != nil {
		return Rule{}, err
	}
	if decision == policy.NotFound {
		return Rule{}, items.ErrNotFound
	}
	if decision != policy.Allowed {
		return Rule{}, items.ErrForbidden
	}
	if container != nil {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM items i JOIN item_locations l ON l.item_id=i.id WHERE i.id=$1::uuid AND l.space_id=$2::uuid
 AND i.type IN ('project','folder') AND i.deleted_at IS NULL AND l.deleted_root_id IS NULL)`, *container, spaceID).Scan(&ok); err != nil || !ok {
			return Rule{}, errors.Join(err, items.InvalidField{Field: "container_id", Message: "container_id must be a project or folder in this Space"})
		}
	}
	if err := checkReferences(ctx, tx, "space", spaceID, container, true, def); err != nil {
		return Rule{}, err
	}
	id, vid := uuid.New(), uuid.New()
	r, err := scanRule(tx.QueryRow(ctx, `INSERT INTO automation_rules(id,space_id,container_id,name,owner_id) VALUES ($1,$2::uuid,$3::uuid,$4,$5::uuid) RETURNING `+ruleColumns,
		id, spaceID, container, name, actor.ID))
	if err != nil {
		return Rule{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO automation_rule_versions(id,rule_id,version,definition,status,created_by) VALUES ($1,$2,1,$3,'draft',$4::uuid)`, vid, id, raw, actor.ID); err != nil {
		return Rule{}, err
	}
	if err := audit(ctx, tx, actor, "automation_rule.created", id.String(), map[string]any{"space_id": spaceID}); err != nil {
		return Rule{}, err
	}
	return r, tx.Commit(ctx)
}

// SaveDraft replaces the draft of a rule, or starts the next version when there is none. Published versions never change.
func SaveDraft(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ruleID string, raw json.RawMessage) (Version, error) {
	def, err := ParseDefinition(raw)
	if err != nil {
		return Version{}, items.InvalidField{Field: "definition", Message: err.Error()}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Version{}, err
	}
	defer tx.Rollback(ctx)
	rule, err := manage(ctx, tx, actor, ruleID, true)
	if err != nil {
		return Version{}, err
	}
	if actor.ID != rule.OwnerID {
		current, err := ruleDefinitions(ctx, tx, rule.ID)
		if err != nil {
			return Version{}, err
		}
		if err := requireAssignAuthority(ctx, tx, actor, rule, append(current, raw)...); err != nil {
			return Version{}, err
		}
	}
	if err := validateForScope(def, rule.Scope); err != nil {
		return Version{}, items.InvalidField{Field: "definition", Message: err.Error()}
	}
	if err := checkReferences(ctx, tx, rule.Scope, rule.SpaceID, rule.ContainerID, len(rule.TargetSpaceIDs) == 1, def); err != nil {
		return Version{}, err
	}
	var v Version
	err = tx.QueryRow(ctx, `UPDATE automation_rule_versions SET definition=$2 WHERE rule_id=$1 AND status='draft' RETURNING id::text,version,status,definition,created_at::text,published_at::text`,
		rule.ID, raw).Scan(&v.ID, &v.Version, &v.Status, &v.Definition, &v.CreatedAt, &v.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO automation_rule_versions(id,rule_id,version,definition,status,created_by)
SELECT gen_random_uuid(),$1,COALESCE(max(version),0)+1,$2,'draft',$3::uuid FROM automation_rule_versions WHERE rule_id=$1
RETURNING id::text,version,status,definition,created_at::text,published_at::text`, rule.ID, raw, actor.ID).Scan(&v.ID, &v.Version, &v.Status, &v.Definition, &v.CreatedAt, &v.PublishedAt)
	}
	if err != nil {
		return Version{}, err
	}
	if err := audit(ctx, tx, actor, "automation_rule.draft_saved", rule.ID, map[string]any{"version": v.Version}); err != nil {
		return Version{}, err
	}
	return v, tx.Commit(ctx)
}

// Publish makes the draft the current version: the previous one is retired and the rule points at the new one, in one transaction.
// Events already being handled keep the version they started with; rolling back is publishing an older definition as a new draft.
func Publish(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ruleID string) (Rule, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Rule{}, err
	}
	defer tx.Rollback(ctx)
	rule, err := manage(ctx, tx, actor, ruleID, true)
	if err != nil {
		return Rule{}, err
	}
	if actor.ID != rule.OwnerID {
		current, err := ruleDefinitions(ctx, tx, rule.ID)
		if err != nil {
			return Rule{}, err
		}
		if err := requireAssignAuthority(ctx, tx, actor, rule, current...); err != nil {
			return Rule{}, err
		}
	}
	var draft string
	err = tx.QueryRow(ctx, "SELECT id::text FROM automation_rule_versions WHERE rule_id=$1 AND status='draft'", rule.ID).Scan(&draft)
	if errors.Is(err, pgx.ErrNoRows) {
		return Rule{}, items.Invalid("there is no draft to publish")
	}
	if err != nil {
		return Rule{}, err
	}
	if _, err := tx.Exec(ctx, "UPDATE automation_rule_versions SET status='retired' WHERE rule_id=$1 AND status='published'", rule.ID); err != nil {
		return Rule{}, err
	}
	if _, err := tx.Exec(ctx, "UPDATE automation_rule_versions SET status='published', published_at=now() WHERE id=$1::uuid", draft); err != nil {
		return Rule{}, err
	}
	out, err := scanRule(tx.QueryRow(ctx, "UPDATE automation_rules SET current_version_id=$2::uuid, updated_at=now() WHERE id=$1::uuid RETURNING "+ruleColumns, rule.ID, draft))
	if err != nil {
		return Rule{}, err
	}
	if err := audit(ctx, tx, actor, "automation_rule.published", rule.ID, map[string]any{"version_id": draft}); err != nil {
		return Rule{}, err
	}
	return out, tx.Commit(ctx)
}

// Update renames a rule or switches it on or off. Switching it on revalidates the rule first (owner and dependencies), so a
// paused or invalid rule runs again only when it can. A rule can only be enabled once a version is published and is not archived.
func Update(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ruleID string, name *string, enabled *bool) (Rule, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Rule{}, err
	}
	defer tx.Rollback(ctx)
	rule, err := manage(ctx, tx, actor, ruleID, true)
	if err != nil {
		return Rule{}, err
	}
	if name != nil {
		clean, err := cleanName(*name)
		if err != nil {
			return Rule{}, err
		}
		rule.Name = clean
	}
	if enabled != nil {
		switch {
		case *enabled && rule.CurrentVersionID == nil:
			return Rule{}, items.Invalid("publish a version before enabling the rule")
		case *enabled && rule.ArchivedAt != nil:
			return Rule{}, items.Invalid("restore the rule before enabling it")
		case *enabled:
			def, err := publishedDefinition(ctx, tx, rule.ID)
			if err != nil {
				return Rule{}, err
			}
			if rule.Scope == "workspace" {
				if err := enableWorkspace(ctx, tx, rule, def); err != nil {
					return Rule{}, err
				}
			} else {
				code, err := revalidate(ctx, tx, rule.Scope, rule.OwnerID, rule.SpaceID, rule.ContainerID, def)
				if err != nil {
					return Rule{}, err
				}
				if code != "" {
					return Rule{}, items.InvalidField{Field: "enabled", Message: "the rule cannot run until this is fixed: " + code}
				}
			}
			rule.Health, rule.HealthReason = healthOK, ""
		}
		if *enabled != rule.Enabled {
			action := "automation_rule.enabled"
			if !*enabled {
				action = "automation_rule.disabled"
			}
			if err := audit(ctx, tx, actor, action, rule.ID, nil); err != nil {
				return Rule{}, err
			}
		}
		rule.Enabled = *enabled
	}
	out, err := scanRule(tx.QueryRow(ctx, "UPDATE automation_rules SET name=$2, enabled=$3, health=$4, health_reason=$5, updated_at=now() WHERE id=$1::uuid RETURNING "+ruleColumns,
		rule.ID, rule.Name, rule.Enabled, rule.Health, rule.HealthReason))
	if err != nil {
		return Rule{}, err
	}
	return out, tx.Commit(ctx)
}

// Get returns a rule with its versions, newest first, to anyone who can edit its Space.
func Get(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ruleID string) (Rule, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Rule{}, err
	}
	defer tx.Rollback(ctx)
	parsed, err := uuid.Parse(ruleID)
	if err != nil {
		return Rule{}, items.ErrNotFound
	}
	r, err := scanRule(tx.QueryRow(ctx, "SELECT "+ruleColumns+" FROM automation_rules WHERE id=$1", parsed))
	if errors.Is(err, pgx.ErrNoRows) {
		return Rule{}, items.ErrNotFound
	}
	if err != nil {
		return Rule{}, err
	}
	if r.Scope == "workspace" {
		if actor.Type != policy.Admin && r.OwnerID != actor.ID {
			return Rule{}, items.ErrNotFound
		}
	} else {
		decision, _, err := policy.CanOnSpace(ctx, tx, actor, policy.Edit, r.SpaceID)
		if err != nil {
			return Rule{}, err
		}
		if decision == policy.NotFound {
			return Rule{}, items.ErrNotFound
		}
		if decision != policy.Allowed {
			return Rule{}, items.ErrForbidden
		}
	}
	rows, err := tx.Query(ctx, `SELECT id::text,version,status,definition,created_at::text,published_at::text FROM automation_rule_versions WHERE rule_id=$1 ORDER BY version DESC`, parsed)
	if err != nil {
		return Rule{}, err
	}
	r.Versions, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (Version, error) {
		var v Version
		return v, row.Scan(&v.ID, &v.Version, &v.Status, &v.Definition, &v.CreatedAt, &v.PublishedAt)
	})
	return r, err
}

// List returns the rules of a Space for people who can edit it, with the registry columns (A2).
func List(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID string) ([]Rule, error) {
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
	return listRegistry(ctx, pool, "r.space_id=$1::uuid", spaceID)
}

// listRegistry is the registry query for the rules matching where. Its arguments are bound in order.
func listRegistry(ctx context.Context, pool *pgxpool.Pool, where string, args ...any) ([]Rule, error) {
	rows, err := pool.Query(ctx, `SELECT r.id::text,COALESCE(r.space_id::text,''),r.container_id::text,r.name,r.owner_id::text,r.enabled,r.current_version_id::text,
 r.health,r.health_reason,r.archived_at::text,r.scope,r.target_space_ids::text[],
 COALESCE(NULLIF(btrim(u.first_name||' '||u.last_name),''), u.email::text),
 c.title,
 COALESCE(COALESCE(cv.definition,lv.definition)->'when'->>'type',''),
 COALESCE(jsonb_array_length(COALESCE(cv.definition,lv.definition)->'then'),0),
 EXISTS(SELECT 1 FROM automation_rule_versions d WHERE d.rule_id=r.id AND d.status='draft'),
 lr.created_at::text, lr.status,
 (SELECT count(*) FROM automation_runs x WHERE x.rule_id=r.id AND x.status IN ('failed','denied','stopped') AND x.created_at > now() - interval '7 days')
FROM automation_rules r
JOIN users u ON u.id=r.owner_id
LEFT JOIN items c ON c.id=r.container_id
LEFT JOIN automation_rule_versions cv ON cv.id=r.current_version_id
LEFT JOIN LATERAL (SELECT definition FROM automation_rule_versions WHERE rule_id=r.id ORDER BY version DESC LIMIT 1) lv ON true
LEFT JOIN LATERAL (SELECT created_at,status FROM automation_runs WHERE rule_id=r.id ORDER BY created_at DESC, id DESC LIMIT 1) lr ON true
WHERE `+where+` ORDER BY r.created_at,r.id`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Rule, error) {
		var r Rule
		err := row.Scan(&r.ID, &r.SpaceID, &r.ContainerID, &r.Name, &r.OwnerID, &r.Enabled, &r.CurrentVersionID, &r.Health, &r.HealthReason, &r.ArchivedAt,
			&r.Scope, &r.TargetSpaceIDs, &r.OwnerName, &r.ContainerTitle, &r.Trigger, &r.ActionCount, &r.HasDraft, &r.LastRunAt, &r.LastRunStatus, &r.ErrorCount)
		return r, err
	})
}

// Duplicate copies the latest definition of a rule into a new, disabled rule owned by whoever duplicates it. Permissions are not
// copied: the copy runs with its new owner's rights, and it needs a publish and an enable before it acts.
func Duplicate(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ruleID string) (Rule, error) {
	src, err := Get(ctx, pool, actor, ruleID)
	if err != nil {
		return Rule{}, err
	}
	if len(src.Versions) == 0 {
		return Rule{}, items.Invalid("the rule has no version to copy")
	}
	name := []rune("Copy of " + src.Name)
	if len(name) > 120 {
		name = name[:120]
	}
	if src.Scope == "workspace" {
		return CreateWorkspace(ctx, pool, actor, string(name), src.Versions[0].Definition)
	}
	return Create(ctx, pool, actor, src.SpaceID, string(name), src.ContainerID, src.Versions[0].Definition)
}
