package policy

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Field access (CF-D03), shared by the item, list, export, and analytics paths. Rules are in items/field_access.go.
const (
	FieldGrantView = "view"
	FieldGrantEdit = "edit"
)

// HiddenCustomFieldIDs returns the restricted custom fields the actor may not read (need "view") or may not write (need "edit").
// Fields with no grant rows are never listed: they inherit item access.
func HiddenCustomFieldIDs(ctx context.Context, tx DB, actor Actor, need string) ([]string, error) {
	allowed := []string{FieldGrantView, FieldGrantEdit}
	if need == FieldGrantEdit {
		allowed = []string{FieldGrantEdit}
	}
	const match = `((g.subject_kind='user' AND g.subject_id=$1::uuid)
  OR (g.subject_kind='group' AND g.subject_id IN (SELECT group_id FROM group_members WHERE user_id=$1::uuid)))`
	rows, err := tx.Query(ctx, `SELECT f.id::text FROM custom_fields f
WHERE EXISTS(SELECT 1 FROM custom_field_grants g WHERE g.field_id=f.id)
AND (EXISTS(SELECT 1 FROM custom_field_grants g WHERE g.field_id=f.id AND g.level='none' AND `+match+`)
  OR NOT EXISTS(SELECT 1 FROM custom_field_grants g WHERE g.field_id=f.id AND g.level=ANY($2::text[]) AND `+match+`))`,
		actor.ID, allowed)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	return nonNilIDs(ids), nil
}

func nonNilIDs(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// actorRow is the (id, user_type) pair read from users. It is separate from Actor so Actor can grow fields
// (such as a form grant) without breaking positional scans.
type actorRow struct {
	ID   string
	Type UserType
}

// CollectActors reads (id, user_type) rows into Actors. Query must select exactly those two columns.
func CollectActors(rows pgx.Rows) ([]Actor, error) {
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[actorRow])
	if err != nil {
		return nil, err
	}
	out := make([]Actor, len(list))
	for i, r := range list {
		out[i] = Actor{ID: r.ID, Type: r.Type}
	}
	return out, nil
}
