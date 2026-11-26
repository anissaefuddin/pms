package items

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/events"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/projectexcel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExcelIssue struct {
	Sheet        string `json:"sheet"`
	Row          int    `json:"row"`
	Column       string `json:"column"`
	Reason       string `json:"reason"`
	Message      string `json:"message"`
	SuggestedFix string `json:"suggested_fix"`
}

type ExcelPreview struct {
	Columns      []string     `json:"columns"`
	Rows         int          `json:"rows"`
	Errors       []ExcelIssue `json:"errors"`
	Sample       [][]string   `json:"sample"`
	Dependencies int          `json:"dependencies"`
	// Rescheduled lists rows whose Start was derived from a predecessor; explicit dates are never changed.
	Rescheduled []ScheduledRow `json:"rescheduled,omitempty"`
	ReportID    string         `json:"report_id,omitempty"`
}

// ScheduledRow is one row whose dates the importer derived from its predecessors.
type ScheduledRow struct {
	Row    int    `json:"row"`
	Key    string `json:"key"`
	Start  string `json:"start"`
	Finish string `json:"finish,omitempty"`
}

type excelNode struct {
	row      int
	id, kind string
	parents  []string
	depends  []string // External Keys of the Finish-to-Start predecessors
	// startDay and finishDay are the row's dates on the working calendar, once known (explicit or derived).
	startDay, finishDay time.Time
	durationDays        int
	// Status is resolved after the rows are read, against the workflow each row inherits (see resolveWorkflows).
	typeKey, statusName, workflowName string
	fields                            map[string]json.RawMessage
	custom                            map[string]json.RawMessage
}

