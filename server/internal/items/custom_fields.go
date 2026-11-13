package items

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomFieldOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Color string `json:"color"`
}

type CustomField struct {
	ID         string              `json:"id"`
	SpaceID    string              `json:"space_id"`
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Options    []CustomFieldOption `json:"options"`
	Position   int                 `json:"position"`
	ArchivedAt *time.Time          `json:"archived_at"`
	Config     CustomFieldConfig   `json:"config"`
}

type CustomFieldCreate struct {
	Name    string              `json:"name"`
	Type    string              `json:"type"`
	Options []CustomFieldOption `json:"options"`
	Config  CustomFieldConfig   `json:"config"`
}

type CustomFieldUpdate struct {
	Name     *string              `json:"name"`
	Options  *[]CustomFieldOption `json:"options"`
	Position *int                 `json:"position"`
	Config   *CustomFieldConfig   `json:"config"`
}

const customFieldColumns = "id::text, space_id::text, name, type, options, position, archived_at, config"

var optionColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func fieldName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if size := utf8.RuneCountInString(name); size < 1 || size > 120 || strings.ContainsRune(name, 0) {
		return "", Invalid("name or option label must be 1 to 120 characters")
	}
	return name, nil
}

func validFieldID(raw string) bool {
	id, err := uuid.Parse(raw)
	return err == nil && id != uuid.Nil && id.String() == raw
}

func validateOptions(input []CustomFieldOption) ([]CustomFieldOption, error) {
	if len(input) < 1 || len(input) > 100 {
		return nil, Invalid("a custom field must have 1 to 100 options")
	}
	out := slices.Clone(input)
	ids := make(map[string]bool, len(out))
	for i, option := range out {
		label, err := fieldName(option.Label)
		if err != nil {
			return nil, err
		}
		if !validFieldID(option.ID) || ids[option.ID] || !optionColor.MatchString(option.Color) {
			return nil, Invalid("options require distinct UUIDs and #RRGGBB colors")
		}
		out[i].Label = label
		ids[option.ID] = true
	}
	return out, nil
}

func fieldSpace(ctx context.Context, tx pgx.Tx, actor policy.Actor, spaceID string, manage bool) (string, error) {
	id, err := uuid.Parse(spaceID)
	if err != nil {
		return "", ErrNotFound
	}
	space, _, err := policy.SpaceAccessFor(ctx, tx, actor, id.String())
	if err != nil {
		return "", err
	}
	if !space.Caps.Has(policy.View) {
		return "", ErrNotFound
	}
	if manage {
		if !space.Caps.Contains(policy.RoleCaps(policy.Full)) {
			return "", ErrForbidden
		}
		if _, err := tx.Exec(ctx, "SELECT id FROM spaces WHERE id=$1 FOR UPDATE", id); err != nil {
			return "", err
		}
	}
	return id.String(), nil
}

func ListCustomFields(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID string) ([]CustomField, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	id, err := fieldSpace(ctx, tx, actor, spaceID, false)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, "SELECT "+customFieldColumns+" FROM custom_fields WHERE space_id=$1::uuid ORDER BY position", id)
	if err != nil {
		return nil, err
	}
	fields, err := pgx.CollectRows(rows, pgx.RowToStructByPos[CustomField])
	if err != nil {
		return nil, err
	}
	if fields == nil {
		fields = []CustomField{}
	}
	return fields, tx.Commit(ctx)
}

func CreateCustomField(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID string, input CustomFieldCreate) (CustomField, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return CustomField{}, err
	}
	defer tx.Rollback(ctx)
	id, err := fieldSpace(ctx, tx, actor, spaceID, true)
	if err != nil {
		return CustomField{}, writeError(err)
	}
	name, err := fieldName(input.Name)
	if err != nil {
		return CustomField{}, err
	}
	options, err := validateCustomFieldOptions(input.Type, input.Options)
	if err != nil {
		return CustomField{}, err
	}
	if err := validateCustomFieldConfig(input.Type, input.Config); err != nil {
		return CustomField{}, err
	}
	field, err := scanCustomField(tx.QueryRow(ctx, `INSERT INTO custom_fields(space_id,name,type,options,position,config)
SELECT $1::uuid,$2,$3,$4,COALESCE(max(position)+1,0),$5 FROM custom_fields WHERE space_id=$1::uuid
RETURNING `+customFieldColumns, id, name, input.Type, options, input.Config))
	if err != nil {
		return CustomField{}, writeError(err)
	}
	return field, commitCustomFields(ctx, tx)
}

