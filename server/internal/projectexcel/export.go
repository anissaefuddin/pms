package projectexcel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

var ErrTooLarge = errors.New("project exceeds the Excel export limit")

// Headers are the canonical LENS columns, in the order the exporter writes them.
var Headers = []string{"id", "parent_id", "type", "title", "start_date", "due_date", "duration_days", "is_milestone", "type_key"}

// ExportHeaders are the public names written to the workbook; Canonical maps them back, so older files still import.
var ExportHeaders = []string{"External Key", "Parent Key", "type", "title", "start_date", "Finish Date", "duration_days", "is_milestone", "type_key",
	"Status", "Workflow", "Depends On"}

type ExportArgs struct {
	ID     uuid.UUID    `json:"id"`
	Filter ExportFilter `json:"filter,omitempty"`
}

func (ExportArgs) Kind() string { return "project_excel_export" }

// ExportFilter is the Table View state a tree export follows. Search matches titles, StatusGroups and Assignee narrow the
// tasks, and Folders and projects that hold a match stay so the parent structure is kept. Sort orders siblings by
// title, start, or finish (a leading "-" reverses). Columns keeps only the named columns; External Key and Parent Key
// always stay so the workbook can be imported again. An empty filter exports the whole tree in rank order.
type ExportFilter struct {
	// Scoped is set when the request had a filter. Matches is the snapshot the Table View engine returned for it (see
	// httpserver exportMatches), so the export and the table agree on which items are in.
	Scoped  bool        `json:"scoped,omitempty"`
	Matches []uuid.UUID `json:"matches,omitempty"`
	Sort    string      `json:"sort,omitempty"`
	Columns []string    `json:"columns,omitempty"`
}

func (f ExportFilter) active() bool { return f.Scoped }

// columnIndex maps the names a client may ask for to positions in the exported row (see ExportHeaders).
var columnIndex = map[string]int{"id": 0, "parent_id": 1, "type": 2, "title": 3, "start_date": 4, "due_date": 5, "duration_days": 6,
	"is_milestone": 7, "type_key": 8, "status": 9, "workflow": 10, "depends_on": 11}

// ValidColumn reports whether a filter may name a column: a LENS column key, or "field:<id>" for a custom field.
func ValidColumn(name string) bool {
	_, ok := columnIndex[name]
	return ok || strings.HasPrefix(name, "field:")
}

type ExportWorker struct {
	river.WorkerDefaults[ExportArgs]
	Pool *pgxpool.Pool
}

func CheckScope(ctx context.Context, tx pgx.Tx, actor policy.Actor, project, space uuid.UUID) error {
	var exists bool
	err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM items WHERE id=$1 AND type IN ('project','folder') AND deleted_at IS NULL)", project).Scan(&exists)
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

// tree is every item under the roots in $1 that sits in the Space $2. A project export has one root; a Space export has
// every root placement in the Space.
const tree = `
WITH RECURSIVE tree(id) AS (
	SELECT unnest($1::uuid[])
	UNION
	SELECT l.item_id FROM tree t JOIN item_locations l ON l.parent_item_id=t.id
	JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
	WHERE l.space_id=$2 AND l.deleted_root_id IS NULL
)`

// Roots lists the top of an export: the project itself, or every root placement in a Space.
func Roots(ctx context.Context, tx pgx.Tx, scope string, project *uuid.UUID, space uuid.UUID) ([]uuid.UUID, error) {
	if scope == ScopeProject {
		if project == nil {
			return nil, pgx.ErrNoRows
		}
		return []uuid.UUID{*project}, nil
	}
	rows, err := tx.Query(ctx, `SELECT l.item_id FROM item_locations l JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
WHERE l.space_id=$1 AND l.parent_item_id IS NULL AND l.deleted_root_id IS NULL ORDER BY l.rank COLLATE "C", l.item_id`, space)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

// Export scopes: a project tree, or the whole Space (the roots the caller can see, see httpserver exportMatches).
const (
	ScopeProject = "project"
	ScopeSpace   = "space"
)

// CanDownload rechecks, at download, that every exported item is still inside the scope and that the caller can still view
// the Space. A Space export also needs the caller to hold the view that produced it.
func CanDownload(ctx context.Context, tx pgx.Tx, actor policy.Actor, scope string, project *uuid.UUID, space uuid.UUID, ids []uuid.UUID, fields bool) (bool, error) {
	access, _, err := policy.SpaceAccessFor(ctx, tx, actor, space.String())
	if err != nil || !access.Caps.Has(policy.View) {
		return false, err
	}
	roots, err := Roots(ctx, tx, scope, project, space)
	if err != nil {
		return false, err
	}
	var allowed bool
	err = tx.QueryRow(ctx, tree+` SELECT NOT EXISTS (
	SELECT 1 FROM unnest($3::uuid[]) exported(id)
	WHERE NOT EXISTS(SELECT 1 FROM tree WHERE tree.id=exported.id)
)`, roots, space, ids).Scan(&allowed)
	if err != nil || !allowed {
		return false, err
	}
	// Every exported item must still be viewable by the caller: an item share removed after the export ends the download.
	strs := make([]string, 0, len(ids))
	for _, id := range ids {
		strs = append(strs, id.String())
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, strs, nil)
	if err != nil {
		return false, err
	}
	for _, id := range strs {
		if !caps[id].Has(policy.View) {
			return false, nil
		}
	}
	return true, nil
}

func (w *ExportWorker) Work(ctx context.Context, job *river.Job[ExportArgs]) error {
	err := w.export(ctx, job.Args.ID, job.Args.Filter)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrTooLarge) || errors.Is(err, pgx.ErrNoRows) || job.Attempt >= job.MaxAttempts {
		message := "The workbook could not be generated."
		if errors.Is(err, ErrTooLarge) {
			message = "Project exceeds the Excel export limit: " + strings.TrimPrefix(err.Error(), ErrTooLarge.Error()+": ")
		}
		_, updateErr := w.Pool.Exec(ctx, "UPDATE project_excel_exports SET status='failed',error=$2 WHERE id=$1 AND status='pending'", job.Args.ID, message)
		return updateErr
	}
	return err
}

