package items

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/projectexcel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExcelIssue struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

type ExcelPreview struct {
	Columns []string     `json:"columns"`
	Rows    int          `json:"rows"`
	Errors  []ExcelIssue `json:"errors"`
	Sample  [][]string   `json:"sample"`
}

type excelNode struct {
	row      int
	id, kind string
	parents  []string
	fields   map[string]json.RawMessage
	custom   map[string]json.RawMessage
}

func ImportExcel(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, space uuid.UUID, rows [][]string, mapping map[string]string, digest string, commit bool, batch *notifications.Batch) (ExcelPreview, []uuid.UUID, error) {
	options := pgx.TxOptions{IsoLevel: pgx.Serializable}
	if !commit {
		options.IsoLevel = pgx.RepeatableRead
	}
	tx, err := pool.BeginTx(ctx, options)
	if err != nil {
		return ExcelPreview{}, nil, err
	}
	defer tx.Rollback(ctx)
	actorQuery := "SELECT user_type::text FROM users WHERE id=$1::uuid AND status='active'"
	if commit {
		actorQuery += " FOR SHARE"
	}
	if err := tx.QueryRow(ctx, actorQuery, actor.ID).Scan(&actor.Type); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = ErrForbidden
		}
		return ExcelPreview{}, nil, writeError(err)
	}
	if err := requireSpace(ctx, tx, actor, policy.Create, space); err != nil {
		return ExcelPreview{}, nil, err
	}
	preview, nodes, err := planExcel(ctx, tx, actor, space, rows, mapping)
	if err != nil || !commit || len(preview.Errors) > 0 {
		return preview, nil, writeError(err)
	}
	var old []uuid.UUID
	err = tx.QueryRow(ctx, `SELECT item_ids FROM project_excel_imports WHERE user_id=$1::uuid AND space_id=$2 AND digest=$3 AND expires_at>now()`, actor.ID, space, digest).Scan(&old)
	if err == nil {
		return preview, old, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return preview, nil, writeError(err)
	}
	// Reserve the receipt before creating anything. A concurrent retry either
	// replays the committed receipt or returns a serialization conflict.
	tag, err := tx.Exec(ctx, `INSERT INTO project_excel_imports(user_id,space_id,digest,item_ids)
VALUES($1::uuid,$2,$3,'{}') ON CONFLICT(user_id,space_id,digest) DO UPDATE
SET expires_at=excluded.expires_at,item_ids=excluded.item_ids WHERE project_excel_imports.expires_at<=now()`, actor.ID, space, digest)
	if err != nil {
		return preview, nil, writeError(err)
	}
	if tag.RowsAffected() != 1 {
		return preview, nil, ErrConflict
	}
	created := map[string]uuid.UUID{}
	ids := make([]uuid.UUID, 0, len(nodes))
	for _, node := range nodes {
		locations := []Location{{SpaceID: space}}
		if len(node.parents) > 0 {
			locations = locations[:0]
			for _, key := range node.parents {
				parent := created[key]
				locations = append(locations, Location{SpaceID: space, ParentID: &parent})
			}
		}
		node.fields["locations"] = marshalJSON(locations)
		var custom json.RawMessage
		if len(node.custom) > 0 {
			custom = marshalJSON(node.custom)
		}
		item, err := createWithCustomFields(ctx, tx, actor, node.fields, custom, batch)
		if err != nil {
			var field InvalidField
			var invalid Invalid
			if errors.As(err, &field) {
				preview.Errors = append(preview.Errors, ExcelIssue{node.row, field.Field, field.Message})
				return preview, nil, nil
			}
			if errors.As(err, &invalid) {
				preview.Errors = append(preview.Errors, ExcelIssue{node.row, "", invalid.Error()})
				return preview, nil, nil
			}
			if pgCode(err) == "23514" {
				preview.Errors = append(preview.Errors, ExcelIssue{node.row, "", "the row violates an item constraint"})
				return preview, nil, nil
			}
			return preview, nil, writeError(err)
		}
		var result struct {
			ID uuid.UUID `json:"id"`
		}
		if err := json.Unmarshal(item.Body, &result); err != nil {
			return preview, nil, err
		}
		created[node.id] = result.ID
		ids = append(ids, result.ID)
	}
	_, err = tx.Exec(ctx, "UPDATE project_excel_imports SET item_ids=$4 WHERE user_id=$1::uuid AND space_id=$2 AND digest=$3", actor.ID, space, digest, ids)
	if err == nil {
		err = tx.Commit(ctx)
	}
	return preview, ids, writeError(err)
}

