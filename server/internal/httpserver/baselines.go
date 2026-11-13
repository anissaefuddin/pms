package httpserver

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const maxBaselinesPerProject = 10

func (s *Server) baselineRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/items/{projectId}/baselines", s.requireUser(s.listBaselines))
	mux.HandleFunc("POST /api/v1/items/{projectId}/baselines", s.requireUser(s.createBaseline))
	mux.HandleFunc("GET /api/v1/baselines/{baselineId}/variance", s.requireUser(s.baselineVariance))
}

type baselineBody struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	CreatedBy *string   `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Items     int       `json:"item_count"`
}

const baselineSelect = `SELECT b.id::text,b.project_id::text,b.name,
 (SELECT concat_ws(' ',u.first_name,u.last_name) FROM users u WHERE u.id=b.created_by),b.created_at,
 (SELECT count(*)::int FROM baseline_items x WHERE x.baseline_id=b.id) FROM baselines b `

func (s *Server) listBaselines(w http.ResponseWriter, r *http.Request) {
	project, err := uuid.Parse(r.PathValue("projectId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err := approvalCaps(ctx, tx, actorOf(r), project, policy.View); err != nil {
		writeItemError(w, err)
		return
	}
	rows, err := tx.Query(ctx, baselineSelect+"WHERE b.project_id=$1 ORDER BY b.created_at DESC,b.id", project)
	if err != nil {
		writeItemError(w, err)
		return
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[baselineBody])
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

// createBaseline freezes the dates of the project's items that the actor can see.
func (s *Server) createBaseline(w http.ResponseWriter, r *http.Request) {
	project, err := uuid.Parse(r.PathValue("projectId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 120 || !utf8.ValidString(name) || strings.ContainsRune(name, 0) {
		writeItemError(w, items.InvalidField{Field: "name", Message: "name must be 1 to 120 characters"})
		return
	}
	ctx := r.Context()
	actor := actorOf(r)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err := approvalCaps(ctx, tx, actor, project, policy.Edit); err != nil {
		writeItemError(w, err)
		return
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", "baseline:"+project.String()); err != nil {
		writeItemError(w, err)
		return
	}
	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM baselines WHERE project_id=$1", project).Scan(&count); err != nil {
		writeItemError(w, err)
		return
	}
	if count >= maxBaselinesPerProject {
		writeItemError(w, items.Invalid("a project can have at most 10 baselines"))
		return
	}
	rows, err := tx.Query(ctx, `
WITH RECURSIVE t(id, depth) AS (
  SELECT l.item_id, 1 FROM item_locations l JOIN items c ON c.id=l.item_id AND c.deleted_at IS NULL
   WHERE l.parent_item_id=$1 AND l.deleted_root_id IS NULL
  UNION
  SELECT l.item_id, t.depth+1 FROM t JOIN item_locations l ON l.parent_item_id=t.id AND l.deleted_root_id IS NULL
   JOIN items c ON c.id=l.item_id AND c.deleted_at IS NULL WHERE t.depth < 30
)
SELECT i.id::text,i.start_date,i.due_date FROM items i WHERE i.id IN (SELECT id FROM t) AND (i.start_date IS NOT NULL OR i.due_date IS NOT NULL) ORDER BY i.id LIMIT 5001`, project)
	if err != nil {
		writeItemError(w, err)
		return
	}
	type dated struct {
		ID           string
		Start, Final *time.Time
	}
	all, err := pgx.CollectRows(rows, pgx.RowToStructByPos[dated])
	if err != nil {
		writeItemError(w, err)
		return
	}
	if len(all) > 5000 {
		writeItemError(w, items.Invalid("a baseline can hold at most 5000 dated items"))
		return
	}
	ids := make([]string, len(all))
	for i, d := range all {
		ids[i] = d.ID
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		writeItemError(w, err)
		return
	}
	id, err := uuid.NewV7()
	if err != nil {
		writeItemError(w, err)
		return
	}
	if _, err := tx.Exec(ctx, "INSERT INTO baselines(id,project_id,name,created_by) VALUES($1,$2,$3,$4::uuid)", id, project, name, actor.ID); err != nil {
		writeItemError(w, err)
		return
	}
	batch := &pgx.Batch{}
	for _, d := range all {
		if caps[d.ID].Has(policy.View) {
			batch.Queue("INSERT INTO baseline_items(baseline_id,item_id,start_date,due_date) VALUES($1,$2,$3,$4)", id, d.ID, d.Start, d.Final)
		}
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		writeItemError(w, err)
		return
	}
	row, err := tx.Query(ctx, baselineSelect+"WHERE b.id=$1", id)
	if err != nil {
		writeItemError(w, err)
		return
	}
	made, err := pgx.CollectOneRow(row, pgx.RowToStructByPos[baselineBody])
	if err != nil {
		writeItemError(w, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, made)
}

type varianceRow struct {
	ItemID        string  `json:"item_id"`
	BaselineStart *string `json:"baseline_start"`
	BaselineDue   *string `json:"baseline_due"`
	Start         *string `json:"start"`
	Due           *string `json:"due"`
	StartVariance *int    `json:"start_variance_days"`
	DueVariance   *int    `json:"due_variance_days"`
}

// baselineVariance compares the frozen dates with the current ones; positive means later than planned.
func (s *Server) baselineVariance(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("baselineId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	ctx := r.Context()
	actor := actorOf(r)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var project uuid.UUID
	if err := tx.QueryRow(ctx, "SELECT project_id FROM baselines WHERE id=$1", id).Scan(&project); err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	if _, err := approvalCaps(ctx, tx, actor, project, policy.View); err != nil {
		writeItemError(w, err)
		return
	}
	rows, err := tx.Query(ctx, `SELECT b.item_id::text,b.start_date::text,b.due_date::text,i.start_date::text,i.due_date::text,
 (i.start_date-b.start_date)::int,(i.due_date-b.due_date)::int
 FROM baseline_items b JOIN items i ON i.id=b.item_id AND i.deleted_at IS NULL WHERE b.baseline_id=$1 ORDER BY b.item_id`, id)
	if err != nil {
		writeItemError(w, err)
		return
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[varianceRow])
	if err != nil {
		writeItemError(w, err)
		return
	}
	ids := make([]string, len(list))
	for i, v := range list {
		ids[i] = v.ItemID
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		writeItemError(w, err)
		return
	}
	out := list[:0]
	for _, v := range list {
		if caps[v.ItemID].Has(policy.View) {
			out = append(out, v)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}
