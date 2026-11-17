package items

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TypeField pairs an item type with a custom field; Default is the value a new item of the type starts with (null = none).
type TypeField struct {
	FieldID  string          `json:"field_id"`
	SpaceID  string          `json:"space_id"`
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Archived bool            `json:"archived"`
	Default  json.RawMessage `json:"default"`
}

func ListTypeFields(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, typeID string) ([]TypeField, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(typeID)
	if err != nil {
		return nil, ErrNotFound
	}
	if _, err := getItemType(ctx, pool, "id=$1", id); err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT f.id::text,COALESCE(f.space_id::text,''),f.name,f.type,f.archived_at IS NOT NULL,COALESCE(tf.default_value,'null'::jsonb)
FROM item_type_fields tf JOIN custom_fields f ON f.id=tf.field_id WHERE tf.type_id=$1 ORDER BY f.space_id NULLS FIRST,f.position`, id)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (TypeField, error) {
		var t TypeField
		return t, r.Scan(&t.FieldID, &t.SpaceID, &t.Name, &t.Type, &t.Archived, &t.Default)
	})
}

// SetTypeField makes a custom field apply to a type; a non-null default is validated by the field's own type rules.
func SetTypeField(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, typeID, fieldID string, def json.RawMessage) (TypeField, error) {
	if err := requireAdmin(actor); err != nil {
		return TypeField{}, err
	}
	tid, err1 := uuid.Parse(typeID)
	fid, err2 := uuid.Parse(fieldID)
	if err1 != nil || err2 != nil {
		return TypeField{}, ErrNotFound
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return TypeField{}, err
	}
	defer tx.Rollback(ctx)
	t, err := getItemType(ctx, tx, "id=$1 FOR SHARE", tid)
	if err != nil {
		return TypeField{}, err
	}
	field, err := scanCustomField(tx.QueryRow(ctx, "SELECT "+customFieldColumns+" FROM custom_fields WHERE id=$1 FOR SHARE", fid))
	if errors.Is(err, pgx.ErrNoRows) {
		return TypeField{}, ErrNotFound
	}
	if err != nil {
		return TypeField{}, err
	}
	if field.ArchivedAt != nil {
		return TypeField{}, InvalidField{"field_id", "archived custom fields cannot be attached to a type"}
	}
	// Workspace types use Workspace fields only (decision: Workspace only).
	if t.SpaceID == nil && field.SpaceID != "" {
		return TypeField{}, InvalidField{"field_id", "a Workspace item type can use only Workspace custom fields"}
	}
	var stored json.RawMessage
	if len(def) > 0 && string(def) != "null" {
		if !defaultable(field, t.Base) {
			return TypeField{}, InvalidField{"default", "this field type cannot have a default"}
		}
		if stored, err = normalizeCustomFieldValue(field, def); err != nil {
			return TypeField{}, err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO item_type_fields(type_id,field_id,default_value) VALUES($1,$2,$3)
ON CONFLICT(type_id,field_id) DO UPDATE SET default_value=excluded.default_value`, tid, fid, stored); err != nil {
		return TypeField{}, err
	}
	if stored == nil {
		stored = json.RawMessage("null")
	}
	return TypeField{field.ID, field.SpaceID, field.Name, field.Type, false, stored}, tx.Commit(ctx)
}

func DeleteTypeField(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, typeID, fieldID string) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	tid, err1 := uuid.Parse(typeID)
	fid, err2 := uuid.Parse(fieldID)
	if err1 != nil || err2 != nil {
		return ErrNotFound
	}
	tag, err := pool.Exec(ctx, "DELETE FROM item_type_fields WHERE type_id=$1 AND field_id=$2", tid, fid)
	if err == nil && tag.RowsAffected() == 0 {
		err = ErrNotFound
	}
	return err
}

// defaultable: values that need no lookup of people, relations or other items.
func defaultable(field CustomField, base string) bool {
	switch field.Type {
	case "people", "relation", "formula":
		return false
	}
	return field.Config.ProjectBudget == nil || !*field.Config.ProjectBudget || base == "project"
}

// typeFieldDefaults returns the default values for fields of the given Spaces that apply to the type. A default that no
// longer validates (an option was removed, say) is skipped rather than failing the create; archived fields are skipped too.
func typeFieldDefaults(ctx context.Context, tx pgx.Tx, t ItemType, spaces []uuid.UUID) (map[string]json.RawMessage, error) {
	rows, err := tx.Query(ctx, "SELECT "+customFieldColumns+", tf.default_value FROM item_type_fields tf JOIN custom_fields f ON f.id=tf.field_id "+
		"WHERE tf.type_id=$1 AND tf.default_value IS NOT NULL AND f.archived_at IS NULL AND (f.space_id IS NULL OR f.space_id=ANY($2))", t.ID, spaces)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var f CustomField
		var raw json.RawMessage
		if err := rows.Scan(&f.ID, &f.SpaceID, &f.Name, &f.Type, &f.Options, &f.Position, &f.ArchivedAt, &f.Config, &raw); err != nil {
			return nil, err
		}
		if !defaultable(f, t.Base) {
			continue
		}
		if v, err := normalizeCustomFieldValue(f, raw); err == nil {
			out[f.ID] = v
		}
	}
	return out, rows.Err()
}
