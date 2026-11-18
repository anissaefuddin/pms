package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/projectexcel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Server) projectExcelRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/excel-imports", s.requireUser(s.importProjectExcel))
	mux.HandleFunc("POST /api/v1/items/{projectId}/excel-exports", s.requireUser(s.createExcelExport))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/excel-exports", s.requireUser(s.createSpaceExcelExport))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/excel-templates/{kind}", s.requireUser(s.excelTemplate))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/excel-imports/{reportId}", s.requireUser(s.excelImportReport))
	mux.HandleFunc("GET /api/v1/excel-imports/{reportId}", s.requireUser(s.excelImportReportByID))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/excel-destinations", s.requireUser(s.excelDestinations))
	mux.HandleFunc("GET /api/v1/excel-exports/{exportId}", s.requireUser(s.getExcelExport))
	mux.HandleFunc("GET /api/v1/excel-exports/{exportId}/file", s.requireUser(s.getExcelExport))
}

func excelError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "project or export not found")
		return
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01") {
		writeError(w, 409, "Excel export changed concurrently; try again")
		return
	}
	writeItemError(w, err)
}

// exportFilterInput is the filter a client sends with an export: the Table View state (see tableFilter in the web client).
type exportFilterInput struct {
	StatusGroups []string `json:"status_groups"`
	Assignee     string   `json:"assignee"`
	Search       string   `json:"search"`
	Sort         string   `json:"sort"`
	Columns      []string `json:"columns"`
	Types        []string `json:"types"`
	Fields       []string `json:"fields"`
}

func (s *Server) createExcelExport(w http.ResponseWriter, r *http.Request) {
	project, ok := pathUUID(w, r, "projectId")
	if !ok {
		return
	}
	var input struct {
		SpaceID uuid.UUID          `json:"space_id"`
		Filter  *exportFilterInput `json:"filter"`
	}
	if !decode(w, r, &input) {
		return
	}
	s.enqueueExport(w, r, projectexcel.ScopeProject, &project, input.SpaceID, input.Filter)
}

// createSpaceExcelExport exports a whole Space: every root the caller can see, with the hierarchy under it (EXP-03).
func (s *Server) createSpaceExcelExport(w http.ResponseWriter, r *http.Request) {
	space, ok := pathUUID(w, r, "spaceId")
	if !ok {
		return
	}
	var input struct {
		Filter *exportFilterInput `json:"filter"`
	}
	if !decode(w, r, &input) {
		return
	}
	s.enqueueExport(w, r, projectexcel.ScopeSpace, nil, space, input.Filter)
}