func (w *ExportWorker) export(ctx context.Context, id uuid.UUID, filter ExportFilter) error {
	tx, err := w.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var scope string
	var project *uuid.UUID
	var space uuid.UUID
	var actor policy.Actor
	if err := tx.QueryRow(ctx, `SELECT e.scope,e.project_id,e.space_id,u.id::text,u.user_type::text
FROM project_excel_exports e JOIN users u ON u.id=e.user_id AND u.status='active'
WHERE e.id=$1 AND e.status='pending' AND e.expires_at>now()`, id).Scan(&scope, &project, &space, &actor.ID, &actor.Type); err != nil {
		return err
	}
	if scope == ScopeProject {
		if project == nil {
			return pgx.ErrNoRows
		}
		if err := CheckScope(ctx, tx, actor, *project, space); err != nil {
			return err
		}
	} else if access, _, err := policy.SpaceAccessFor(ctx, tx, actor, space.String()); err != nil || !access.Caps.Has(policy.View) {
		return pgx.ErrNoRows
	}
	roots, err := Roots(ctx, tx, scope, project, space)
	if err != nil {
		return err
	}
	progress := func(done, total int) {
		// Autocommit, outside the export transaction, so a poll sees the count while the workbook is still being built.
		_, _ = w.Pool.Exec(ctx, "UPDATE project_excel_exports SET total_items=$2, processed_items=$3 WHERE id=$1 AND status='pending'", id, total, done)
		if ProgressObserver != nil {
			ProgressObserver(id, done, total)
		}
	}
	data, ids, err := exportRows(ctx, tx, actor, scope, roots, space, filter, progress)
	if err != nil {
		return err
	}
	workbook, err := Write(data)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTooLarge, err)
	}
	// The result is written outside the snapshot transaction: progress updates touch the same row while the snapshot is open,
	// and a snapshot that writes a row changed meanwhile fails with a serialization error.
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	written := len(data) - 1
	_, err = w.Pool.Exec(ctx, `UPDATE project_excel_exports SET status='ready',workbook=$2,item_ids=$3,includes_fields=$4,total_items=$5,processed_items=$5
WHERE id=$1 AND status='pending' AND expires_at>now()`, id, workbook, ids, len(data[0]) > len(ExportHeaders), written)
	return err
}

// ProgressObserver, when set, is called after each progress report is stored. It exists so tests can hold a job in the
// middle of its walk and read the counts a poll would see. Production leaves it nil.
var ProgressObserver func(exportID uuid.UUID, done, total int)

// exportProgressEvery is how many written rows pass between progress reports.
const exportProgressEvery = 50

