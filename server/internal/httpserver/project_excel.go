package httpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

func (s *Server) createExcelExport(w http.ResponseWriter, r *http.Request) {
	project, ok := pathUUID(w, r, "projectId")
	if !ok {
		return
	}
	var input struct {
		SpaceID uuid.UUID `json:"space_id"`
	}
	if !decode(w, r, &input) {
		return
	}
	ctx, actor := r.Context(), actorOf(r)
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
	if err := projectexcel.CheckScope(ctx, tx, actor, project, input.SpaceID); err != nil {
		excelError(w, err)
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
	err = tx.QueryRow(ctx, `INSERT INTO project_excel_exports(project_id,space_id,user_id)
VALUES($1,$2,$3::uuid) RETURNING id`, project, input.SpaceID, actor.ID).Scan(&id)
	if err == nil {
		err = s.mail.ExportProjectTx(ctx, tx, projectexcel.ExportArgs{ID: id})
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		excelError(w, err)
		return
	}
	writeJSON(w, 202, map[string]any{"id": id, "status": "pending"})
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
	var project, space uuid.UUID
	var ids []uuid.UUID
	var status, message string
	var fields bool
	err = tx.QueryRow(ctx, `SELECT e.project_id,e.space_id,e.status,COALESCE(e.error,''),e.item_ids,e.includes_fields,u.user_type::text
FROM project_excel_exports e JOIN users u ON u.id=e.user_id AND u.status='active'
WHERE e.id=$1 AND e.user_id=$2::uuid AND e.expires_at>now()`, id, actor.ID).
		Scan(&project, &space, &status, &message, &ids, &fields, &actor.Type)
	if err == nil {
		err = projectexcel.CheckScope(ctx, tx, actor, project, space)
	}
	if err != nil {
		excelError(w, err)
		return
	}
	if status == "ready" {
		allowed, err := projectexcel.CanDownload(ctx, tx, actor, project, space, ids, fields)
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
		if err := tx.Commit(ctx); err != nil {
			excelError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="project-%s.xlsx"`, project))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write(file)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		excelError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "status": status, "error": message})
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
			if key != "mapping" && key != "commit" && key != "digest" {
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
	canonical, _ := json.Marshal(mapping)
	hash := sha256.New()
	hash.Write(canonical)
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
	rows, err := projectexcel.Read(file)
	if err != nil {
		writeError(w, 422, err.Error())
		return
	}
	batch := s.notificationBatch()
	preview, ids, err := items.ImportExcel(ctx, s.pool, actor, space, rows, mapping, digest, commit, batch)
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
				fields = append(fields, map[string]string{"field": field, "message": issue.Message})
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
	writeJSON(w, 201, map[string]any{"item_ids": ids})
}
