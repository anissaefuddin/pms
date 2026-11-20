package projectexcel

import (
	"context"
	"fmt"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TemplateVersion is in every template's file name, so a file made from an older layout can be recognised.
const TemplateVersion = 1

// TemplateField is an active custom field the template offers as a column.
type TemplateField struct{ ID, Name string }

// TemplateRows returns the header and one example row for a template: basic (one level), hierarchy (adds Parent Key), or
// advanced (adds status, workflow, assignee, dependencies and the Space's custom fields). Every header is importable as written.
func TemplateRows(kind string, fields []TemplateField) ([][]string, error) {
	basic := []string{"External Key", "type", "title", "start_date", "Finish Date", "duration_days", "is_milestone", "type_key"}
	var header []string
	switch kind {
	case "basic":
		header = basic
	case "hierarchy":
		header = append([]string{"External Key", "Parent Key"}, basic[1:]...)
	case "advanced":
		header = append([]string{"External Key", "Parent Key"}, basic[1:]...)
		header = append(header, "Status", "Workflow", "Assignee Email", "Depends On")
		for _, field := range fields {
			header = append(header, "field:"+field.ID)
		}
	default:
		return nil, fmt.Errorf("unknown template %q", kind)
	}
	example := make([]string, len(header))
	for i, name := range header {
		switch {
		case strings.HasPrefix(name, "field:"):
			example[i] = ""
		case name == "External Key":
			example[i] = "T001"
		case name == "title":
			example[i] = "Example task"
		case name == "type":
			example[i] = "task"
		}
	}
	return [][]string{header, example}, nil
}

// TemplateFields lists the active custom fields of a Space that the actor may read: a template offers only what the import accepts.
func TemplateFields(ctx context.Context, tx pgx.Tx, actor policy.Actor, space uuid.UUID) ([]TemplateField, error) {
	hidden, err := policy.HiddenCustomFieldIDs(ctx, tx, actor, policy.FieldGrantView)
	if err != nil {
		return nil, err
	}
	if hidden == nil {
		hidden = []string{}
	}
	rows, err := tx.Query(ctx, `SELECT id::text, name FROM custom_fields
WHERE (space_id=$1 OR space_id IS NULL) AND archived_at IS NULL AND NOT (id::text = ANY($2::text[])) ORDER BY name, id`, space, hidden)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[TemplateField])
}
