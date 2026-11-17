package projectexcel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

var ErrTooLarge = errors.New("project exceeds the Excel export limit")

var Headers = []string{"id", "parent_id", "type", "title", "start_date", "due_date", "duration_days", "is_milestone", "type_key"}

type ExportArgs struct {
	ID uuid.UUID `json:"id"`
}

func (ExportArgs) Kind() string { return "project_excel_export" }

type ExportWorker struct {
	river.WorkerDefaults[ExportArgs]
	Pool *pgxpool.Pool
}

func CheckScope(ctx context.Context, tx pgx.Tx, actor policy.Actor, project, space uuid.UUID) error {
	var exists bool
	err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM items WHERE id=$1 AND type='project' AND deleted_at IS NULL)", project).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return pgx.ErrNoRows
	}
	caps, err := policy.ItemCapsInSpace(ctx, tx, actor, project.String(), space.String())
	if err != nil {
		return err
	}
	if !caps.Has(policy.View) {
		return pgx.ErrNoRows
	}
	return nil
}

const tree = `
WITH RECURSIVE tree(id) AS (
	SELECT $1::uuid
	UNION
	SELECT l.item_id FROM tree t JOIN item_locations l ON l.parent_item_id=t.id
	JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
	WHERE l.space_id=$2 AND l.deleted_root_id IS NULL
)`

func CanDownload(ctx context.Context, tx pgx.Tx, actor policy.Actor, project, space uuid.UUID, ids []uuid.UUID, fields bool) (bool, error) {
	if fields {
		access, _, err := policy.SpaceAccessFor(ctx, tx, actor, space.String())
		if err != nil || !access.Caps.Has(policy.View) {
			return false, err
		}
	}
	var allowed bool
	err := tx.QueryRow(ctx, tree+` SELECT NOT EXISTS (
	SELECT 1 FROM unnest($3::uuid[]) exported(id)
	WHERE NOT EXISTS(SELECT 1 FROM tree WHERE tree.id=exported.id)
)`, project, space, ids).Scan(&allowed)
	return allowed, err
}

func (w *ExportWorker) Work(ctx context.Context, job *river.Job[ExportArgs]) error {
	err := w.export(ctx, job.Args.ID)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrTooLarge) || errors.Is(err, pgx.ErrNoRows) || job.Attempt >= job.MaxAttempts {
		message := "The workbook could not be generated."
		if errors.Is(err, ErrTooLarge) {
			message = "Project exceeds the Excel export limit."
		}
		_, updateErr := w.Pool.Exec(ctx, "UPDATE project_excel_exports SET status='failed',error=$2 WHERE id=$1 AND status='pending'", job.Args.ID, message)
		return updateErr
	}
	return err
}