// enqueueExport validates the scope, its target and the filter, takes the Table View snapshot the caller can see, and queues
// the export. A project export names a project in the Space; a Space export names the Space and no project.
func (s *Server) enqueueExport(w http.ResponseWriter, r *http.Request, scope string, project *uuid.UUID, space uuid.UUID, input *exportFilterInput) {
	ctx, actor := r.Context(), actorOf(r)
	var filter projectexcel.ExportFilter
	if input != nil {
		for _, group := range input.StatusGroups {
			if !slices.Contains([]string{"active", "completed", "deferred", "cancelled"}, group) {
				writeError(w, 422, "status_groups must be active, completed, deferred, or cancelled")
				return
			}
		}
		assignee := input.Assignee
		if assignee == "me" {
			assignee = actor.ID
		} else if assignee != "" {
			if _, err := uuid.Parse(assignee); err != nil {
				writeError(w, 422, "assignee must be a user id or me")
				return
			}
		}
		sortKey := strings.TrimPrefix(input.Sort, "-")
		if input.Sort != "" && !slices.Contains([]string{"title", "start_date", "due_date"}, sortKey) {
			writeError(w, 422, "sort must be title, start_date, or due_date, optionally with a leading minus")
			return
		}
		for _, column := range input.Columns {
			if !projectexcel.ValidColumn(column) {
				writeError(w, 422, "unknown column "+column)
				return
			}
		}
		if len(input.Search) > 200 {
			writeError(w, 422, "search must be at most 200 characters")
			return
		}
		filter = projectexcel.ExportFilter{Scoped: true, Sort: input.Sort, Columns: input.Columns}
		root, rootChildren := space.String(), false
		if scope == projectexcel.ScopeProject {
			root, rootChildren = project.String(), true
		}
		matches, err := s.exportMatches(ctx, actor, root, rootChildren, items.ListFilter{StatusGroups: input.StatusGroups, Assignee: assignee,
			Types: input.Types, Fields: input.Fields}, input.Search, scope == projectexcel.ScopeSpace)
		if err != nil {
			writeItemError(w, err)
			return
		}
		filter.Matches = matches
	} else if scope == projectexcel.ScopeSpace {
		// A Space export always holds the snapshot of what the caller can see, even without a filter.
		matches, err := s.exportMatches(ctx, actor, space.String(), false, items.ListFilter{}, "", true)
		if err != nil {
			writeItemError(w, err)
			return
		}
		filter = projectexcel.ExportFilter{Scoped: true, Matches: matches}
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		excelError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if err := tx.QueryRow(ctx, "SELECT user_type::text FROM users WHERE id=$1::uuid AND status='active' FOR SHARE", actor.ID).Scan(&actor.Type); err != nil {
		excelError(w, err)
		return
	}
	if scope == projectexcel.ScopeProject {
		if err := projectexcel.CheckScope(ctx, tx, actor, *project, space); err != nil {
			excelError(w, err)
			return
		}
	} else if access, _, err := policy.SpaceAccessFor(ctx, tx, actor, space.String()); err != nil {
		excelError(w, err)
		return
	} else if !access.Caps.Has(policy.View) {
		writeError(w, 404, "space not found")
		return
	}
	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM project_excel_exports WHERE user_id=$1::uuid AND expires_at>now()", actor.ID).Scan(&count); err != nil {
		excelError(w, err)
		return
	}
	if count >= 5 {
		writeError(w, 429, "at most five Excel exports per hour")
		return
	}
	var id uuid.UUID
	err = tx.QueryRow(ctx, `INSERT INTO project_excel_exports(project_id,space_id,user_id,scope)
VALUES($1,$2,$3::uuid,$4) RETURNING id`, project, space, actor.ID, scope).Scan(&id)
	if err == nil {
		err = s.mail.ExportProjectTx(ctx, tx, projectexcel.ExportArgs{ID: id, Filter: filter})
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		excelError(w, err)
		return
	}
	writeJSON(w, 202, map[string]any{"id": id, "status": "pending", "scope": scope})
}

func (s *Server) getExcelExport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "exportId")
	if !ok {
		return
	}
	ctx, actor := r.Context(), actorOf(r)
	w.Header().Set("Cache-Control", "private, no-store")
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		excelError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var project *uuid.UUID
	var space uuid.UUID
	var scope string
	var ids []uuid.UUID
	var status, message string
	var fields bool
	var total, processed int
	err = tx.QueryRow(ctx, `SELECT e.scope,e.project_id,e.space_id,e.status,COALESCE(e.error,''),e.item_ids,e.includes_fields,e.total_items,e.processed_items,u.user_type::text
FROM project_excel_exports e JOIN users u ON u.id=e.user_id AND u.status='active'
WHERE e.id=$1 AND e.user_id=$2::uuid AND e.expires_at>now()`, id, actor.ID).
		Scan(&scope, &project, &space, &status, &message, &ids, &fields, &total, &processed, &actor.Type)
	if err == nil {
		err = checkExportAccess(ctx, tx, actor, scope, project, space)
	}
	if err != nil {
		excelError(w, err)
		return
	}
	if status == "ready" {
		allowed, err := projectexcel.CanDownload(ctx, tx, actor, scope, project, space, ids, fields)
		if err != nil {
			excelError(w, err)
			return
		}
		if !allowed {
			excelError(w, pgx.ErrNoRows)
			return
		}
	}
	if r.URL.Path == "/api/v1/excel-exports/"+r.PathValue("exportId")+"/file" {
		if status != "ready" {
			writeError(w, 409, "export is not ready")
			return
		}
		var file []byte
		if err := tx.QueryRow(ctx, "SELECT workbook FROM project_excel_exports WHERE id=$1", id).Scan(&file); err != nil {
			excelError(w, err)
			return
		}
		// Checked on every request: a grant revoked after the export must stop the stored file from leaving the server.
		readable, err := projectexcel.Readable(ctx, tx, actor, file)
		if err != nil {
			excelError(w, err)
			return
		}
		if !readable {
			writeError(w, 404, "export is no longer available; create a new export")
			return
		}
		if err := tx.Commit(ctx); err != nil {
			excelError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		name := "space-" + space.String()
		if project != nil {
			name = "project-" + project.String()
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, name))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write(file)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		excelError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "status": status, "error": message, "scope": scope,
		"total_items": total, "processed_items": processed})
}

