package httpserver

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const maxGanttSnapshotsPerProject = 20

// GanttSnapshotRoutes is separate so the main router can wire this isolated feature.
func (s *Server) GanttSnapshotRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/items/{projectId}/gantt-snapshots", s.requireUser(s.listGanttSnapshots))
	mux.HandleFunc("POST /api/v1/items/{projectId}/gantt-snapshots", s.requireUser(s.createGanttSnapshot))
	mux.HandleFunc("DELETE /api/v1/gantt-snapshots/{snapshotId}", s.requireUser(s.revokeGanttSnapshot))
	// No requireUser: a stranger holds the token, and no cookie is read.
	if s.cfg.PublicLinks { // B3-17: public Gantt snapshots are off by default
		mux.HandleFunc("GET /api/v1/public/gantt/{token}", s.getPublicGanttSnapshot)
	}
}

type ganttSnapshotItem struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	ParentID  *string  `json:"parent_id"`
	StartDate *string  `json:"start_date"`
	DueDate   *string  `json:"due_date"`
	Progress  *int     `json:"progress_percent"`
	Assignees []string `json:"assignees,omitempty"`
}

type ganttSnapshotPayload struct {
	ProjectID    string              `json:"project_id"`
	ProjectTitle string              `json:"project_title"`
	Items        []ganttSnapshotItem `json:"items"`
}

type ganttSnapshotBody struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	Name          string    `json:"name"`
	ShowAssignees bool      `json:"show_assignees"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
	Token         string    `json:"token,omitempty"`
}

func snapshotHash(token string) [32]byte { return sha256.Sum256([]byte(token)) }

func (s *Server) listGanttSnapshots(w http.ResponseWriter, r *http.Request) {
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
	rows, err := tx.Query(ctx, `SELECT id::text,project_id::text,name,show_assignees,expires_at,created_at,''
 FROM gantt_snapshots WHERE project_id=$1 ORDER BY created_at DESC,id`, project)
	if err != nil {
		writeItemError(w, err)
		return
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[ganttSnapshotBody])
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (s *Server) createGanttSnapshot(w http.ResponseWriter, r *http.Request) {
	project, err := uuid.Parse(r.PathValue("projectId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	var body struct {
		Name          string    `json:"name"`
		ShowAssignees bool      `json:"show_assignees"`
		ExpiresAt     time.Time `json:"expires_at"`
	}
	if !decode(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 120 || !utf8.ValidString(name) || strings.ContainsRune(name, 0) {
		writeItemError(w, items.InvalidField{Field: "name", Message: "name must be 1 to 120 characters"})
		return
	}
	now := time.Now().UTC()
	if body.ExpiresAt.Before(now.Add(time.Minute)) || body.ExpiresAt.After(now.Add(90*24*time.Hour)) {
		writeItemError(w, items.InvalidField{Field: "expires_at", Message: "expiry must be between 1 minute and 90 days from now"})
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	actor := actorOf(r)
	if _, err := approvalCaps(ctx, tx, actor, project, policy.Edit); err != nil {
		writeItemError(w, err)
		return
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", "gantt-snapshot:"+project.String()); err != nil {
		writeItemError(w, err)
		return
	}
	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM gantt_snapshots WHERE project_id=$1", project).Scan(&count); err != nil {
		writeItemError(w, err)
		return
	}
	if count >= maxGanttSnapshotsPerProject {
		writeItemError(w, items.Invalid("a project can have at most 20 snapshots"))
		return
	}
	var title string
	if err := tx.QueryRow(ctx, "SELECT title FROM items WHERE id=$1 AND type='project' AND deleted_at IS NULL", project).Scan(&title); err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	rows, err := tx.Query(ctx, `WITH RECURSIVE t(id,parent_id,depth) AS (
 SELECT l.item_id,l.parent_item_id,1 FROM item_locations l JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
  WHERE l.parent_item_id=$1 AND l.deleted_root_id IS NULL
 UNION SELECT l.item_id,l.parent_item_id,t.depth+1 FROM t JOIN item_locations l ON l.parent_item_id=t.id AND l.deleted_root_id IS NULL
  JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL WHERE t.depth<30)
 SELECT i.id::text,i.title,t.parent_id::text,i.start_date::text,i.due_date::text,i.progress_percent,
 ARRAY(SELECT concat_ws(' ',u.first_name,u.last_name) FROM item_assignees a JOIN users u ON u.id=a.user_id WHERE a.item_id=i.id ORDER BY u.id)
 FROM t JOIN items i ON i.id=t.id ORDER BY t.depth,i.id LIMIT 5001`, project)
	if err != nil {
		writeItemError(w, err)
		return
	}
	all, err := pgx.CollectRows(rows, pgx.RowToStructByPos[ganttSnapshotItem])
	if err != nil {
		writeItemError(w, err)
		return
	}
	if len(all) > 5000 {
		writeItemError(w, items.Invalid("a snapshot can hold at most 5000 items"))
		return
	}
	ids := make([]string, len(all))
	for i := range all {
		ids[i] = all[i].ID
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		writeItemError(w, err)
		return
	}
	visible := make([]ganttSnapshotItem, 0, len(all))
	for _, item := range all {
		if caps[item.ID].Has(policy.View) {
			if !body.ShowAssignees {
				item.Assignees = nil
			}
			visible = append(visible, item)
		}
	}
	payload, err := json.Marshal(ganttSnapshotPayload{ProjectID: project.String(), ProjectTitle: title, Items: visible})
	if err != nil {
		writeItemError(w, err)
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		writeItemError(w, err)
		return
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := snapshotHash(token)
	id, err := uuid.NewV7()
	if err != nil {
		writeItemError(w, err)
		return
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantt_snapshots(id,project_id,name,token_hash,created_by,show_assignees,expires_at,payload)
 VALUES($1,$2,$3,$4,$5::uuid,$6,$7,$8)`, id, project, name, hash[:], actor.ID, body.ShowAssignees, body.ExpiresAt, payload); err != nil {
		writeItemError(w, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ganttSnapshotBody{ID: id.String(), ProjectID: project.String(), Name: name, ShowAssignees: body.ShowAssignees, ExpiresAt: body.ExpiresAt, CreatedAt: now, Token: token})
}

