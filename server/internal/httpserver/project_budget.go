package httpserver

import (
	"context"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

// projectBudget compares one budget field of a project with its cost (DEF-017). Amounts are minor units (hundredths), the same
// convention as rates. Variance and utilization exist only for a complete cost; utilization also needs a budget above zero.
type projectBudget struct {
	FieldID      string   `json:"field_id"`
	Name         string   `json:"name"`
	Currency     string   `json:"currency"`
	PlannedMinor *int64   `json:"planned_minor"`
	Variance     *int64   `json:"variance_minor"` // planned - actual: negative is over budget
	Utilization  *float64 `json:"utilization_percent"`
}

// projectBudgets lists the budget fields the caller may read on this project.
func projectBudgets(ctx context.Context, tx pgx.Tx, actor policy.Actor, project string, summary map[string]any) ([]projectBudget, error) {
	hidden, err := items.HiddenFieldIDs(ctx, tx, actor, items.GrantView)
	if err != nil {
		return nil, err
	}
	var cost *int64
	if value, ok := summary["actual_cost_minor"].(int64); ok {
		cost = &value
	}
	rows, err := tx.Query(ctx, `SELECT f.id::text, f.name, COALESCE(f.config->>'currency',''), round((i.fields->>f.id::text)::numeric*100)::bigint,
 CASE WHEN $3::bigint IS NOT NULL THEN round((i.fields->>f.id::text)::numeric*100)::bigint-$3::bigint END,
 CASE WHEN $3::bigint IS NOT NULL AND (i.fields->>f.id::text)::numeric>0 THEN round($3::numeric/((i.fields->>f.id::text)::numeric*100)*100,1)::float8 END
FROM items i JOIN custom_fields f ON f.archived_at IS NULL AND (f.config->>'project_budget')::boolean
WHERE i.id=$1::uuid AND NOT (f.id::text = ANY($2::text[]))
  AND EXISTS(SELECT 1 FROM item_locations l WHERE l.item_id=i.id AND l.deleted_root_id IS NULL AND custom_field_applies(i.id, f.id, l.space_id))
ORDER BY f.position, f.id`, project, hidden, cost)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[projectBudget])
	if list == nil {
		list = []projectBudget{}
	}
	return list, err
}
