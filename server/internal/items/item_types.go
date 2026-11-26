package items

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ItemType struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	Icon    string `json:"icon"`
	Base    string `json:"base"`
	Builtin bool   `json:"is_builtin"`
	Active  bool   `json:"active"`
	// QuickAccess puts an active custom type in the Create menu. It configures visibility only, never permission.
	QuickAccess       bool           `json:"quick_access"`
	AllowedChildren   []string       `json:"allowed_children"`
	DefaultWorkflowID *string        `json:"default_workflow_id"`
	Defaults          map[string]any `json:"defaults"`
	Layout            map[string]any `json:"layout"`
	Version           int            `json:"version"`
	// SpaceID is nil for Workspace types (usable in every Space) and the Space the type belongs to otherwise.
	SpaceID *string `json:"space_id"`
}

const itemTypeColumns = `id::text,key,name,icon,base::text,is_builtin,active,
 ARRAY(SELECT unnest(allowed_children)::text),default_workflow_id::text,defaults,layout,version,space_id::text,quick_access`

func scanItemType(row pgx.Row) (ItemType, error) {
	var t ItemType
	err := row.Scan(&t.ID, &t.Key, &t.Name, &t.Icon, &t.Base, &t.Builtin, &t.Active, &t.AllowedChildren, &t.DefaultWorkflowID, &t.Defaults, &t.Layout, &t.Version, &t.SpaceID, &t.QuickAccess)
	return t, err
}

type dbQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// ListItemTypes lists the types the actor may see: Workspace types, and Space types of Spaces the actor can view.
// A system administrator sees every type, because the settings page manages them all.
func ListItemTypes(ctx context.Context, db dbQuerier, actor policy.Actor) ([]ItemType, error) {
	visible, err := policy.VisibleSpaces(ctx, db, actor, "")
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, space := range visible {
		if space.Caps.Has(policy.View) {
			ids = append(ids, space.ID)
		}
	}
	rows, err := db.Query(ctx, "SELECT "+itemTypeColumns+" FROM item_type_defs WHERE space_id IS NULL OR $1 OR space_id::text = ANY($2::text[]) ORDER BY is_builtin DESC, lower(name)",
		actor.Type == policy.Admin, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ItemType, error) { return scanItemType(row) })
}

func getItemType(ctx context.Context, db dbQuerier, where string, arg any) (ItemType, error) {
	t, err := scanItemType(db.QueryRow(ctx, "SELECT "+itemTypeColumns+" FROM item_type_defs WHERE "+where, arg))
	if errors.Is(err, pgx.ErrNoRows) {
		return ItemType{}, ErrNotFound
	}
	return t, err
}

var (
	typeKeyPattern  = regexp.MustCompile(`^[a-z][a-z0-9_]{1,39}$`)
	typeIconPattern = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)
)

// ItemTypeInput carries the writable fields; nil means "leave as is" on update.
type ItemTypeInput struct {
	// SpaceID is read on create only: nil makes a Workspace type. The scope cannot change later.
	SpaceID           *string         `json:"space_id"`
	Key               *string         `json:"key"`
	Name              *string         `json:"name"`
	Icon              *string         `json:"icon"`
	Base              *string         `json:"base"`
	Active            *bool           `json:"active"`
	QuickAccess       *bool           `json:"quick_access"`
	AllowedChildren   *[]string       `json:"allowed_children"`
	DefaultWorkflowID json.RawMessage `json:"default_workflow_id"`
	Defaults          map[string]any  `json:"defaults"`
	Layout            map[string]any  `json:"layout"`
}

func requireAdmin(actor policy.Actor) error {
	if actor.Type != policy.Admin {
		return ErrForbidden
	}
	return nil
}

// authorizeTypeScope: a Workspace type is managed by the system administrator; a Space type by whoever can edit that Space.
func authorizeTypeScope(ctx context.Context, db dbQuerier, actor policy.Actor, spaceID *string) error {
	if spaceID == nil {
		return requireAdmin(actor)
	}
	decision, _, err := policy.CanOnSpace(ctx, db, actor, policy.Edit, *spaceID)
	if err != nil {
		return err
	}
	switch decision {
	case policy.NotFound:
		return ErrNotFound
	case policy.Forbidden:
		return ErrForbidden
	}
	return nil
}