func (s *Server) getPublicGanttSnapshot(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if wait := s.publicLimiter.take("read:" + s.clientIP(r)); wait > 0 {
		writeRateLimit(w, wait)
		return
	}
	if len(token) != 43 {
		writeItemError(w, items.ErrNotFound)
		return
	}
	hash := snapshotHash(token)
	var payload json.RawMessage
	var project string
	var creator, creatorType *string
	err := s.pool.QueryRow(r.Context(), `SELECT g.payload, g.project_id::text, u.id::text, u.user_type::text
FROM gantt_snapshots g LEFT JOIN users u ON u.id=g.created_by AND u.status='active'
WHERE g.token_hash=$1 AND g.expires_at>now()`, hash[:]).Scan(&payload, &project, &creator, &creatorType)
	// The token alone is not enough: the person who created the snapshot must still be able to view its project (B3-17).
	// A deleted or deactivated creator, or one whose Customer access has ended, makes the link return not found.
	if err == nil {
		if creator == nil || creatorType == nil {
			err = items.ErrNotFound
		} else {
			caps, cerr := policy.ItemCaps(r.Context(), s.pool, policy.Actor{ID: *creator, Type: policy.UserType(*creatorType)}, project)
			if cerr != nil {
				err = cerr
			} else if !caps.Has(policy.View) {
				err = items.ErrNotFound
			}
		}
	}
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

// revokeGanttSnapshot kills the link at once; the people who got it see the same 404 as for an expired one.
func (s *Server) revokeGanttSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("snapshotId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var project uuid.UUID
	if err := tx.QueryRow(ctx, "SELECT project_id FROM gantt_snapshots WHERE id=$1 FOR UPDATE", id).Scan(&project); err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	if _, err := approvalCaps(ctx, tx, actorOf(r), project, policy.Edit); err != nil {
		writeItemError(w, err)
		return
	}
	if _, err := tx.Exec(ctx, "DELETE FROM gantt_snapshots WHERE id=$1", id); err != nil {
		writeItemError(w, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeItemError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
