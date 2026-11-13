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
	ID               string    `json:"id"`
	SpaceID          string    `json:"space_id"`
	ContainerID      *string   `json:"container_id"`
	Name             string    `json:"name"`
	OwnerID          string    `json:"owner_id"`
	Enabled          bool      `json:"enabled"`
	CurrentVersionID *string   `json:"current_version_id"`
	Versions         []Version `json:"versions,omitempty"`
}

const ruleColumns = "id::text,space_id::text,container_id::text,name,owner_id::text,enabled,current_version_id::text"

func scanRule(row pgx.Row) (Rule, error) {
	var r Rule
	err := row.Scan(&r.ID, &r.SpaceID, &r.ContainerID, &r.Name, &r.OwnerID, &r.Enabled, &r.CurrentVersionID)
	return r, err
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 120 || strings.ContainsRune(name, 0) {
		return "", items.InvalidField{Field: "name", Message: "name must be 1 to 120 characters"}
	}
	return name, nil
}

// checkReferences verifies what a definition points at: statuses, users and the item type exist in this installation.
// Whether the owner may use them is decided when the rule runs.
func checkReferences(ctx context.Context, tx pgx.Tx, d Definition) error {
	bad := func(m string) error { return items.InvalidField{Field: "definition", Message: m} }
	exists := func(sql string, args ...any) (bool, error) {
		var ok bool
		err := tx.QueryRow(ctx, sql, args...).Scan(&ok)
		return ok, err
	}
	if d.When.ToStatusID != "" {
		if ok, err := exists("SELECT EXISTS(SELECT 1 FROM statuses WHERE id=$1::uuid)", d.When.ToStatusID); err != nil || !ok {
			return errors.Join(err, errIf(!ok, bad("when.to_status_id is not a status")))
		}
	}
	users := append([]string{}, d.When.UserID)
	for _, a := range d.Then {
		users = append(users, a.UserIDs...)
		if a.TypeKey != "" {
			if ok, err := exists("SELECT EXISTS(SELECT 1 FROM item_type_defs WHERE key=$1 AND active)", a.TypeKey); err != nil || !ok {
				return errors.Join(err, errIf(!ok, bad("type_key "+a.TypeKey+" is not an active item type")))
			}
		}
		if a.Type == "set_field" && a.Field == "status_id" {
			var id string
			if json.Unmarshal(a.Value, &id) != nil || !isUUID(id) {
				return bad("set_field status_id needs a status id")
			}
			if ok, err := exists("SELECT EXISTS(SELECT 1 FROM statuses WHERE id=$1::uuid)", id); err != nil || !ok {
				return errors.Join(err, errIf(!ok, bad("status_id is not a status")))
			}
		}
	}
	for _, id := range users {
		if id == "" {
			continue
		}
		if ok, err := exists("SELECT EXISTS(SELECT 1 FROM users WHERE id=$1::uuid AND status='active')", id); err != nil || !ok {
			return errors.Join(err, errIf(!ok, bad("every user in a rule must be an active user")))
		}
	}
	return nil
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
	if err := checkReferences(ctx, tx, def); err != nil {
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
	if err := checkReferences(ctx, tx, def); err != nil {
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
	return out, tx.Commit(ctx)
}

// Update renames a rule or switches it on or off. A rule can only be enabled once a version is published.
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
		if *enabled && rule.CurrentVersionID == nil {
			return Rule{}, items.Invalid("publish a version before enabling the rule")
		}
		rule.Enabled = *enabled
	}
	out, err := scanRule(tx.QueryRow(ctx, "UPDATE automation_rules SET name=$2, enabled=$3, updated_at=now() WHERE id=$1::uuid RETURNING "+ruleColumns, rule.ID, rule.Name, rule.Enabled))
	if err != nil {
		return Rule{}, err
	}
	return out, tx.Commit(ctx)
}

func Delete(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ruleID string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rule, err := manage(ctx, tx, actor, ruleID, true)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "UPDATE automation_rules SET current_version_id=NULL, enabled=false WHERE id=$1::uuid", rule.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM automation_rules WHERE id=$1::uuid", rule.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

// List returns the rules of a Space for people who can edit it.
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
	rows, err := tx.Query(ctx, "SELECT "+ruleColumns+" FROM automation_rules WHERE space_id=$1::uuid ORDER BY created_at,id", spaceID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Rule, error) { return scanRule(row) })
}