func (w *ExportWorker) export(ctx context.Context, id uuid.UUID) error {
	tx, err := w.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var project, space uuid.UUID
	var actor policy.Actor
	if err := tx.QueryRow(ctx, `SELECT e.project_id,e.space_id,u.id::text,u.user_type::text
FROM project_excel_exports e JOIN users u ON u.id=e.user_id AND u.status='active'
WHERE e.id=$1 AND e.status='pending' AND e.expires_at>now()`, id).Scan(&project, &space, &actor.ID, &actor.Type); err != nil {
		return err
	}
	if err := CheckScope(ctx, tx, actor, project, space); err != nil {
		return err
	}
	data, ids, err := exportRows(ctx, tx, actor, project, space)
	if err != nil {
		return err
	}
	workbook, err := Write(data)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTooLarge, err)
	}
	if _, err = tx.Exec(ctx, `UPDATE project_excel_exports SET status='ready',workbook=$2,item_ids=$3,includes_fields=$4
WHERE id=$1 AND status='pending' AND expires_at>now()`, id, workbook, ids, len(data[0]) > len(Headers)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func exportRows(ctx context.Context, tx pgx.Tx, actor policy.Actor, project, space uuid.UUID) ([][]string, []uuid.UUID, error) {
	spaceAccess, _, err := policy.SpaceAccessFor(ctx, tx, actor, space.String())
	if err != nil {
		return nil, nil, err
	}
	// Restricted fields the requester cannot read are left out of the export, including formulas that read them.
	hidden, err := policy.HiddenCustomFieldIDs(ctx, tx, actor, policy.FieldGrantView)
	if err != nil {
		return nil, nil, err
	}
	if hidden == nil {
		hidden = []string{}
	}
	rows, err := tx.Query(ctx, tree+`
SELECT i.id,i.type::text,i.title,COALESCE(to_char(i.start_date,'YYYY-MM-DD'),''),
COALESCE(to_char(i.due_date,'YYYY-MM-DD'),''),COALESCE(i.duration_days::text,''),i.is_milestone::text,ty.key,
COALESCE((SELECT jsonb_object_agg(f.id,i.fields->f.id::text) FROM custom_fields f
 WHERE $3 AND (f.space_id=$2 OR f.space_id IS NULL) AND i.fields ? f.id::text
 AND custom_field_applies(i.id, f.id, $2) AND NOT (f.id::text = ANY($5::text[]))
 AND NOT (f.type='formula' AND EXISTS(SELECT 1 FROM custom_fields h WHERE h.id::text = ANY($5::text[])
   AND h.space_id IS NOT DISTINCT FROM f.space_id AND position(lower(h.name) IN lower(f.config->>'expression'))>0))),'{}')
FROM tree t JOIN items i ON i.id=t.id JOIN item_type_defs ty ON ty.id=i.type_id LIMIT $4`, project, space, spaceAccess.Caps.Has(policy.View), MaxRows, hidden)
	if err != nil {
		return nil, nil, err
	}
	type node struct {
		id     uuid.UUID
		values []string
		fields map[string]json.RawMessage
	}
	nodes := map[uuid.UUID]node{}
	headers := map[string]bool{}
	bytes := 0
	for rows.Next() {
		var n node
		n.values = make([]string, len(Headers))
		if err := rows.Scan(&n.id, &n.values[2], &n.values[3], &n.values[4], &n.values[5], &n.values[6], &n.values[7], &n.values[8], &n.fields); err != nil {
			rows.Close()
			return nil, nil, err
		}
		n.values[0] = n.id.String()
		for _, cell := range n.values {
			bytes += len(cell)
		}
		for field, value := range n.fields {
			headers[field] = true
			bytes += len(value)
		}
		if bytes > 8<<20 || len(headers)+len(Headers) > MaxColumns {
			rows.Close()
			return nil, nil, ErrTooLarge
		}
		nodes[n.id] = n
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(nodes) >= MaxRows || len(headers)+len(Headers) > MaxColumns {
		return nil, nil, ErrTooLarge
	}
	rows, err = tx.Query(ctx, tree+`
SELECT l.parent_item_id,l.item_id FROM item_locations l JOIN tree p ON p.id=l.parent_item_id JOIN tree c ON c.id=l.item_id
WHERE l.space_id=$2 AND l.deleted_root_id IS NULL ORDER BY l.parent_item_id,l.rank COLLATE "C",l.item_id LIMIT 10001`, project, space)
	if err != nil {
		return nil, nil, err
	}
	children := map[uuid.UUID][]uuid.UUID{}
	parents := map[uuid.UUID][]string{}
	count := 0
	for rows.Next() {
		var parent, child uuid.UUID
		if err := rows.Scan(&parent, &child); err != nil {
			rows.Close()
			return nil, nil, err
		}
		children[parent] = append(children[parent], child)
		parents[child] = append(parents[child], parent.String())
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if count > 10000 {
		return nil, nil, ErrTooLarge
	}
	var custom []string
	for field := range headers {
		custom = append(custom, field)
	}
	sort.Strings(custom)
	header := append([]string{}, Headers...)
	for _, field := range custom {
		header = append(header, "field:"+field)
	}
	data := [][]string{header}
	var ids []uuid.UUID
	seen := map[uuid.UUID]bool{}
	var visit func(uuid.UUID, uuid.UUID)
	visit = func(id, parent uuid.UUID) {
		if seen[id] {
			return
		}
		seen[id] = true
		n := nodes[id]
		if parent != uuid.Nil {
			n.values[1] = parent.String()
			if len(parents[id]) > 1 {
				encoded, _ := json.Marshal(parents[id])
				n.values[1] = string(encoded)
			}
		}
		for _, field := range custom {
			n.values = append(n.values, string(n.fields[field]))
		}
		data = append(data, n.values)
		ids = append(ids, id)
		for _, child := range children[id] {
			visit(child, id)
		}
	}
	visit(project, uuid.Nil)
	return data, ids, nil
}