func UpdateCustomField(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID, fieldID string, input CustomFieldUpdate) (CustomField, error) {
	parsed, err := uuid.Parse(fieldID)
	if err != nil {
		return CustomField{}, ErrNotFound
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return CustomField{}, err
	}
	defer tx.Rollback(ctx)
	space, err := fieldSpace(ctx, tx, actor, spaceID, true)
	if err != nil {
		return CustomField{}, writeError(err)
	}
	field, err := scanCustomField(tx.QueryRow(ctx, "SELECT "+customFieldColumns+
		" FROM custom_fields WHERE id=$1 AND space_id=$2::uuid FOR UPDATE", parsed, space))
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomField{}, ErrNotFound
	}
	if err != nil {
		return CustomField{}, writeError(err)
	}
	if field.ArchivedAt != nil {
		return CustomField{}, Invalid("archived custom fields cannot be changed")
	}
	if input.Name == nil && input.Options == nil && input.Position == nil && input.Config == nil {
		return CustomField{}, Invalid("at least one custom field property is required")
	}
	if input.Name != nil {
		field.Name, err = fieldName(*input.Name)
		if err != nil {
			return CustomField{}, err
		}
	}
	if input.Options != nil {
		options, err := validateCustomFieldOptions(field.Type, *input.Options)
		if err != nil {
			return CustomField{}, err
		}
		if err := preserveUsedOptions(ctx, tx, field, options); err != nil {
			return CustomField{}, writeError(err)
		}
		field.Options = options
	}
	if input.Config != nil {
		if err := validateCustomFieldConfig(field.Type, *input.Config); err != nil {
			return CustomField{}, err
		}
		if err := preserveCustomFieldConfig(ctx, tx, field, *input.Config); err != nil {
			return CustomField{}, writeError(err)
		}
		field.Config = *input.Config
	}
	if input.Position != nil && *input.Position != field.Position {
		if err := moveCustomField(ctx, tx, field, *input.Position); err != nil {
			return CustomField{}, writeError(err)
		}
		field.Position = *input.Position
	}
	_, err = tx.Exec(ctx, "UPDATE custom_fields SET name=$2, options=$3, position=$4, config=$5 WHERE id=$1", parsed, field.Name, field.Options, field.Position, field.Config)
	if err != nil {
		return CustomField{}, writeError(err)
	}
	return field, commitCustomFields(ctx, tx)
}