// writeWorkbookError answers a workbook the importer refuses: a problem+json body whose errors name the sheet, the reason and the fix.
func writeWorkbookError(w http.ResponseWriter, err error) {
	var issue *projectexcel.Error
	if !errors.As(err, &issue) {
		writeError(w, 422, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": "Unprocessable Entity", "status": 422,
		"code": "validation_failed", "detail": issue.Error(), "request_id": w.Header().Get("X-Request-ID"),
		"errors": []map[string]string{{"field": "workbook", "message": issue.Error(), "sheet": issue.Sheet, "reason": issue.Reason, "suggested_fix": issue.Fix}}})
}

func (s *Server) importProjectExcel(w http.ResponseWriter, r *http.Request) {
	space, ok := pathUUID(w, r, "spaceId")
	if !ok {
		return
	}
	ctx, actor := r.Context(), actorOf(r)
	access, _, err := policy.SpaceAccessFor(ctx, s.pool, actor, space.String())
	if err != nil {
		writeItemError(w, err)
		return
	}
	if !access.Caps.Has(policy.View) {
		writeItemError(w, items.ErrNotFound)
		return
	}
	if !access.Caps.Has(policy.Create) {
		writeItemError(w, items.ErrForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, projectexcel.MaxFileBytes+(64<<10))
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, 422, "upload an XLSX workbook as multipart form data")
		return
	}
	var file []byte
	values := map[string]string{}
	seen := map[string]bool{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeError(w, 422, "invalid or oversized upload")
			return
		}
		key := part.FormName()
		if seen[key] {
			writeError(w, 422, "duplicate upload field")
			return
		}
		seen[key] = true
		if key == "file" {
			if !strings.HasSuffix(strings.ToLower(part.FileName()), ".xlsx") {
				writeError(w, 422, "choose an .xlsx file")
				return
			}
			file, err = io.ReadAll(io.LimitReader(part, projectexcel.MaxFileBytes+1))
			if err != nil || len(file) > projectexcel.MaxFileBytes {
				writeError(w, 422, "workbook exceeds 5 MiB")
				return
			}
		} else {
			if key != "mapping" && key != "commit" && key != "digest" && key != "destination" {
				writeError(w, 422, "unknown upload field")
				return
			}
			data, err := io.ReadAll(io.LimitReader(part, (32<<10)+1))
			if err != nil || len(data) > 32<<10 {
				writeError(w, 422, "upload field is too large")
				return
			}
			values[key] = string(data)
		}
	}
	var mapping map[string]string
	if value := values["mapping"]; value != "" && (json.Unmarshal([]byte(value), &mapping) != nil || mapping == nil || len(mapping) > projectexcel.MaxColumns) {
		writeError(w, 422, "mapping must be an object of column names")
		return
	}
	if len(mapping) == 0 {
		mapping = nil
	}
	destination := uuid.Nil
	if value := values["destination"]; value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			writeError(w, 422, "destination must be the id of a project or folder")
			return
		}
		destination = parsed
	}
	canonical, _ := json.Marshal(mapping)
	hash := sha256.New()
	hash.Write(canonical)
	hash.Write([]byte{0})
	hash.Write([]byte(destination.String())) // the destination is part of the digest, so a retry cannot land elsewhere
	hash.Write([]byte{0})
	hash.Write(file)
	digest := hex.EncodeToString(hash.Sum(nil))
	commit := values["commit"] == "true"
	if values["commit"] != "" && !commit {
		writeError(w, 422, "commit must be true")
		return
	}
	if commit && values["digest"] != digest {
		writeError(w, 409, "workbook or mapping changed; preview again")
		return
	}
	sheet, err := projectexcel.ReadLENS(file)
	if err != nil {
		writeWorkbookError(w, err)
		return
	}
	batch := s.notificationBatch()
	preview, ids, err := items.ImportExcel(ctx, s.pool, actor, space, destination, sheet.Name, sheet.Rows, mapping, digest, commit, batch)
	if err != nil {
		writeItemError(w, err)
		return
	}
	if !commit || len(preview.Errors) > 0 {
		if commit {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(422)
			fields := make([]map[string]string, 0, len(preview.Errors))
			for _, issue := range preview.Errors {
				field := fmt.Sprintf("rows.%d", issue.Row)
				if issue.Column != "" {
					field += "." + issue.Column
				}
				fields = append(fields, map[string]string{"field": field, "message": issue.Message, "sheet": issue.Sheet, "reason": issue.Reason, "suggested_fix": issue.SuggestedFix})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": "Unprocessable Entity", "status": 422,
				"code": "validation_failed", "detail": "the import has invalid rows", "request_id": w.Header().Get("X-Request-ID"), "errors": fields})
			return
		}
		writeJSON(w, 200, struct {
			items.ExcelPreview
			Digest string `json:"digest"`
		}{preview, digest})
		return
	}
	s.publishNotifications(batch)
	writeJSON(w, 201, map[string]any{"item_ids": ids, "summary": map[string]any{"created": len(ids), "dependencies": preview.Dependencies,
		"rescheduled": preview.Rescheduled, "report_id": preview.ReportID}})
}

