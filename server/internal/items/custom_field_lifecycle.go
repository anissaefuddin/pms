package items

import (
	"context"
	"slices"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CustomFieldImpact counts what refers to a field, so an archive or restore can show it first. Nothing is blocked by it
// except a formula that reads the field (guardFormulaReferences), which keeps the existing archive rule.
type CustomFieldImpact struct {
	Items        int      `json:"items"`
	ItemTypes    int      `json:"item_types"`
	StatusGuards int      `json:"status_guards"`
	Views        int      `json:"views"`
	RequestForms int      `json:"request_forms"`
	Automations  int      `json:"automations"`
	Blueprints   int      `json:"blueprints"`
	Widgets      int      `json:"widgets"`
	Relations    int      `json:"relations"`
	Formulas     []string `json:"formulas"`
}

// CustomFieldImpactOf counts the references to a field inside its Space (or across the Workspace for a Workspace field).
// The JSON columns are matched by the field's UUID text: a stored reference always contains it, so the count cannot miss one.
// shortcut: text match over JSON, may count an unrelated occurrence of the UUID, which only over-warns; upgrade to typed reference tables when the JSON shapes stabilise.
func CustomFieldImpactOf(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID, fieldID string) (CustomFieldImpact, error) {
	parsed, err := uuid.Parse(fieldID)
	if err != nil {
		return CustomFieldImpact{}, ErrNotFound
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return CustomFieldImpact{}, err
	}
	defer tx.Rollback(ctx)
	space, err := fieldSpace(ctx, tx, actor, spaceID, true)
	if err != nil {
		return CustomFieldImpact{}, err
	}
	field, err := scanCustomField(tx.QueryRow(ctx, "SELECT "+customFieldColumns+
		" FROM custom_fields WHERE id=$1 AND space_id IS NOT DISTINCT FROM NULLIF($2,'')::uuid", parsed, space))
	if err != nil {
		return CustomFieldImpact{}, ErrNotFound
	}
	var out CustomFieldImpact
	ref := "%" + parsed.String() + "%"
	scope := nullableUUID(space)
	err = tx.QueryRow(ctx, `SELECT
  (SELECT count(*) FROM items WHERE fields ? $1::text AND deleted_at IS NULL),
  (SELECT count(*) FROM item_type_fields WHERE field_id=$2::uuid),
  (SELECT count(*) FROM statuses s WHERE $1::text = ANY(s.required_fields)),
  (SELECT count(*) FROM views WHERE config::text LIKE $4::text AND ($3::uuid IS NULL OR scope_space_id=$3::uuid)),
  (SELECT count(*) FROM request_forms WHERE definition::text LIKE $4::text AND ($3::uuid IS NULL OR space_id=$3::uuid)),
  (SELECT count(DISTINCT r.id) FROM automation_rules r JOIN automation_rule_versions v ON v.rule_id=r.id WHERE v.definition::text LIKE $4::text AND ($3::uuid IS NULL OR r.space_id=$3::uuid)),
  (SELECT count(*) FROM blueprints WHERE tree::text LIKE $4::text AND ($3::uuid IS NULL OR space_id=$3::uuid)),
  (SELECT count(*) FROM widgets w JOIN dashboards d ON d.id=w.dashboard_id WHERE w.config::text LIKE $4::text AND ($3::uuid IS NULL OR d.space_id=$3::uuid)),
  (SELECT count(*) FROM item_relations WHERE field_id=$2::uuid)`,
		parsed.String(), parsed, scope, ref).Scan(&out.Items, &out.ItemTypes, &out.StatusGuards, &out.Views,
		&out.RequestForms, &out.Automations, &out.Blueprints, &out.Widgets, &out.Relations)
	if err != nil {
		return CustomFieldImpact{}, err
	}
	out.Formulas = []string{}
	if space != "" { // formulas read only fields of their own Space, so a Workspace field has none
		_, formulas, err := loadSpaceFormulas(ctx, tx, space)
		if err != nil {
			return CustomFieldImpact{}, err
		}
		for _, f := range formulas {
			if f.ID != field.ID && slices.Contains(f.Refs, strings.ToLower(field.Name)) {
				out.Formulas = append(out.Formulas, f.Name)
			}
		}
	}
	return out, nil
}

// RestoreCustomField makes an archived field active again. The name must still be free among active fields, and a formula
// is checked again against the current fields before it counts as active.
func RestoreCustomField(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID, fieldID string) (CustomField, error) {
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
		" FROM custom_fields WHERE id=$1 AND space_id IS NOT DISTINCT FROM NULLIF($2,'')::uuid FOR UPDATE", parsed, space))
	if err != nil {
		return CustomField{}, ErrNotFound
	}
	if field.ArchivedAt == nil {
		return field, nil
	}
	var clash bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM custom_fields WHERE id<>$1 AND archived_at IS NULL
AND space_id IS NOT DISTINCT FROM NULLIF($2,'')::uuid AND lower(name)=lower($3))`, parsed, space, field.Name).Scan(&clash); err != nil {
		return CustomField{}, err
	}
	if clash {
		return CustomField{}, Invalid("an active custom field already uses this name; rename it before restoring")
	}
	if _, err := tx.Exec(ctx, "UPDATE custom_fields SET archived_at=NULL WHERE id=$1", parsed); err != nil {
		return CustomField{}, writeError(err)
	}
	field.ArchivedAt = nil
	if field.Type == "formula" {
		if err := checkFormulaDefinition(ctx, tx, space, field); err != nil {
			return CustomField{}, err
		}
		if err := recomputeSpaceFormulas(ctx, tx, space); err != nil {
			return CustomField{}, writeError(err)
		}
	}
	return field, commitCustomFields(ctx, tx)
}

// nullableUUID is "" for the Workspace, so SQL can test the Space with IS NULL.
func nullableUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}