func exportRows(ctx context.Context, tx pgx.Tx, actor policy.Actor, scope string, roots []uuid.UUID, space uuid.UUID, filter ExportFilter, progress func(done, total int)) ([][]string, []uuid.UUID, error) {
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
COALESCE(st.name,''),COALESCE(wf.name,''),
COALESCE((SELECT jsonb_object_agg(f.id,i.fields->f.id::text) FROM custom_fields f
 WHERE $3 AND (f.space_id=$2 OR f.space_id IS NULL) AND i.fields ? f.id::text
 AND custom_field_applies(i.id, f.id, $2) AND NOT (f.id::text = ANY($5::text[]))
 AND NOT (f.type='formula' AND EXISTS(SELECT 1 FROM custom_fields h WHERE h.id::text = ANY($5::text[])
   AND h.space_id IS NOT DISTINCT FROM f.space_id AND position(lower(h.name) IN lower(f.config->>'expression'))>0))),'{}')
FROM tree t JOIN items i ON i.id=t.id JOIN item_type_defs ty ON ty.id=i.type_id
LEFT JOIN statuses st ON st.id=i.status_id LEFT JOIN workflows wf ON wf.id=st.workflow_id LIMIT $4`, roots, space, spaceAccess.Caps.Has(policy.View), MaxRows, hidden)
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
		n.values = make([]string, len(ExportHeaders))
		if err := rows.Scan(&n.id, &n.values[2], &n.values[3], &n.values[4], &n.values[5], &n.values[6], &n.values[7], &n.values[8], &n.values[9], &n.values[10], &n.fields); err != nil {
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
	if len(nodes) >= MaxRows {
		return nil, nil, fmt.Errorf("%w: %d items, at most %d per export", ErrTooLarge, len(nodes), MaxRows-1)
	}
	if len(headers)+len(Headers) > MaxColumns {
		return nil, nil, fmt.Errorf("%w: %d custom fields, at most %d columns", ErrTooLarge, len(headers), MaxColumns)
	}
	// Dependencies are written only between items in this file, as Finish-to-Start, keyed by External Key (the item ID).
	var keys []uuid.UUID
	for id := range nodes {
		keys = append(keys, id)
	}
	deps, err := tx.Query(ctx, `SELECT d.successor_id, d.predecessor_id FROM item_dependencies d
WHERE d.type='FS' AND d.predecessor_id=ANY($1::uuid[]) AND d.successor_id=ANY($1::uuid[]) ORDER BY d.successor_id, d.predecessor_id`, keys)
	if err != nil {
		return nil, nil, err
	}
	for deps.Next() {
		var successor, predecessor uuid.UUID
		if err := deps.Scan(&successor, &predecessor); err != nil {
			deps.Close()
			return nil, nil, err
		}
		if n, ok := nodes[successor]; ok {
			if n.values[11] != "" {
				n.values[11] += ";"
			}
			n.values[11] += predecessor.String() + ":FS"
		}
	}
	if err := deps.Err(); err != nil {
		return nil, nil, err
	}
	rows, err = tx.Query(ctx, tree+`
SELECT l.parent_item_id,l.item_id FROM item_locations l JOIN tree p ON p.id=l.parent_item_id JOIN tree c ON c.id=l.item_id
WHERE l.space_id=$2 AND l.deleted_root_id IS NULL ORDER BY l.parent_item_id,l.rank COLLATE "C",l.item_id LIMIT 10001`, roots, space)
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
	header := append([]string{}, ExportHeaders...)
	for _, field := range custom {
		header = append(header, "field:"+field)
	}
	data := [][]string{header}
	if filter.Sort != "" {
		values := make(map[uuid.UUID][]string, len(nodes))
		for id, n := range nodes {
			values[id] = n.values
		}
		sortSiblings(children, values, filter.Sort)
	}
	var kept map[uuid.UUID]bool
	if filter.active() || scope == ScopeSpace {
		// A Space export keeps only what the caller can see (the snapshot), with the parents that hold it.
		kept = keepWithParents(roots, children, filterMatches(filter), scope == ScopeProject)
	}
	if progress != nil {
		progress(0, countReachable(roots, children, kept))
	}
	var ids []uuid.UUID
	seen := map[uuid.UUID]bool{}
	var visit func(uuid.UUID, uuid.UUID)
	visit = func(id, parent uuid.UUID) {
		if seen[id] || (kept != nil && !kept[id]) {
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
			n.values = append(n.values, plainValue(n.fields[field]))
		}
		data = append(data, n.values)
		ids = append(ids, id)
		if written := len(data) - 1; progress != nil && written%exportProgressEvery == 0 {
			progress(written, countReachable(roots, children, kept))
		}
		for _, child := range children[id] {
			visit(child, id)
		}
	}
	for _, root := range roots {
		visit(root, uuid.Nil)
	}
	if len(filter.Columns) > 0 {
		data = selectColumns(data, filter.Columns)
	}
	return data, ids, nil
}

// Readable reports whether the actor may still read every custom field column in a stored workbook. It runs at download,
// so a grant revoked after the export was made takes effect at once. A formula column is refused too when its expression
// names a field the actor cannot read, the same rule the exporter applies.
func Readable(ctx context.Context, tx pgx.Tx, actor policy.Actor, file []byte) (bool, error) {
	rows, err := Read(file)
	if err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return true, nil
	}
	var ids []string
	for _, header := range rows[0] {
		if id, ok := strings.CutPrefix(header, "field:"); ok {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return true, nil
	}
	hidden, err := policy.HiddenCustomFieldIDs(ctx, tx, actor, policy.FieldGrantView)
	if err != nil {
		return false, err
	}
	if hidden == nil {
		hidden = []string{}
	}
	var leaked bool
	err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM custom_fields f WHERE f.id::text = ANY($1::text[]) AND (
 f.id::text = ANY($2::text[])
 OR (f.type='formula' AND EXISTS(SELECT 1 FROM custom_fields h WHERE h.id::text = ANY($2::text[])
   AND h.space_id IS NOT DISTINCT FROM f.space_id AND position(lower(h.name) IN lower(f.config->>'expression'))>0))))`, ids, hidden).Scan(&leaked)
	return !leaked, err
}

