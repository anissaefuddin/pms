package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/previews"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Preview and proofing of one file version (FIL-02, D-80). Everything is keyed by (attachment, version) and
// authorised through the owning item, like downloads.
const annotationJSON = `jsonb_build_object('id',n.id,'attachment_id',n.attachment_id,'version',n.version,'page',n.page,
'x',n.x::float8,'y',n.y::float8,'body',n.body,'resolved',n.resolved_at IS NOT NULL,'created_at',n.created_at,
'author',jsonb_build_object('id',u.id,'name',concat_ws(' ',u.first_name,u.last_name)))`

func (s *Server) proofingRoutes(mux *http.ServeMux) {
	const base = "/api/v1/attachments/{attachmentId}/versions/{version}"
	s.compareRoutes(mux)
	mux.HandleFunc("GET "+base+"/preview", s.requireUser(s.getPreview))
	mux.HandleFunc("GET "+base+"/preview/{page}", s.requireUser(s.getPreviewPage))
	mux.HandleFunc("GET "+base+"/annotations", s.requireUser(s.listAnnotations))
	mux.HandleFunc("POST "+base+"/annotations", s.requireUser(s.createAnnotation))
	mux.HandleFunc("PATCH "+base+"/annotations/{annotationId}", s.requireUser(s.changeAnnotation))
	mux.HandleFunc("DELETE "+base+"/annotations/{annotationId}", s.requireUser(s.changeAnnotation))
}

// versionItem resolves {attachmentId}/{version} to the owning item and checks the capability. 404 when the
// file, the version or view access is missing.
func (s *Server) versionItem(r *http.Request, need policy.Capability) (att uuid.UUID, version int, item uuid.UUID, caps policy.Caps, err error) {
	if att, err = uuid.Parse(r.PathValue("attachmentId")); err == nil {
		version, err = strconv.Atoi(r.PathValue("version"))
	}
	return s.versionItemOf(r, att, version, err, need)
}

func (s *Server) versionItemOf(r *http.Request, att uuid.UUID, version int, err error, need policy.Capability) (uuid.UUID, int, uuid.UUID, policy.Caps, error) {
	var item uuid.UUID
	var caps policy.Caps
	if err != nil || version < 1 {
		return att, 0, item, 0, items.ErrNotFound
	}
	err = s.pool.QueryRow(r.Context(), `SELECT a.item_id FROM attachments a JOIN attachment_versions v ON v.attachment_id=a.id AND v.version=$2
WHERE a.id=$1 AND a.deleted_at IS NULL`, att, version).Scan(&item)
	if errors.Is(err, pgx.ErrNoRows) {
		err = items.ErrNotFound
	}
	if err == nil {
		caps, err = attachmentAccess(r, s.pool, item, need)
	}
	return att, version, item, caps, err
}

// ensurePreview records a pending preview for a version and queues the render, once (the primary key makes it idempotent).
func (s *Server) ensurePreview(ctx context.Context, tx pgx.Tx, att uuid.UUID, version int) error {
	tag, err := tx.Exec(ctx, "INSERT INTO attachment_previews(attachment_id,version) VALUES($1,$2) ON CONFLICT DO NOTHING", att, version)
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	return s.mail.PreviewTx(ctx, tx, previews.Args{AttachmentID: att, Version: version})
}

func (s *Server) getPreview(w http.ResponseWriter, r *http.Request) {
	att, version, _, _, err := s.versionItem(r, policy.View)
	if err != nil {
		writeItemError(w, err)
		return
	}
	out, err := s.previewOf(r.Context(), att, version)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

// previewOf is the preview status of one version (queues the render on first use); shared with compare (S11.1.4).
func (s *Server) previewOf(ctx context.Context, att uuid.UUID, version int) (map[string]any, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = s.ensurePreview(ctx, tx, att, version); err != nil {
		return nil, err
	}
	var status, note string
	var raw []byte
	if err = tx.QueryRow(ctx, "SELECT status,note,pages FROM attachment_previews WHERE attachment_id=$1 AND version=$2", att, version).Scan(&status, &note, &raw); err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		return nil, err
	}
	var pages []previews.Page
	_ = json.Unmarshal(raw, &pages)
	out := make([]map[string]int, 0, len(pages)) // storage keys stay server-side
	for _, p := range pages {
		out = append(out, map[string]int{"page": p.Page, "width": p.Width, "height": p.Height})
	}
	return map[string]any{"attachment_id": att, "version": version, "status": status, "note": note, "pages": out}, nil
}