// validateTypeFields checks everything that does not need the database.
func validateTypeFields(in ItemTypeInput) error {
	if in.Key != nil && !typeKeyPattern.MatchString(*in.Key) {
		return InvalidField{"key", "key must be 2 to 40 lowercase letters, digits or underscores, starting with a letter"}
	}
	if in.Name != nil {
		if n := utf8.RuneCountInString(strings.TrimSpace(*in.Name)); n < 1 || n > 80 {
			return InvalidField{"name", "name must be 1 to 80 characters"}
		}
	}
	if in.Icon != nil && !typeIconPattern.MatchString(*in.Icon) {
		return InvalidField{"icon", "icon must be a name of lowercase letters, digits and dashes"}
	}
	if in.Base != nil && !slices.Contains([]string{"task", "project", "folder"}, *in.Base) {
		return InvalidField{"base", "base must be project, folder or task"}
	}
	for key, value := range in.Defaults {
		switch key {
		case "importance":
			if v, _ := value.(string); !slices.Contains([]string{"high", "normal", "low"}, v) {
				return InvalidField{"defaults.importance", "importance must be high, normal or low"}
			}
		case "inherit": // fields a new child copies from its parent when the caller leaves them out
			list, ok := value.([]any)
			if !ok || len(list) > 1 || (len(list) == 1 && list[0] != "importance") {
				return InvalidField{"defaults.inherit", "inherit may list only importance"}
			}
		case "duration_days", "effort_minutes":
			if v, ok := value.(float64); !ok || v < 0 || v != float64(int64(v)) || v > 2147483647 {
				return InvalidField{"defaults." + key, key + " must be a non-negative whole number"}
			}
		default:
			return InvalidField{"defaults." + key, "unknown default; use importance, duration_days or effort_minutes"}
		}
	}
	return validateLayout(in.Layout)
}

var layoutBuiltins = []string{"status", "assignees", "dates", "importance", "effort", "description", "locations", "subitems", "attachments", "comments", "approvals", "activity"}

// validateLayout accepts {"sections":[{"title":..,"fields":[..]}]} where a field is a panel block or a custom field id (CUST-04).
func validateLayout(layout map[string]any) error {
	if len(layout) == 0 {
		return nil
	}
	bad := func(message string) error { return InvalidField{"layout", message} }
	sections, ok := layout["sections"].([]any)
	if len(layout) != 1 || !ok || len(sections) == 0 || len(sections) > 10 {
		return bad("layout must hold 1 to 10 sections")
	}
	seen := map[string]bool{}
	for _, raw := range sections {
		section, ok := raw.(map[string]any)
		title, _ := section["title"].(string)
		fields, _ := section["fields"].([]any)
		if !ok || len(section) != 2 || utf8.RuneCountInString(strings.TrimSpace(title)) < 1 || utf8.RuneCountInString(title) > 60 || len(fields) > 40 {
			return bad("each section needs a title of 1 to 60 characters and up to 40 fields")
		}
		for _, f := range fields {
			key, _ := f.(string)
			if !slices.Contains(layoutBuiltins, key) && !validFieldID(key) {
				return bad("unknown field " + key)
			}
			if seen[key] {
				return bad("field " + key + " appears twice")
			}
			seen[key] = true
		}
	}
	return nil
}

// checkChildren: every allowed child exists, and a task-class type may only contain task-class types.
func checkChildren(ctx context.Context, db dbQuerier, base string, ids []string) error {
	for _, id := range ids {
		parsed, err := uuid.Parse(id)
		if err != nil {
			return InvalidField{"allowed_children", "allowed_children must be a list of item type ids"}
		}
		var childBase string
		if err := db.QueryRow(ctx, "SELECT base::text FROM item_type_defs WHERE id=$1", parsed).Scan(&childBase); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return InvalidField{"allowed_children", "unknown item type " + id}
			}
			return err
		}
		if base == "task" && childBase != "task" {
			return InvalidField{"allowed_children", "a task type can contain only task types"}
		}
	}
	return nil
}

func checkTypeWorkflow(ctx context.Context, db dbQuerier, base string, raw json.RawMessage) (*uuid.UUID, error) {
	if raw == nil {
		return nil, nil
	}
	var id *uuid.UUID
	if json.Unmarshal(raw, &id) != nil {
		return nil, InvalidField{"default_workflow_id", "default_workflow_id must be a workflow id or null"}
	}
	if id == nil {
		return nil, nil
	}
	if base == "task" {
		return nil, InvalidField{"default_workflow_id", "a task takes its workflow from its project or folder"}
	}
	var exists bool
	if err := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflows WHERE id=$1)", *id).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, InvalidField{"default_workflow_id", "unknown workflow"}
	}
	return id, nil
}