// plainValue shows a scalar custom value as text (a JSON string loses its quotes); lists and objects stay JSON, which keeps
// people fields as IDs so a re-import still matches them.
func plainValue(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	return string(raw)
}

// filterMatches returns the snapshot of items the filter keeps; the tree walk then keeps their parents.
func filterMatches(filter ExportFilter) map[uuid.UUID]bool {
	matches := make(map[uuid.UUID]bool, len(filter.Matches))
	for _, id := range filter.Matches {
		matches[id] = true
	}
	return matches
}

// sortSiblings orders each item's children by the Table View sort. Title, start, and finish; "-" reverses.
func sortSiblings(children map[uuid.UUID][]uuid.UUID, values map[uuid.UUID][]string, sortKey string) {
	desc := strings.HasPrefix(sortKey, "-")
	key := strings.TrimPrefix(sortKey, "-")
	index := map[string]int{"title": 3, "start_date": 4, "due_date": 5}[key]
	for parent, list := range children {
		sort.SliceStable(list, func(i, j int) bool {
			a, b := values[list[i]][index], values[list[j]][index]
			if desc {
				return a > b
			}
			return a < b
		})
		children[parent] = list
	}
}

// keepWithParents marks every item that is a match or has a kept descendant; the project is always kept.
func keepWithParents(roots []uuid.UUID, children map[uuid.UUID][]uuid.UUID, matches map[uuid.UUID]bool, forceRoots bool) map[uuid.UUID]bool {
	memo := map[uuid.UUID]bool{}
	open := map[uuid.UUID]bool{}
	var mark func(uuid.UUID) bool
	mark = func(id uuid.UUID) bool {
		if v, ok := memo[id]; ok {
			return v
		}
		if open[id] {
			return false
		}
		open[id] = true
		keep := matches[id]
		for _, child := range children[id] {
			if mark(child) {
				keep = true
			}
		}
		open[id] = false
		memo[id] = keep
		return keep
	}
	for _, root := range roots {
		mark(root)
		if forceRoots {
			memo[root] = true // a project stays even when nothing under it matches: the file keeps its structure
		}
	}
	return memo
}

// selectColumns keeps External Key and Parent Key, which the importer needs, plus the columns the client named.
func selectColumns(data [][]string, wanted []string) [][]string {
	names := map[string]bool{}
	for _, name := range wanted {
		names[name] = true
	}
	var keep []int
	for i, header := range data[0] {
		key := ""
		if i < len(exportKeys) {
			key = exportKeys[i]
		} else {
			key = header
		}
		if i < 2 || names[key] {
			keep = append(keep, i)
		}
	}
	out := make([][]string, len(data))
	for r, row := range data {
		picked := make([]string, 0, len(keep))
		for _, i := range keep {
			picked = append(picked, row[i])
		}
		out[r] = picked
	}
	return out
}

// exportKeys are the column keys for ExportHeaders, in order.
var exportKeys = []string{"id", "parent_id", "type", "title", "start_date", "due_date", "duration_days", "is_milestone", "type_key",
	"status", "workflow", "depends_on"}

// countReachable counts the items a walk from the roots will write: every item the kept set allows (all, without a kept set).
func countReachable(roots []uuid.UUID, children map[uuid.UUID][]uuid.UUID, kept map[uuid.UUID]bool) int {
	seen := map[uuid.UUID]bool{}
	var walk func(uuid.UUID) int
	walk = func(id uuid.UUID) int {
		if seen[id] || (kept != nil && !kept[id]) {
			return 0
		}
		seen[id] = true
		n := 1
		for _, child := range children[id] {
			n += walk(child)
		}
		return n
	}
	total := 0
	for _, root := range roots {
		total += walk(root)
	}
	return total
}