func ArchiveCustomField(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID, fieldID string) error {
	parsed, err := uuid.Parse(fieldID)
	if err != nil {
		return ErrNotFound
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	space, err := fieldSpace(ctx, tx, actor, spaceID, true)
	if err != nil {
		return writeError(err)
	}
	tag, err := tx.Exec(ctx, "UPDATE custom_fields SET archived_at=COALESCE(archived_at,now()) WHERE id=$1 AND space_id=$2::uuid", parsed, space)
	if err != nil {
		return writeError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return commitCustomFields(ctx, tx)
}

func scanCustomField(row pgx.Row) (CustomField, error) {
	var field CustomField
	err := row.Scan(&field.ID, &field.SpaceID, &field.Name, &field.Type, &field.Options, &field.Position, &field.ArchivedAt, &field.Config)
	return field, err
}

func commitCustomFields(ctx context.Context, tx pgx.Tx) error {
	err := tx.Commit(ctx)
	if pgCode(err) == "23505" {
		return ErrConflict
	}
	return writeError(err)
}

func moveCustomField(ctx context.Context, tx pgx.Tx, field CustomField, target int) error {
	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM custom_fields WHERE space_id=$1::uuid", field.SpaceID).Scan(&count); err != nil {
		return err
	}
	if target < 0 || target >= count {
		return Invalid("position is outside the custom field list")
	}
	_, err := tx.Exec(ctx, `UPDATE custom_fields
SET position=position+CASE WHEN $2::integer<$3::integer THEN -1 ELSE 1 END
WHERE space_id=$1::uuid AND position BETWEEN least($2::integer,$3::integer)
  AND greatest($2::integer,$3::integer) AND position<>$2::integer`,
		field.SpaceID, field.Position, target)
	return err
}

func preserveUsedOptions(ctx context.Context, tx pgx.Tx, field CustomField, replacement []CustomFieldOption) error {
	var removed []string
	for _, option := range field.Options {
		if !slices.ContainsFunc(replacement, func(candidate CustomFieldOption) bool { return candidate.ID == option.ID }) {
			removed = append(removed, option.ID)
		}
	}
	if len(removed) == 0 {
		return nil
	}
	var used bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM items WHERE fields ? $1
  AND ((fields->>$1)=ANY($2::text[]) OR (jsonb_typeof(fields->$1)='array' AND (fields->$1) ?| $2::text[])))`,
		field.ID, removed).Scan(&used)
	if err == nil && used {
		return Invalid("an option still in use cannot be removed")
	}
	return err
}

// updateCustomFieldValues merges only the submitted keys, preserving values in other Spaces.
func updateCustomFieldValues(ctx context.Context, tx pgx.Tx, actor policy.Actor, itemID string, raw json.RawMessage) (json.RawMessage, error) {
	var updates map[string]json.RawMessage
	if json.Unmarshal(raw, &updates) != nil || updates == nil {
		return nil, InvalidField{"fields", "fields must be an object keyed by custom field UUID"}
	}
	ids := make([]string, 0, len(updates))
	for id := range updates {
		if !validFieldID(id) {
			return nil, InvalidField{"fields", "unknown or unavailable custom field"}
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var current map[string]json.RawMessage
	var kind string
	if err := tx.QueryRow(ctx, "SELECT fields,type::text FROM items WHERE id=$1::uuid", itemID).Scan(&current, &kind); err != nil {
		return nil, err
	}
	if current == nil {
		current = make(map[string]json.RawMessage)
	}
	if len(ids) == 0 {
		return json.Marshal(current)
	}
	spaces, err := policy.VisibleSpaces(ctx, tx, actor, "")
	if err != nil {
		return nil, err
	}
	visibleSpaces := make([]string, 0, len(spaces))
	for _, space := range spaces {
		if space.Caps.Has(policy.View) {
			visibleSpaces = append(visibleSpaces, space.ID)
		}
	}
	rows, err := tx.Query(ctx, "SELECT "+customFieldColumns+` FROM custom_fields f
WHERE f.id=ANY(ARRAY(SELECT unnest($2::text[])::uuid))
AND f.space_id=ANY(ARRAY(SELECT unnest($3::text[])::uuid))
AND EXISTS(SELECT 1 FROM item_locations l WHERE l.item_id=$1::uuid AND l.space_id=f.space_id AND l.deleted_root_id IS NULL)
ORDER BY f.id FOR SHARE OF f`, itemID, ids, visibleSpaces)
	if err != nil {
		return nil, err
	}
	definitions, err := pgx.CollectRows(rows, pgx.RowToStructByPos[CustomField])
	if err != nil {
		return nil, err
	}
	if len(definitions) != len(ids) {
		return nil, InvalidField{"fields", "unknown or unavailable custom field"}
	}
	for _, field := range definitions {
		value := updates[field.ID]
		if string(value) == "null" {
			delete(current, field.ID)
			continue
		}
		if field.ArchivedAt != nil {
			return nil, InvalidField{"fields", "archived custom fields cannot receive new values"}
		}
		if field.Config.ProjectBudget != nil && *field.Config.ProjectBudget && kind != "project" {
			return nil, InvalidField{"fields." + field.ID, "budget can only be set on projects"}
		}
		normalized, err := normalizeCustomFieldValue(field, value)
		if err != nil {
			return nil, err
		}
		if field.Type == "people" {
			if err := validateNewFieldPeople(ctx, tx, actor, field, current[field.ID], normalized); err != nil {
				return nil, err
			}
		}
		current[field.ID] = normalized
	}
	return json.Marshal(current)
}

func validSelectValue(field CustomField, raw json.RawMessage) bool {
	var ids []string
	switch field.Type {
	case "single_select":
		var id string
		if json.Unmarshal(raw, &id) != nil {
			return false
		}
		ids = []string{id}
	case "multi_select":
		if json.Unmarshal(raw, &ids) != nil || ids == nil || len(ids) > len(field.Options) {
			return false
		}
	default:
		return false
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if seen[id] || !slices.ContainsFunc(field.Options, func(option CustomFieldOption) bool { return option.ID == id }) {
			return false
		}
		seen[id] = true
	}
	return true
}