func CreateItemType(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, in ItemTypeInput) (ItemType, error) {
	if in.Key == nil || in.Name == nil || in.Base == nil {
		return ItemType{}, Invalid("key, name and base are required")
	}
	if err := validateTypeFields(in); err != nil {
		return ItemType{}, err
	}
	if in.SpaceID != nil {
		if _, err := uuid.Parse(*in.SpaceID); err != nil {
			return ItemType{}, InvalidField{"space_id", "space_id must be a Space id or null"}
		}
	}
	icon := "circle"
	if in.Icon != nil {
		icon = *in.Icon
	}
	if in.AllowedChildren == nil {
		in.AllowedChildren = &[]string{}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return ItemType{}, err
	}
	defer tx.Rollback(ctx)
	if err := authorizeTypeScope(ctx, tx, actor, in.SpaceID); err != nil {
		return ItemType{}, err
	}
	if err := checkChildren(ctx, tx, *in.Base, *in.AllowedChildren); err != nil {
		return ItemType{}, err
	}
	workflow, err := checkTypeWorkflow(ctx, tx, *in.Base, in.DefaultWorkflowID)
	if err != nil {
		return ItemType{}, err
	}
	if in.Defaults == nil {
		in.Defaults = map[string]any{}
	}
	t, err := scanItemType(tx.QueryRow(ctx, `INSERT INTO item_type_defs(key,name,icon,base,allowed_children,default_workflow_id,defaults,layout,space_id)
VALUES ($1,btrim($2),$3,$4::item_type,$5::uuid[],$6,$7,COALESCE($8::jsonb,'{}'::jsonb),$9::uuid) RETURNING `+itemTypeColumns,
		*in.Key, *in.Name, icon, *in.Base, *in.AllowedChildren, workflow, in.Defaults, in.Layout, in.SpaceID))
	if pgCode(err) == "23505" {
		return ItemType{}, InvalidField{"name", "an item type with that key or name already exists"}
	}
	if err != nil {
		return ItemType{}, err
	}
	return t, tx.Commit(ctx)
}

// UpdateItemType changes a type. Built-in types may be renamed and re-iconed but never deactivated or re-classed.
func UpdateItemType(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, id string, version int, in ItemTypeInput) (ItemType, error) {
	if in.Key != nil || in.Base != nil || in.SpaceID != nil {
		return ItemType{}, Invalid("key, base and Space cannot be changed")
	}
	if err := validateTypeFields(in); err != nil {
		return ItemType{}, err
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return ItemType{}, ErrNotFound
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return ItemType{}, err
	}
	defer tx.Rollback(ctx)
	current, err := getItemType(ctx, tx, "id=$1 FOR UPDATE", parsed)
	if err != nil {
		return ItemType{}, err
	}
	if err := authorizeTypeScope(ctx, tx, actor, current.SpaceID); err != nil {
		return ItemType{}, err
	}
	if current.Version != version {
		return ItemType{}, ErrConflict
	}
	if in.Active != nil && !*in.Active && current.Builtin {
		return ItemType{}, Invalid("built-in item types cannot be deactivated")
	}
	if in.AllowedChildren != nil {
		if err := checkChildren(ctx, tx, current.Base, *in.AllowedChildren); err != nil {
			return ItemType{}, err
		}
	}
	workflow, err := checkTypeWorkflow(ctx, tx, current.Base, in.DefaultWorkflowID)
	if err != nil {
		return ItemType{}, err
	}
	setWorkflow := in.DefaultWorkflowID != nil
	if in.QuickAccess != nil && *in.QuickAccess {
		if current.Builtin {
			return ItemType{}, InvalidField{"quick_access", "built-in types have no Quick Access setting"}
		}
		if active := in.Active == nil && current.Active || in.Active != nil && *in.Active; !active {
			return ItemType{}, InvalidField{"quick_access", "an inactive type cannot be in Quick Access"}
		}
	}
	updated, err := scanItemType(tx.QueryRow(ctx, `UPDATE item_type_defs SET name=COALESCE(btrim($2),name), icon=COALESCE($3,icon), active=COALESCE($4,active),
 allowed_children=COALESCE($5::uuid[],allowed_children), default_workflow_id=CASE WHEN $6 THEN $7 ELSE default_workflow_id END,
 defaults=COALESCE($8,defaults), layout=COALESCE($9,layout), quick_access=COALESCE($10,quick_access), version=version+1 WHERE id=$1 RETURNING `+itemTypeColumns,
		parsed, in.Name, in.Icon, in.Active, in.AllowedChildren, setWorkflow, workflow, in.Defaults, in.Layout, in.QuickAccess))
	if pgCode(err) == "23505" {
		return ItemType{}, InvalidField{"name", "an item type with that name already exists"}
	}
	if err != nil {
		return ItemType{}, err
	}
	return updated, tx.Commit(ctx)
}