// excelTemplate serves a versioned LENS template whose advanced columns are the Space's custom fields the caller may read.
func (s *Server) excelTemplate(w http.ResponseWriter, r *http.Request) {
	space, ok := pathUUID(w, r, "spaceId")
	if !ok {
		return
	}
	kind := r.PathValue("kind")
	ctx, actor := r.Context(), actorOf(r)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		excelError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	access, _, err := policy.SpaceAccessFor(ctx, tx, actor, space.String())
	if err != nil {
		excelError(w, err)
		return
	}
	if !access.Caps.Has(policy.View) {
		writeError(w, 404, "space not found")
		return
	}
	fields, err := projectexcel.TemplateFields(ctx, tx, actor, space)
	if err != nil {
		excelError(w, err)
		return
	}
	rows, err := projectexcel.TemplateRows(kind, fields)
	if err != nil {
		writeError(w, 404, "template not found")
		return
	}
	file, err := projectexcel.Write(rows)
	if err != nil {
		excelError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="lens-template-%s-v%d.xlsx"`, kind, projectexcel.TemplateVersion))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(file)
}

// excelImportReport returns the report of an import the caller made in this Space.
func (s *Server) excelImportReport(w http.ResponseWriter, r *http.Request) {
	space, ok := pathUUID(w, r, "spaceId")
	if !ok {
		return
	}
	report, ok := pathUUID(w, r, "reportId")
	if !ok {
		return
	}
	ctx, actor := r.Context(), actorOf(r)
	access, _, err := policy.SpaceAccessFor(ctx, s.pool, actor, space.String())
	if err != nil {
		excelError(w, err)
		return
	}
	if !access.Caps.Has(policy.View) {
		writeError(w, 404, "report not found")
		return
	}
	var raw []byte
	err = s.pool.QueryRow(ctx, "SELECT report FROM excel_import_reports WHERE id=$1 AND space_id=$2 AND user_id=$3::uuid", report, space, actor.ID).Scan(&raw)
	if err != nil {
		excelError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
}

// exportMatches is the snapshot an export follows. With a filter, it is every item the item engine returns for the filter.
// With all (a Space export), it is every item the caller can see when no filter is given. It walks every container
// (so a match deep in the tree is found), pages through each one, and applies the search to titles the way the table does.
func (s *Server) exportMatches(ctx context.Context, actor policy.Actor, root string, rootChildren bool, filter items.ListFilter, search string, all bool) ([]uuid.UUID, error) {
	type row struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Type  string `json:"type"`
	}
	read := func(container string, children bool, f items.ListFilter) ([]row, error) {
		var out []row
		f.Limit = 200
		for cursor := ""; ; {
			f.Cursor = cursor
			page, err := items.List(ctx, s.pool, actor, container, children, f)
			if err != nil {
				return nil, err
			}
			for _, raw := range page.Items {
				var r row
				if err := json.Unmarshal(raw, &r); err != nil {
					return nil, err
				}
				out = append(out, r)
			}
			if page.NextCursor == nil {
				return out, nil
			}
			cursor = *page.NextCursor
		}
	}
	active := len(filter.StatusGroups) > 0 || filter.Assignee != "" || len(filter.Types) > 0 || len(filter.Fields) > 0 || search != ""
	matches := []uuid.UUID{}
	needle := strings.ToLower(search)
	seen := map[string]bool{root: true}
	queue := []string{root}
	first := true
	for len(queue) > 0 {
		container := queue[0]
		queue = queue[1:]
		childrenOf := true
		if first {
			childrenOf = rootChildren
			first = false
		}
		children, err := read(container, childrenOf, items.ListFilter{})
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			if (child.Type == "folder" || child.Type == "project") && !seen[child.ID] {
				seen[child.ID] = true
				queue = append(queue, child.ID)
			}
			if all && !active {
				id, err := uuid.Parse(child.ID)
				if err != nil {
					return nil, err
				}
				matches = append(matches, id)
			}
		}
		if !active {
			continue
		}
		picked, err := read(container, childrenOf, filter)
		if err != nil {
			return nil, err
		}
		for _, hit := range picked {
			if needle != "" && !strings.Contains(strings.ToLower(hit.Title), needle) {
				continue
			}
			id, err := uuid.Parse(hit.ID)
			if err != nil {
				return nil, err
			}
			matches = append(matches, id)
		}
	}
	return matches, nil
}

// excelDestinations lists the projects and folders one level under a parent (or at the Space root) that the caller can see,
// and whether the caller can create items under each. The destination picker disables the ones with can_create false.
func (s *Server) excelDestinations(w http.ResponseWriter, r *http.Request) {
	space, ok := pathUUID(w, r, "spaceId")
	if !ok {
		return
	}
	ctx, actor := r.Context(), actorOf(r)
	access, _, err := policy.SpaceAccessFor(ctx, s.pool, actor, space.String())
	if err != nil {
		excelError(w, err)
		return
	}
	if !access.Caps.Has(policy.View) {
		writeError(w, 404, "space not found")
		return
	}
	container, children := space.String(), false
	if parent := r.URL.Query().Get("parent"); parent != "" {
		id, err := uuid.Parse(parent)
		if err != nil {
			writeError(w, 422, "parent must be an item id")
			return
		}
		container, children = id.String(), true
	}
	page, err := items.List(ctx, s.pool, actor, container, children, items.ListFilter{Types: []string{"project", "folder"}, Limit: 200})
	if err != nil {
		writeItemError(w, err)
		return
	}
	type row struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Type      string `json:"type"`
		CanCreate bool   `json:"can_create"`
	}
	out := make([]row, 0, len(page.Items))
	for _, raw := range page.Items {
		var r0 row
		if err := json.Unmarshal(raw, &r0); err != nil {
			excelError(w, err)
			return
		}
		caps, err := policy.ItemCapsInSpace(ctx, s.pool, actor, r0.ID, space.String())
		if err != nil {
			excelError(w, err)
			return
		}
		r0.CanCreate = caps.Has(policy.Create)
		out = append(out, r0)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "next_cursor": page.NextCursor})
}

// excelImportReportByID returns a report the caller made, found by id. Access follows the report's Space.
func (s *Server) excelImportReportByID(w http.ResponseWriter, r *http.Request) {
	report, ok := pathUUID(w, r, "reportId")
	if !ok {
		return
	}
	ctx, actor := r.Context(), actorOf(r)
	var space uuid.UUID
	var raw []byte
	err := s.pool.QueryRow(ctx, "SELECT space_id, report FROM excel_import_reports WHERE id=$1 AND user_id=$2::uuid", report, actor.ID).Scan(&space, &raw)
	if err != nil {
		excelError(w, err)
		return
	}
	access, _, err := policy.SpaceAccessFor(ctx, s.pool, actor, space.String())
	if err != nil {
		excelError(w, err)
		return
	}
	if !access.Caps.Has(policy.View) {
		writeError(w, 404, "report not found")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
}

// checkExportAccess is the view a caller needs to read an export: the project's view for a project export, the Space's view
// for a Space export. Both are checked on every status and download request.
func checkExportAccess(ctx context.Context, tx pgx.Tx, actor policy.Actor, scope string, project *uuid.UUID, space uuid.UUID) error {
	if scope == projectexcel.ScopeProject {
		if project == nil {
			return pgx.ErrNoRows
		}
		return projectexcel.CheckScope(ctx, tx, actor, *project, space)
	}
	access, _, err := policy.SpaceAccessFor(ctx, tx, actor, space.String())
	if err != nil {
		return err
	}
	if !access.Caps.Has(policy.View) {
		return pgx.ErrNoRows
	}
	return nil
}