func (s *Server) getPreviewPage(w http.ResponseWriter, r *http.Request) {
	att, version, _, _, err := s.versionItem(r, policy.View)
	page, perr := strconv.Atoi(r.PathValue("page"))
	if err == nil && perr != nil {
		err = items.ErrNotFound
	}
	var key string
	if err == nil {
		err = s.pool.QueryRow(r.Context(), `SELECT p->>'key' FROM attachment_previews, jsonb_array_elements(pages) p
WHERE attachment_id=$1 AND version=$2 AND (p->>'page')::int=$3`, att, version, page).Scan(&key)
		if errors.Is(err, pgx.ErrNoRows) {
			err = items.ErrNotFound
		}
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	file, err := s.attachmentStore().Open(key)
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer file.Close()
	// The body is our own re-encoded PNG, so it may render inline; the other headers keep it inert.
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = io.Copy(w, file)
}

func (s *Server) listAnnotations(w http.ResponseWriter, r *http.Request) {
	att, version, _, _, err := s.versionItem(r, policy.View)
	if err != nil {
		writeItemError(w, err)
		return
	}
	rows, err := s.pool.Query(r.Context(), "SELECT "+annotationJSON+` FROM attachment_annotations n JOIN users u ON u.id=n.author_id
WHERE n.attachment_id=$1 AND n.version=$2 ORDER BY n.created_at,n.id`, att, version)
	if err != nil {
		writeItemError(w, err)
		return
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[json.RawMessage])
	if err != nil {
		writeItemError(w, err)
		return
	}
	if out == nil {
		out = []json.RawMessage{}
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (s *Server) createAnnotation(w http.ResponseWriter, r *http.Request) {
	att, version, item, _, err := s.versionItem(r, policy.Comment)
	if err != nil {
		writeItemError(w, err)
		return
	}
	var raw struct {
		Page *int     `json:"page"`
		X    *float64 `json:"x"`
		Y    *float64 `json:"y"`
		Body string   `json:"body"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err = dec.Decode(&raw); err != nil {
		writeItemError(w, items.Invalid("invalid annotation"))
		return
	}
	body := strings.TrimSpace(raw.Body)
	page := 1
	if raw.Page != nil {
		page = *raw.Page
	}
	if raw.X == nil || raw.Y == nil || *raw.X < 0 || *raw.X > 1 || *raw.Y < 0 || *raw.Y > 1 || page < 1 || body == "" || len([]rune(body)) > 4000 {
		writeItemError(w, items.Invalid("annotation needs x and y between 0 and 1, a page of 1 or more, and 1-4000 characters of text"))
		return
	}
	id, err := uuid.NewV7()
	if err != nil {
		writeItemError(w, err)
		return
	}
	var out json.RawMessage
	err = s.pool.QueryRow(r.Context(), `WITH n AS (INSERT INTO attachment_annotations(id,attachment_id,version,author_id,page,x,y,body)
VALUES($1,$2,$3,$4::uuid,$5,$6,$7,$8) RETURNING *)
SELECT `+annotationJSON+` FROM n JOIN users u ON u.id=n.author_id`, id, att, version, actorOf(r).ID, page, *raw.X, *raw.Y, body).Scan(&out)
	if err != nil {
		writeItemError(w, err)
		return
	}
	s.publishItem(r, "item.updated", item.String(), item.String(), 0)
	writeJSON(w, 201, out)
}

// changeAnnotation resolves/reopens (PATCH {"resolved":bool}, needs comment) or deletes (author, or delete on the item).
func (s *Server) changeAnnotation(w http.ResponseWriter, r *http.Request) {
	att, version, item, caps, err := s.versionItem(r, policy.Comment)
	id, perr := uuid.Parse(r.PathValue("annotationId"))
	if err == nil && perr != nil {
		err = items.ErrNotFound
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	ctx := r.Context()
	var author string
	err = s.pool.QueryRow(ctx, "SELECT author_id::text FROM attachment_annotations WHERE id=$1 AND attachment_id=$2 AND version=$3", id, att, version).Scan(&author)
	if errors.Is(err, pgx.ErrNoRows) {
		err = items.ErrNotFound
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	if r.Method == http.MethodDelete {
		if author != actorOf(r).ID && !caps.Has(policy.Delete) {
			writeItemError(w, items.ErrForbidden)
			return
		}
		_, err = s.pool.Exec(ctx, "DELETE FROM attachment_annotations WHERE id=$1", id)
		if err != nil {
			writeItemError(w, err)
			return
		}
		s.publishItem(r, "item.updated", item.String(), item.String(), 0)
		w.WriteHeader(204)
		return
	}
	var in struct {
		Resolved *bool `json:"resolved"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	if err = json.NewDecoder(r.Body).Decode(&in); err != nil || in.Resolved == nil {
		writeItemError(w, items.Invalid("resolved is required"))
		return
	}
	var out json.RawMessage
	err = s.pool.QueryRow(ctx, `WITH n AS (UPDATE attachment_annotations SET resolved_at=CASE WHEN $2 THEN coalesce(resolved_at,now()) END WHERE id=$1 RETURNING *)
SELECT `+annotationJSON+` FROM n JOIN users u ON u.id=n.author_id`, id, *in.Resolved).Scan(&out)
	if err != nil {
		writeItemError(w, err)
		return
	}
	s.publishItem(r, "item.updated", item.String(), item.String(), 0)
	writeJSON(w, 200, out)
}