// checkAllowedUnder refuses to place an existing item under a parent whose type lists allowed children that do not include it.
func checkAllowedUnder(ctx context.Context, db dbQuerier, parent uuid.UUID, item any) error {
	var ok bool
	var parentName, childName string
	if err := db.QueryRow(ctx, `SELECT cardinality(pt.allowed_children)=0 OR ct.id=ANY(pt.allowed_children), pt.name, ct.name
FROM items p JOIN item_type_defs pt ON pt.id=p.type_id, items c JOIN item_type_defs ct ON ct.id=c.type_id WHERE p.id=$1 AND c.id=$2`, parent, item).Scan(&ok, &parentName, &childName); err != nil {
		return err
	}
	if !ok {
		return Invalid("a " + childName + " cannot be placed inside a " + parentName)
	}
	return nil
}

type RelationTarget struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	TypeKey string `json:"type_key"`
}

type RelationGroup struct {
	FieldID string           `json:"field_id"`
	Name    string           `json:"name"`
	Items   []RelationTarget `json:"items"`
	Hidden  int              `json:"hidden"` // targets the caller may not see: counted, never named
}

// ItemRelations lists the targets of an item's relation fields. Only targets the caller can view carry a title.
func ItemRelations(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, itemID string) ([]RelationGroup, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	caps, err := policy.ItemCaps(ctx, tx, actor, itemID)
	if err != nil {
		return nil, err
	}
	if !caps.Has(policy.View) {
		return nil, ErrNotFound
	}
	spaces, err := policy.VisibleSpaces(ctx, tx, actor, "")
	if err != nil {
		return nil, err
	}
	spaceIDs := []string{}
	for _, space := range spaces {
		if space.Caps.Has(policy.View) {
			spaceIDs = append(spaceIDs, space.ID)
		}
	}
	hiddenRel, err := HiddenFieldIDs(ctx, tx, actor, GrantView)
	if err != nil {
		return nil, err
	}
	type row struct{ field, name, id, title, typ, key string }
	rows, err := tx.Query(ctx, `SELECT f.id::text,f.name,t.id::text,t.title,t.type::text,ty.key
FROM item_relations r JOIN custom_fields f ON f.id=r.field_id AND f.archived_at IS NULL
JOIN items t ON t.id=r.to_item_id AND t.deleted_at IS NULL JOIN item_type_defs ty ON ty.id=t.type_id
WHERE r.from_item_id=$1::uuid AND f.space_id=ANY(ARRAY(SELECT unnest($2::text[])::uuid)) AND NOT (f.id::text = ANY($3::text[]))
  AND EXISTS(SELECT 1 FROM item_locations l WHERE l.item_id=r.from_item_id AND l.deleted_root_id IS NULL
    AND l.space_id=ANY(ARRAY(SELECT unnest($2::text[])::uuid)) AND custom_field_applies(r.from_item_id, f.id, l.space_id))
ORDER BY f.position,t.seq`, itemID, spaceIDs, nonNilStrings(hiddenRel))
	if err != nil {
		return nil, err
	}
	all, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var x row
		return x, r.Scan(&x.field, &x.name, &x.id, &x.title, &x.typ, &x.key)
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(all))
	for _, r := range all {
		ids = append(ids, r.id)
	}
	targetCaps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		return nil, err
	}
	groups := []RelationGroup{}
	for _, r := range all {
		if len(groups) == 0 || groups[len(groups)-1].FieldID != r.field {
			groups = append(groups, RelationGroup{FieldID: r.field, Name: r.name, Items: []RelationTarget{}})
		}
		g := &groups[len(groups)-1]
		if targetCaps[r.id].Has(policy.View) {
			g.Items = append(g.Items, RelationTarget{ID: r.id, Title: r.title, Type: r.typ, TypeKey: r.key})
		} else {
			g.Hidden++
		}
	}
	return groups, tx.Commit(ctx)
}

// ItemTypeUsage counts the live Items of every item type (administrators only; the registry shows it as a column).
func ItemTypeUsage(ctx context.Context, db dbQuerier, actor policy.Actor) (map[string]int, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	rows, err := db.Query(ctx, `SELECT t.id::text, count(i.id) FROM item_type_defs t
LEFT JOIN items i ON i.type_id=t.id AND i.deleted_at IS NULL GROUP BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// SpaceItemTypeUsage counts the live items in one Space, per item type. Anyone who can edit the Space sees it (the Space's own admin view).
func SpaceItemTypeUsage(ctx context.Context, db dbQuerier, actor policy.Actor, spaceID string) (map[string]int, error) {
	if _, err := uuid.Parse(spaceID); err != nil {
		return nil, ErrNotFound // DEF-021
	}
	if err := authorizeTypeScope(ctx, db, actor, &spaceID); err != nil {
		return nil, err
	}
	rows, err := db.Query(ctx, `SELECT i.type_id::text, count(DISTINCT i.id) FROM items i
JOIN item_locations l ON l.item_id=i.id WHERE l.space_id=$1::uuid AND i.deleted_at IS NULL GROUP BY i.type_id`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}