func ImportExcel(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, space uuid.UUID, destination uuid.UUID, sheet string, rows [][]string, mapping map[string]string, digest string, commit bool, batch *notifications.Batch) (ExcelPreview, []uuid.UUID, error) {
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
	preview, nodes, err := planExcel(ctx, tx, actor, space, destination, sheet, rows, mapping)
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
	// Creates run as a bulk import: events carry origin=bulk_import, so automation, webhooks and per-item notices stay off.
	ctx = events.WithBulkImport(ctx)
	created := map[string]uuid.UUID{}
	ids := make([]uuid.UUID, 0, len(nodes))
	for _, node := range nodes {
		locations := []Location{{SpaceID: space}}
		if destination != uuid.Nil && len(node.parents) == 0 {
			under := destination
			locations = []Location{{SpaceID: space, ParentID: &under}}
		}
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
				preview.Errors = append(preview.Errors, ExcelIssue{Sheet: sheet, Row: node.row, Column: field.Field, Message: field.Message, Reason: "value", SuggestedFix: "Correct the value in this column and import again."})
				return preview, nil, nil
			}
			if errors.As(err, &invalid) {
				preview.Errors = append(preview.Errors, ExcelIssue{Sheet: sheet, Row: node.row, Message: invalid.Error(), Reason: "row", SuggestedFix: "Correct the values in this row and import again."})
				return preview, nil, nil
			}
			if pgCode(err) == "23514" {
				preview.Errors = append(preview.Errors, ExcelIssue{Sheet: sheet, Row: node.row, Message: "the row violates an item constraint", Reason: "row", SuggestedFix: "Correct the values in this row and import again."})
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
	for _, node := range nodes {
		for _, pred := range node.depends {
			if _, err := tx.Exec(ctx, "INSERT INTO item_dependencies(predecessor_id,successor_id,type,lag) VALUES($1,$2,'FS',0)", created[pred], created[node.id]); err != nil {
				return preview, nil, writeError(err)
			}
		}
	}
	// One import report (kept after the receipt expires) and one summary notification for the importer, instead of one per row.
	report := map[string]any{"sheet": sheet, "destination": destination.String(), "created": len(ids),
		"dependencies": preview.Dependencies, "rescheduled": preview.Rescheduled}
	raw, err := json.Marshal(report)
	if err != nil {
		return preview, nil, err
	}
	var reportID uuid.UUID
	if err := tx.QueryRow(ctx, "INSERT INTO excel_import_reports(space_id,user_id,report) VALUES($1,$2::uuid,$3) RETURNING id", space, actor.ID, raw).Scan(&reportID); err != nil {
		return preview, nil, writeError(err)
	}
	preview.ReportID = reportID.String()
	if batch != nil {
		if err := batch.NotifyRef(ctx, tx, actor, actor.ID, "import_summary", fmt.Sprintf("Imported %d items. Report %s", len(ids), reportID), "excel_import_report", reportID.String()); err != nil {
			return preview, nil, writeError(err)
		}
	}
	_, err = tx.Exec(ctx, "UPDATE project_excel_imports SET item_ids=$4 WHERE user_id=$1::uuid AND space_id=$2 AND digest=$3", actor.ID, space, digest, ids)
	if err == nil {
		err = tx.Commit(ctx)
	}
	return preview, ids, writeError(err)
}

func planExcel(ctx context.Context, tx pgx.Tx, actor policy.Actor, space, destination uuid.UUID, sheet string, rows [][]string, mapping map[string]string) (ExcelPreview, []excelNode, error) {
	p := ExcelPreview{Errors: []ExcelIssue{}, Columns: []string{}, Sample: [][]string{}}
	add := func(row int, col, msg string) {
		if len(p.Errors) < 500 {
			p.Errors = append(p.Errors, ExcelIssue{Sheet: sheet, Row: row, Column: col, Message: msg, Reason: issueReason(row, col), SuggestedFix: issueFix(col, msg)})
		}
	}
	if len(rows) == 0 {
		add(1, "", "workbook has no header")
		return p, nil, nil
	}
	target := Location{SpaceID: space}
	if destination != uuid.Nil {
		target.ParentID = &destination
	}
	if err := lockGrants(ctx, tx, actor.ID); err != nil {
		return p, nil, err
	}
	canAssign, err := CanAssignOthers(ctx, tx, actor, []Location{target}) // W3, as on a direct create
	if err != nil {
		return p, nil, err
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
	cal, err := loadCalendar(ctx, tx)
	if err != nil {
		return p, nil, err
	}
	// Restricted fields the caller may not edit: a column that writes one is refused at preview (IMP-T11).
	readOnly, err := policy.HiddenCustomFieldIDs(ctx, tx, actor, policy.FieldGrantEdit)
	if err != nil {
		return p, nil, err
	}
	fields, err := tx.Query(ctx, "SELECT "+customFieldColumns+" FROM custom_fields WHERE (space_id=$1 OR space_id IS NULL) AND archived_at IS NULL", space)
	if err != nil {
		return p, nil, err
	}
	defs, err := pgx.CollectRows(fields, pgx.RowToStructByPos[CustomField])
	if err != nil {
		return p, nil, err
	}
	typeRows, err := tx.Query(ctx, "SELECT key,lower(name),base::text,default_workflow_id FROM item_type_defs WHERE active")
	if err != nil {
		return p, nil, err
	}
	type typeRef struct {
		key, base string
		workflow  *uuid.UUID // the type's default workflow, when it has one
	}
	activeTypes := map[string]typeRef{} // by key and by lower-cased name
	for typeRows.Next() {
		var key, name, base string
		var workflow *uuid.UUID
		if err := typeRows.Scan(&key, &name, &base, &workflow); err != nil {
			return p, nil, err
		}
		activeTypes[key], activeTypes[name] = typeRef{key, base, workflow}, typeRef{key, base, workflow}
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
			target = projectexcel.Canonical(source) // External Key, Parent Key, Finish Date and the older names
		}
		if target == "" {
			continue
		}
		if _, custom := definitions[target]; !custom && !projectexcel.Known(target) {
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
	destinationKind := ""
	inherited := uuid.Nil // the destination's effective workflow; every row under it inherits this
	if destination != uuid.Nil {
		kind, workflow, err := checkDestination(ctx, tx, actor, space, destination)
		if err != nil {
			add(1, "destination", err.Error())
		}
		destinationKind, inherited = kind, workflow
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
		if destinationKind == "project" && kind == "project" {
			add(row, "type", "a project can only be created at the Space root, not under a destination")
		}
		node.typeKey = typeKey
		node.statusName = strings.TrimSpace(values["status"])
		node.workflowName = strings.TrimSpace(values["workflow"])
		if emails := strings.TrimSpace(values["assignee_email"]); emails != "" {
			ids := []string{}
			for _, email := range strings.Split(emails, ";") {
				id, msg := resolveAssignee(ctx, tx, space, strings.TrimSpace(email))
				if msg != "" {
					add(row, "assignee_email", msg)
					continue
				}
				if id != actor.ID && !canAssign {
					add(row, "assignee_email", ErrAssignForbidden.Error())
					continue
				}
				ids = append(ids, id)
			}
			node.fields["assignee_ids"] = marshalJSON(ids)
		}
		if deps := strings.TrimSpace(values["depends_on"]); deps != "" {
			for _, token := range strings.Split(deps, ";") {
				key, kindOfDep, _ := strings.Cut(strings.TrimSpace(token), ":")
				key = strings.TrimSpace(key)
				if kindOfDep = strings.ToUpper(strings.TrimSpace(kindOfDep)); kindOfDep == "" {
					kindOfDep = "FS"
				}
				if key == "" {
					add(row, "depends_on", "a dependency needs an External Key, for example T001:FS")
					continue
				}
				if kindOfDep != "FS" {
					add(row, "depends_on", "only Finish-to-Start (FS) dependencies are supported in an import")
					continue
				}
				node.depends = append(node.depends, key)
			}
		}
		// Dates on the working calendar: a Duration must agree with explicit Start and Finish; a missing Finish is derived.
		start, hasStart := parseDay(values["start_date"])
		due, hasDue := parseDay(values["due_date"])
		days, errDays := strconv.Atoi(strings.TrimSpace(values["duration_days"]))
		hasDays := errDays == nil && days >= 0 && strings.TrimSpace(values["duration_days"]) != ""
		if hasStart && hasDue && hasDays && cal.Duration(start, due) != days {
			add(row, "duration_days", fmt.Sprintf("duration %d does not match Start and Finish: the working calendar gives %d days", days, cal.Duration(start, due)))
		}
		if hasStart && !hasDue && hasDays && days > 0 {
			due, hasDue = cal.Finish(start, days), true
			node.fields["due_date"] = marshalJSON(due.Format("2006-01-02"))
		}
		if hasDays {
			node.durationDays = days
		}
		if hasStart {
			node.startDay = start
		}
		if hasDue {
			node.finishDay = due
		}
		if string(node.fields["is_milestone"]) == "true" && node.fields["duration_days"] != nil && string(node.fields["duration_days"]) != "0" {
			add(row, "duration_days", "a milestone has zero duration")
		}
		for key, value := range values {
			field, ok := definitions[key]
			if !ok {
				continue
			}
			if slices.Contains(readOnly, field.ID) {
				add(row, key, "you cannot edit this restricted field")
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
	resolveWorkflows(ctx, tx, nodes, order, inherited, add)
	// Dependencies: each predecessor must be a row of this file, and the graph must stay acyclic.
	for _, id := range order {
		n := nodes[id]
		for _, pred := range n.depends {
			if _, ok := nodes[pred]; !ok {
				add(n.row, "depends_on", "unknown External Key "+pred+" in this file")
				continue
			}
			if pred == id {
				add(n.row, "depends_on", "an item cannot depend on itself")
				continue
			}
			p.Dependencies++
		}
	}
	dependencyState := map[string]int{} // 1 while visiting, 2 once done
	var postOrder []string              // predecessors before successors
	var dependencyVisit func(id string) bool
	dependencyVisit = func(id string) bool {
		switch dependencyState[id] {
		case 1:
			return true
		case 2:
			return false
		}
		dependencyState[id] = 1
		cyclic := false
		for _, pred := range nodes[id].depends {
			if _, ok := nodes[pred]; ok && pred != id && dependencyVisit(pred) {
				cyclic = true
			}
		}
		dependencyState[id] = 2
		postOrder = append(postOrder, id)
		if cyclic {
			add(nodes[id].row, "depends_on", "dependency cycle: this item depends on itself through its predecessors")
		}
		return cyclic
	}
	for _, id := range order {
		dependencyVisit(id)
	}
	// Auto-schedule: a row with no explicit Start starts on the working day after its predecessors finish, and its Finish
	// follows from Duration. An explicit Start is protected: when it conflicts, the row is reported, never moved.
	for _, id := range postOrder {
		n := nodes[id]
		var latest time.Time
		for _, pred := range n.depends {
			if pn, ok := nodes[pred]; ok && pn.finishDay.After(latest) {
				latest = pn.finishDay
			}
		}
		if latest.IsZero() {
			continue
		}
		if !n.startDay.IsZero() {
			if !n.startDay.After(latest) {
				add(n.row, "depends_on", fmt.Sprintf("starts %s, on or before its predecessor finishes (%s)", n.startDay.Format("2006-01-02"), latest.Format("2006-01-02")))
			}
			continue
		}
		start := cal.NextWorkday(latest.AddDate(0, 0, 1))
		n.startDay = start
		n.fields["start_date"] = marshalJSON(start.Format("2006-01-02"))
		scheduled := ScheduledRow{Row: n.row, Key: id, Start: start.Format("2006-01-02")}
		if n.finishDay.IsZero() && n.durationDays > 0 {
			n.finishDay = cal.Finish(start, n.durationDays)
			n.fields["due_date"] = marshalJSON(n.finishDay.Format("2006-01-02"))
		}
		if !n.finishDay.IsZero() {
			scheduled.Finish = n.finishDay.Format("2006-01-02")
		}
		p.Rescheduled = append(p.Rescheduled, scheduled)
		nodes[id] = n
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

// issueReason says where a problem sits: the header row, a whole row, or one value.
func issueReason(row int, col string) string {
	switch {
	case row == 1:
		return "column"
	case col == "":
		return "row"
	default:
		return "value"
	}
}

// issueFix is the next step for the user; the messages say what is wrong, this says what to do.
func issueFix(col, msg string) string {
	if strings.HasPrefix(msg, "unknown column") {
		return "Map this column to a LENS field, or choose to ignore it."
	}
	switch col {
	case "title":
		return "Fill the Title column with 1 to 500 characters."
	case "id":
		return "Make each External Key unique within the file."
	case "parent_id":
		return "Set Parent Key to the External Key of a row in this file, or leave it empty."
	case "type", "type_key":
		return "Use project, folder, task, or the key of an active item type."
	case "start_date", "due_date":
		return "Write the date as YYYY-MM-DD."
	case "duration_days":
		return "Write a whole number of days, zero or more."
	case "is_milestone":
		return "Write true or false."
	case "":
		return "Correct the values in this row and import again."
	default:
		return "Correct the value in this column and import again."
	}
}

// checkDestination validates the item a whole import is placed under: a project or folder of this Space the caller may add to.
// It returns the destination's kind and default workflow; every row of the import inherits that workflow.
func checkDestination(ctx context.Context, tx pgx.Tx, actor policy.Actor, space, destination uuid.UUID) (string, uuid.UUID, error) {
	var kind string
	var workflow *uuid.UUID
	err := tx.QueryRow(ctx, `SELECT i.type::text, COALESCE(i.default_workflow_id,s.workflow_id,(SELECT id FROM workflows WHERE is_default))
FROM items i LEFT JOIN statuses s ON s.id=i.status_id JOIN item_locations l ON l.item_id=i.id AND l.space_id=$2 AND l.deleted_root_id IS NULL
WHERE i.id=$1 AND i.deleted_at IS NULL`, destination, space).Scan(&kind, &workflow)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", uuid.Nil, errors.New("destination must be an item in this Space")
	}
	if err != nil {
		return "", uuid.Nil, err
	}
	if kind != "project" && kind != "folder" {
		return "", uuid.Nil, errors.New("destination must be a project or a folder")
	}
	caps, err := locationCaps(ctx, tx, actor, Location{SpaceID: space, ParentID: &destination})
	if err != nil {
		return "", uuid.Nil, err
	}
	if !caps.Has(policy.View) {
		return "", uuid.Nil, errors.New("destination must be an item in this Space")
	}
	if !caps.Has(policy.Create) {
		return "", uuid.Nil, errors.New("you cannot add items under this destination")
	}
	if workflow == nil {
		return kind, uuid.Nil, nil
	}
	return kind, *workflow, nil
}

// resolveWorkflows decides the workflow of every row and resolves its Status in it. A row inherits the workflow of its parent
// in the file; a root row uses the destination's workflow, else its Item Type's default, else the default workflow. An
// explicit Workflow column must name that same workflow, and an explicit Status must exist in it; nothing is dropped or
// replaced silently.
func resolveWorkflows(ctx context.Context, tx pgx.Tx, nodes map[string]excelNode, order []string, destination uuid.UUID, add func(int, string, string)) {
	effective := map[string]uuid.UUID{}
	var resolve func(id string, depth int) uuid.UUID
	resolve = func(id string, depth int) uuid.UUID {
		if w, ok := effective[id]; ok {
			return w
		}
		n := nodes[id]
		var w uuid.UUID
		if len(n.parents) > 0 && depth < len(nodes) {
			if _, ok := nodes[n.parents[0]]; ok {
				w = resolve(n.parents[0], depth+1)
			}
		}
		if w == uuid.Nil && len(n.parents) == 0 {
			w = destination
		}
		if w == uuid.Nil {
			w = rootWorkflow(ctx, tx, n.typeKey)
		}
		effective[id] = w
		return w
	}
	for _, id := range order {
		n := nodes[id]
		w := resolve(id, 0)
		if n.statusName == "" && n.workflowName == "" {
			continue
		}
		if w == uuid.Nil {
			add(n.row, "workflow", "no workflow to resolve this row's status in; set a default workflow or a destination")
			continue
		}
		var name string
		if err := tx.QueryRow(ctx, "SELECT name FROM workflows WHERE id=$1", w).Scan(&name); err != nil {
			add(n.row, "workflow", err.Error())
			continue
		}
		if n.workflowName != "" && !strings.EqualFold(n.workflowName, name) {
			add(n.row, "workflow", fmt.Sprintf("workflow %q is not the workflow of this row (%q)", n.workflowName, name))
			continue
		}
		if n.statusName == "" {
			continue
		}
		var status uuid.UUID
		err := tx.QueryRow(ctx, "SELECT id FROM statuses WHERE workflow_id=$1 AND lower(name)=lower($2)", w, n.statusName).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			add(n.row, "status", fmt.Sprintf("status %q is not in workflow %q", n.statusName, name))
			continue
		}
		if err != nil {
			add(n.row, "status", err.Error())
			continue
		}
		// A new item starts in the workflow's first status. If that status locks its transitions, the explicit status must be
		// reachable from it, exactly as the item patch path enforces after creation.
		var initial uuid.UUID
		var locked bool
		if err := tx.QueryRow(ctx, "SELECT id, lock_transitions FROM statuses WHERE workflow_id=$1 ORDER BY position LIMIT 1", w).Scan(&initial, &locked); err != nil {
			add(n.row, "status", err.Error())
			continue
		}
		if locked && initial != status {
			var allowed bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM status_transitions WHERE from_status_id=$1 AND to_status_id=$2)", initial, status).Scan(&allowed); err != nil {
				add(n.row, "status", err.Error())
				continue
			}
			if !allowed {
				add(n.row, "status", fmt.Sprintf("status %q cannot follow the starting status of workflow %q: its transitions are locked", n.statusName, name))
				continue
			}
		}
		n.fields["status_id"] = marshalJSON(status)
		nodes[id] = n
	}
}

// rootWorkflow is the workflow a root row uses when no destination is given: its Item Type's default, else the default workflow.
func rootWorkflow(ctx context.Context, tx pgx.Tx, typeKey string) uuid.UUID {
	var w *uuid.UUID
	if typeKey != "" {
		_ = tx.QueryRow(ctx, "SELECT default_workflow_id FROM item_type_defs WHERE key=$1 AND active", typeKey).Scan(&w)
	}
	if w != nil {
		return *w
	}
	var def uuid.UUID
	if err := tx.QueryRow(ctx, "SELECT id FROM workflows WHERE is_default").Scan(&def); err != nil {
		return uuid.Nil
	}
	return def
}

// resolveAssignee matches an active user by email and checks that the user can reach the Space.
func resolveAssignee(ctx context.Context, tx pgx.Tx, space uuid.UUID, email string) (string, string) {
	var id, userType string
	err := tx.QueryRow(ctx, "SELECT id::text, user_type::text FROM users WHERE email=$1 AND status='active'", email).Scan(&id, &userType)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Sprintf("assignee email %q does not match an active user", email)
	}
	if err != nil {
		return "", err.Error()
	}
	access, _, err := policy.SpaceAccessFor(ctx, tx, policy.Actor{ID: id, Type: policy.UserType(userType)}, space.String())
	if err != nil {
		return "", err.Error()
	}
	if !access.Caps.Has(policy.View) {
		return "", fmt.Sprintf("assignee %q cannot access this Space", email)
	}
	return id, ""
}

// parseDay reads a YYYY-MM-DD cell; blank or malformed cells report false.
func parseDay(value string) (time.Time, bool) {
	day, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	return day, err == nil
}