func planExcel(ctx context.Context, tx pgx.Tx, actor policy.Actor, space uuid.UUID, rows [][]string, mapping map[string]string) (ExcelPreview, []excelNode, error) {
	p := ExcelPreview{Errors: []ExcelIssue{}, Columns: []string{}, Sample: [][]string{}}
	add := func(row int, col, msg string) {
		if len(p.Errors) < 500 {
			p.Errors = append(p.Errors, ExcelIssue{row, col, msg})
		}
	}
	if len(rows) == 0 {
		add(1, "", "workbook has no header")
		return p, nil, nil
	}
	p.Columns = rows[0]
	for _, row := range rows[1:min(len(rows), 6)] {
		sample := make([]string, len(row))
		for i, cell := range row {
			runes := []rune(cell)
			sample[i] = string(runes[:min(200, len(runes))])
		}
		p.Sample = append(p.Sample, sample)
	}
	fields, err := tx.Query(ctx, "SELECT "+customFieldColumns+" FROM custom_fields WHERE space_id=$1 AND archived_at IS NULL", space)
	if err != nil {
		return p, nil, err
	}
	defs, err := pgx.CollectRows(fields, pgx.RowToStructByPos[CustomField])
	if err != nil {
		return p, nil, err
	}
	typeRows, err := tx.Query(ctx, "SELECT key,lower(name),base::text FROM item_type_defs WHERE active")
	if err != nil {
		return p, nil, err
	}
	type typeRef struct{ key, base string }
	activeTypes := map[string]typeRef{} // by key and by lower-cased name
	for typeRows.Next() {
		var key, name, base string
		if err := typeRows.Scan(&key, &name, &base); err != nil {
			return p, nil, err
		}
		activeTypes[key], activeTypes[name] = typeRef{key, base}, typeRef{key, base}
	}
	if err := typeRows.Err(); err != nil {
		return p, nil, err
	}
	definitions := map[string]CustomField{}
	for _, field := range defs {
		definitions["field:"+field.ID] = field
	}
	headers, targets := map[string]bool{}, map[string]bool{}
	columns := make([]string, len(p.Columns))
	for i, source := range p.Columns {
		if strings.TrimSpace(source) == "" || headers[source] {
			add(1, source, "column names must be distinct and nonempty")
			continue
		}
		headers[source] = true
		target, ok := mapping[source]
		if !ok {
			target = strings.ToLower(strings.TrimSpace(source))
		}
		if target == "" {
			continue
		}
		if _, custom := definitions[target]; !custom && !slices.Contains(projectexcel.Headers, target) {
			add(1, source, "unknown column; map it to an available field or ignore it")
			continue
		}
		if targets[target] {
			add(1, source, "target is mapped more than once")
			continue
		}
		targets[target] = true
		columns[i] = target
	}
	for source := range mapping {
		if !headers[source] {
			add(1, source, "mapping source is not in the workbook")
		}
	}
	if !targets["title"] {
		add(1, "title", "map a column to title")
	}
	nodes := map[string]excelNode{}
	var order []string
	for index, raw := range rows[1:] {
		if !slices.ContainsFunc(raw, func(s string) bool { return strings.TrimSpace(s) != "" }) {
			continue
		}
		row := index + 2
		p.Rows++
		if len(raw) > len(columns) && slices.ContainsFunc(raw[len(columns):], func(s string) bool { return strings.TrimSpace(s) != "" }) {
			add(row, "", "data appears outside the named columns")
		}
		values := map[string]string{}
		for i, target := range columns {
			if target != "" && i < len(raw) && raw[i] != "" {
				values[target] = raw[i]
			}
		}
		id := strings.TrimSpace(values["id"])
		if id == "" {
			id = "@row:" + strconv.Itoa(row)
		}
		if _, exists := nodes[id]; exists {
			add(row, "id", "duplicate row ID")
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(values["type"]))
		if kind == "" {
			kind = "task"
		}
		var typeKey string // a custom item type, named by key or name in "type" or by key in "type_key"
		if !slices.Contains([]string{"project", "folder", "task"}, kind) {
			if t, ok := activeTypes[kind]; ok {
				typeKey, kind = t.key, t.base
			} else {
				add(row, "type", "unknown or inactive item type; enter project, folder, task or the key of an active item type")
			}
		}
		if given := strings.TrimSpace(values["type_key"]); given != "" {
			if t, ok := activeTypes[strings.ToLower(given)]; !ok {
				add(row, "type_key", "unknown or inactive item type")
			} else if values["type"] != "" && (typeKey != "" && typeKey != t.key || typeKey == "" && kind != t.base) {
				add(row, "type_key", "type_key does not match the type column")
			} else {
				typeKey, kind = t.key, t.base
			}
		}
		title := strings.TrimSpace(values["title"])
		if n := utf8.RuneCountInString(title); n < 1 || n > 500 || strings.ContainsRune(title, 0) {
			add(row, "title", "title must be 1 to 500 characters")
		}
		node := excelNode{row: row, id: id, kind: kind, fields: map[string]json.RawMessage{"type": marshalJSON(kind), "title": marshalJSON(title)}, custom: map[string]json.RawMessage{}}
		if typeKey != "" {
			node.fields["type_key"] = marshalJSON(typeKey)
		}
		if parent := strings.TrimSpace(values["parent_id"]); parent != "" {
			if strings.HasPrefix(parent, "[") {
				if json.Unmarshal([]byte(parent), &node.parents) != nil {
					add(row, "parent_id", "invalid list of parent IDs")
				}
			} else {
				node.parents = []string{parent}
			}
		}
		if len(node.parents) > 50 {
			add(row, "parent_id", "at most 50 parent IDs are allowed")
		}
		for _, key := range []string{"start_date", "due_date"} {
			if value := strings.TrimSpace(values[key]); value != "" {
				raw := marshalJSON(value)
				if _, err := dateField(raw, key); err != nil {
					add(row, key, "enter a date as YYYY-MM-DD")
				} else {
					node.fields[key] = raw
				}
			}
		}
		if values["start_date"] != "" && values["due_date"] != "" && strings.TrimSpace(values["start_date"]) > strings.TrimSpace(values["due_date"]) {
			add(row, "due_date", "finish date cannot precede start date")
		}
		if value := strings.TrimSpace(values["duration_days"]); value != "" {
			days, err := strconv.ParseInt(value, 10, 32)
			if err != nil || days < 0 {
				add(row, "duration_days", "enter a nonnegative whole number")
			} else {
				node.fields["duration_days"] = marshalJSON(days)
			}
		}
		if value := strings.TrimSpace(values["is_milestone"]); value != "" {
			switch strings.ToLower(value) {
			case "true", "1":
				node.fields["is_milestone"] = marshalJSON(true)
			case "false", "0":
				node.fields["is_milestone"] = marshalJSON(false)
			default:
				add(row, "is_milestone", "enter true or false")
			}
		}
		if string(node.fields["is_milestone"]) == "true" && node.fields["duration_days"] != nil && string(node.fields["duration_days"]) != "0" {
			add(row, "duration_days", "a milestone has zero duration")
		}
		for key, value := range values {
			field, ok := definitions[key]
			if !ok {
				continue
			}
			raw := json.RawMessage(value)
			if slices.Contains([]string{"text", "date", "single_select"}, field.Type) && !strings.HasPrefix(value, "\"") {
				raw = marshalJSON(value)
			}
			normalized, err := normalizeCustomFieldValue(field, raw)
			if err != nil {
				add(row, key, err.Error())
				continue
			}
			if field.Config.ProjectBudget != nil && *field.Config.ProjectBudget && kind != "project" {
				add(row, key, "budget is only available on projects")
				continue
			}
			if field.Type == "people" {
				if err := validateNewFieldPeople(ctx, tx, actor, field, nil, normalized); err != nil {
					var invalid InvalidField
					if !errors.As(err, &invalid) {
						return p, nil, err
					}
					add(row, key, invalid.Message)
					continue
				}
			}
			node.custom[field.ID] = normalized
		}
		nodes[id] = node
		order = append(order, id)
	}
	if p.Rows == 0 {
		add(2, "", "workbook has no item rows")
	}
	var result []excelNode
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if done[id] {
			return
		}
		n := nodes[id]
		if visiting[id] {
			add(n.row, "parent_id", "parent hierarchy contains a cycle")
			return
		}
		visiting[id] = true
		seen := map[string]bool{}
		for _, parent := range n.parents {
			parentNode, ok := nodes[parent]
			if !ok || seen[parent] {
				add(n.row, "parent_id", "parent must identify a distinct row in this workbook")
				continue
			}
			seen[parent] = true
			if parentNode.kind == "task" && n.kind != "task" {
				add(n.row, "parent_id", "a task can contain only tasks")
			}
			visit(parent)
		}
		delete(visiting, id)
		done[id] = true
		result = append(result, n)
	}
	for _, id := range order {
		visit(id)
	}
	return p, result, nil
}
