package httpserver

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const attachmentJSON = `jsonb_build_object('id',a.id,'item_id',a.item_id,'filename',a.filename,
'content_type',a.content_type,'size_bytes',a.size_bytes,'version',a.current_version,'link_url',a.link_url,'created_at',a.created_at,
'uploader',jsonb_build_object('id',u.id,'name',concat_ws(' ',u.first_name,u.last_name)))`

var errBlockedArchive = errors.New("archive contains an executable or cannot be inspected")

func (s *Server) attachmentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/items/{itemId}/attachments", s.requireUser(s.listAttachments))
	mux.HandleFunc("POST /api/v1/items/{itemId}/attachments", s.requireUser(s.uploadAttachment))
	mux.HandleFunc("POST /api/v1/items/{itemId}/attachment-links", s.requireUser(s.createAttachmentLink))
	mux.HandleFunc("GET /api/v1/attachments/{attachmentId}/download", s.requireUser(s.attachmentResource))
	mux.HandleFunc("DELETE /api/v1/attachments/{attachmentId}", s.requireUser(s.attachmentResource))
	mux.HandleFunc("GET /api/v1/attachments/{attachmentId}/versions", s.requireUser(s.listAttachmentVersions))
	mux.HandleFunc("POST /api/v1/attachments/{attachmentId}/versions", s.requireUser(s.uploadAttachment))
	mux.HandleFunc("GET /api/v1/attachments/{attachmentId}/versions/{version}/download", s.requireUser(s.attachmentResource))
	s.proofingRoutes(mux)
}

func (s *Server) attachmentStore() storage.Storage {
	return storage.FromConfig(s.cfg)
}

func attachmentAccess(r *http.Request, db policy.DB, id uuid.UUID, need policy.Capability) (policy.Caps, error) {
	caps, err := policy.ItemCaps(r.Context(), db, actorOf(r), id.String())
	if err != nil {
		return 0, err
	}
	if !caps.Has(policy.View) {
		return 0, items.ErrNotFound
	}
	if !caps.Has(need) {
		return 0, items.ErrForbidden
	}
	return caps, nil
}

func (s *Server) listAttachments(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err = attachmentAccess(r, tx, id, policy.View); err != nil {
		writeItemError(w, err)
		return
	}
	rows, err := tx.Query(ctx, "SELECT "+attachmentJSON+" FROM attachments a JOIN users u ON u.id=a.uploader_id WHERE a.item_id=$1 AND a.deleted_at IS NULL ORDER BY a.created_at,a.id", id)
	if err != nil {
		writeItemError(w, err)
		return
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[json.RawMessage])
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	if out == nil {
		out = []json.RawMessage{}
	}
	writeJSON(w, 200, map[string]any{"items": out, "max_upload_bytes": s.attachmentLimit()})
}

func (s *Server) attachmentLimit() int64 {
	if s.cfg.MaxUploadMB <= 0 {
		return 100 << 20
	}
	return int64(s.cfg.MaxUploadMB) << 20
}

// uploadAttachment serves both POST /items/{id}/attachments (new file, version 1) and
// POST /attachments/{id}/versions (next version of an existing file).
func (s *Server) uploadAttachment(w http.ResponseWriter, r *http.Request) {
	var existing *uuid.UUID
	id, err := uuid.Parse(r.PathValue("itemId"))
	if raw := r.PathValue("attachmentId"); raw != "" {
		var aid uuid.UUID
		if aid, err = uuid.Parse(raw); err == nil {
			existing = &aid
			var link *string
			err = s.pool.QueryRow(r.Context(), "SELECT item_id,link_url FROM attachments WHERE id=$1 AND deleted_at IS NULL", aid).Scan(&id, &link)
			if errors.Is(err, pgx.ErrNoRows) {
				err = items.ErrNotFound
			} else if err == nil && link != nil {
				err = items.Invalid("a link attachment has no file versions")
			}
		}
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	if _, err = attachmentAccess(r, s.pool, id, policy.Attach); err != nil {
		writeItemError(w, err)
		return
	}
	max := s.attachmentLimit()
	controller := http.NewResponseController(w)
	now := time.Now()
	_ = controller.SetReadDeadline(now.Add(5 * time.Minute))
	_ = controller.SetWriteDeadline(now.Add(6 * time.Minute))
	r.Body = http.MaxBytesReader(w, r.Body, max+(64<<10))
	reader, err := r.MultipartReader()
	if err != nil {
		writeItemError(w, items.Invalid("multipart file is required"))
		return
	}
	part, err := reader.NextPart()
	if err != nil {
		writeAttachmentUploadError(w, err)
		return
	}
	if part.FormName() != "file" {
		writeItemError(w, items.Invalid("valid attachment filename is required"))
		return
	}
	staged, err := s.stageAttachment(part, max)
	if err != nil {
		writeAttachmentUploadError(w, err)
		return
	}
	store := s.attachmentStore()
	stored := false
	defer func() {
		if !stored {
			_ = store.Remove(staged.File.Key)
		}
	}()
	if _, err = reader.NextPart(); !errors.Is(err, io.EOF) {
		if err == nil {
			err = items.Invalid("exactly one file is allowed")
		}
		writeAttachmentUploadError(w, err)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err = attachmentAccess(r, tx, id, policy.Attach); err != nil {
		writeItemError(w, err)
		return
	}
	var attachment uuid.UUID
	if existing != nil {
		attachment = *existing
		err = insertAttachmentVersion(ctx, tx, attachment, actorOf(r).ID, staged)
		if errors.Is(err, pgx.ErrNoRows) {
			err = items.ErrNotFound
		}
	} else {
		attachment, err = insertAttachment(ctx, tx, id, actorOf(r).ID, staged)
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	var out json.RawMessage
	err = tx.QueryRow(ctx, "SELECT "+attachmentJSON+" FROM attachments a JOIN users u ON u.id=a.uploader_id WHERE a.id=$1", attachment).Scan(&out)
	if err == nil {
		var current int
		if err = tx.QueryRow(ctx, "SELECT current_version FROM attachments WHERE id=$1", attachment).Scan(&current); err == nil {
			err = s.ensurePreview(ctx, tx, attachment, current)
		}
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	stored = true
	s.publishItem(r, "item.updated", id.String(), id.String(), 0)
	writeJSON(w, 201, out)
}

// createAttachmentLink records a URL as an attachment. Same Attach permission as an upload; only http(s) without credentials.
func (s *Server) createAttachmentLink(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	var in struct{ URL, Title string }
	if err = json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in); err != nil {
		writeItemError(w, items.Invalid("url is required"))
		return
	}
	u, perr := url.Parse(strings.TrimSpace(in.URL))
	title := strings.TrimSpace(in.Title)
	if perr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || len(u.String()) > 2048 {
		writeItemError(w, items.Invalid("url must be an http(s) address without credentials"))
		return
	}
	if title == "" {
		title = u.Host
	}
	if len(title) > 255 || strings.ContainsFunc(title, unicode.IsControl) {
		writeItemError(w, items.Invalid("title is invalid"))
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err = attachmentAccess(r, tx, id, policy.Attach); err != nil {
		writeItemError(w, err)
		return
	}
	aid, err := uuid.NewV7()
	if err != nil {
		writeItemError(w, err)
		return
	}
	sum := sha256.Sum256([]byte(u.String()))
	f := stagedFile{Filename: title, Media: "text/uri-list", File: storage.File{Key: "link:" + aid.String(), Size: int64(len(u.String())), SHA256: hex.EncodeToString(sum[:])}}
	_, err = tx.Exec(ctx, `WITH a AS (INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256,link_url)
VALUES($1,$2,$3::uuid,$4,$5,$6,$7,decode($8,'hex'),$9))
INSERT INTO attachment_versions(attachment_id,version,uploader_id,filename,content_type,size_bytes,storage_key,sha256)
VALUES($1,1,$3::uuid,$4,$5,$6,$7,decode($8,'hex'))`, aid, id, actorOf(r).ID, f.Filename, f.Media, f.File.Size, f.File.Key, f.File.SHA256, u.String())
	var out json.RawMessage
	if err == nil {
		err = tx.QueryRow(ctx, "SELECT "+attachmentJSON+" FROM attachments a JOIN users u ON u.id=a.uploader_id WHERE a.id=$1", aid).Scan(&out)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	s.publishItem(r, "item.updated", id.String(), id.String(), 0)
	writeJSON(w, 201, out)
}

// blockedFile is an upload the checks refuse; it is answered with 422 file_type_blocked.
type blockedFile struct{ message string }

func (b blockedFile) Error() string { return b.message }

// stagedFile is an upload that passed every attachment check and sits in storage but is not recorded yet.
// The caller must record it in the database or remove File.Key.
type stagedFile struct {
	Filename, Media string
	File            storage.File
}

// stageAttachment applies the attachment rules (name, executable and type checks, size limit, ZIP inspection)
// to one multipart file and stores it. Normal uploads and form submissions both use it.
func (s *Server) stageAttachment(part *multipart.Part, max int64) (stagedFile, error) {
	filename := strings.TrimSpace(path.Base(strings.ReplaceAll(part.FileName(), "\\", "/")))
	if filename == "" || filename == "." || len(filename) > 255 || strings.ContainsFunc(filename, unicode.IsControl) {
		return stagedFile{}, items.Invalid("valid attachment filename is required")
	}
	source := bufio.NewReader(part)
	header, err := source.Peek(512)
	if err != nil && !errors.Is(err, io.EOF) {
		return stagedFile{}, err
	}
	if len(header) == 0 {
		return stagedFile{}, items.Invalid("attachment is empty")
	}
	media := http.DetectContentType(header)
	if executableAttachment(filename, header) {
		return stagedFile{}, blockedFile{"executable attachments are not allowed"}
	}
	switch media {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "application/pdf", "application/zip", "text/plain; charset=utf-8":
	default:
		return stagedFile{}, blockedFile{"supported files are PDF, PNG, JPEG, GIF, WebP, plain text, and ZIP"}
	}
	store := s.attachmentStore()
	file, err := store.Put(source, max)
	if err != nil {
		return stagedFile{}, err
	}
	if media == "application/zip" {
		if err := validateAttachmentZIP(store, file); err != nil {
			_ = store.Remove(file.Key)
			if errors.Is(err, errBlockedArchive) {
				return stagedFile{}, blockedFile{err.Error()}
			}
			return stagedFile{}, err
		}
	}
	return stagedFile{Filename: filename, Media: media, File: file}, nil
}

func insertAttachment(ctx context.Context, tx pgx.Tx, itemID uuid.UUID, uploader string, f stagedFile) (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return id, err
	}
	_, err = tx.Exec(ctx, `WITH a AS (INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256)
VALUES($1,$2,$3::uuid,$4,$5,$6,$7,decode($8,'hex')))
INSERT INTO attachment_versions(attachment_id,version,uploader_id,filename,content_type,size_bytes,storage_key,sha256)
VALUES($1,1,$3::uuid,$4,$5,$6,$7,decode($8,'hex'))`, id, itemID, uploader, f.Filename, f.Media, f.File.Size, f.File.Key, f.File.SHA256)
	return id, err
}

// insertAttachmentVersion appends the next version and moves the attachment row to it. ErrNoRows: attachment gone.
func insertAttachmentVersion(ctx context.Context, tx pgx.Tx, id uuid.UUID, uploader string, f stagedFile) error {
	var next int
	err := tx.QueryRow(ctx, "SELECT current_version+1 FROM attachments WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", id).Scan(&next)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO attachment_versions(attachment_id,version,uploader_id,filename,content_type,size_bytes,storage_key,sha256)
VALUES($1,$2,$3::uuid,$4,$5,$6,$7,decode($8,'hex'))`, id, next, uploader, f.Filename, f.Media, f.File.Size, f.File.Key, f.File.SHA256); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE attachments SET current_version=$2,filename=$3,content_type=$4,size_bytes=$5,storage_key=$6,sha256=decode($7,'hex') WHERE id=$1`,
		id, next, f.Filename, f.Media, f.File.Size, f.File.Key, f.File.SHA256)
	return err
}

// listAttachmentVersions returns every version, newest first. Visible to anyone who can view the item.
func (s *Server) listAttachmentVersions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("attachmentId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	ctx := r.Context()
	var item uuid.UUID
	err = s.pool.QueryRow(ctx, "SELECT item_id FROM attachments WHERE id=$1 AND deleted_at IS NULL", id).Scan(&item)
	if errors.Is(err, pgx.ErrNoRows) {
		err = items.ErrNotFound
	}
	if err == nil {
		_, err = attachmentAccess(r, s.pool, item, policy.View)
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	rows, err := s.pool.Query(ctx, `SELECT jsonb_build_object('attachment_id',v.attachment_id,'version',v.version,'filename',v.filename,
'content_type',v.content_type,'size_bytes',v.size_bytes,'created_at',v.created_at,'is_current',v.version=a.current_version,
'uploader',jsonb_build_object('id',u.id,'name',concat_ws(' ',u.first_name,u.last_name)))
FROM attachment_versions v JOIN attachments a ON a.id=v.attachment_id JOIN users u ON u.id=v.uploader_id
WHERE v.attachment_id=$1 ORDER BY v.version DESC`, id)
	if err != nil {
		writeItemError(w, err)
		return
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[json.RawMessage])
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func executableAttachment(name string, prefix []byte) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".exe", ".com", ".dll", ".msi", ".scr", ".bat", ".cmd", ".ps1", ".sh", ".bash", ".zsh", ".fish", ".app", ".run", ".jar", ".elf", ".so", ".dylib", ".vbs":
		return true
	}
	prefix = bytes.TrimSpace(bytes.TrimPrefix(prefix, []byte("\xef\xbb\xbf")))
	if bytes.HasPrefix(prefix, []byte("#!")) || bytes.HasPrefix(prefix, []byte("MZ")) {
		return true
	}
	if len(prefix) >= 4 {
		switch string(prefix[:4]) {
		case "\x7fELF", "\xfe\xed\xfa\xce", "\xce\xfa\xed\xfe", "\xfe\xed\xfa\xcf", "\xcf\xfa\xed\xfe", "\xca\xfe\xba\xbe", "\xbe\xba\xfe\xca", "\xca\xfe\xba\xbf", "\xbf\xba\xfe\xca":
			return true
		}
	}
	return false
}

func validateAttachmentZIP(store storage.Storage, file storage.File) error {
	reader, err := store.Open(file.Key)
	if err != nil {
		return err
	}
	defer reader.Close()
	archive, err := zip.NewReader(reader, file.Size)
	if err != nil || len(archive.File) > 10000 {
		return errBlockedArchive
	}
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		if !entry.Mode().IsRegular() || entry.Flags&1 != 0 {
			return errBlockedArchive
		}
		source, err := entry.Open()
		if err != nil {
			return errBlockedArchive
		}
		var prefix [512]byte
		n, err := io.ReadFull(source, prefix[:])
		source.Close()
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF || executableAttachment(entry.Name, prefix[:n]) {
			return errBlockedArchive
		}
	}
	return nil
}

func writeAttachmentUploadError(w http.ResponseWriter, err error) {
	var max *http.MaxBytesError
	var blocked blockedFile
	switch {
	case errors.As(err, &blocked):
		writeProblem(w, 422, "file_type_blocked", blocked.message)
	case errors.Is(err, storage.ErrTooLarge), errors.As(err, &max):
		writeProblem(w, 413, "file_too_large", "attachment exceeds upload limit")
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		writeItemError(w, items.Invalid("incomplete multipart file"))
	default:
		writeItemError(w, err)
	}
}

func (s *Server) attachmentResource(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("attachmentId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var item uuid.UUID
	var uploader, key, filename string
	var link *string
	var size int64
	version := r.PathValue("version") // empty = current
	n := 0
	if version != "" {
		if n, err = strconv.Atoi(version); err != nil || n < 1 {
			writeItemError(w, items.ErrNotFound)
			return
		}
	}
	err = tx.QueryRow(ctx, `SELECT a.item_id,a.uploader_id::text,coalesce(v.storage_key,a.storage_key),coalesce(v.filename,a.filename),coalesce(v.size_bytes,a.size_bytes),a.link_url
FROM attachments a LEFT JOIN attachment_versions v ON v.attachment_id=a.id AND v.version=$2
WHERE a.id=$1 AND a.deleted_at IS NULL AND ($2=0 OR v.version IS NOT NULL)`, id, n).Scan(&item, &uploader, &key, &filename, &size, &link)
	if errors.Is(err, pgx.ErrNoRows) {
		err = items.ErrNotFound
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	caps, err := attachmentAccess(r, tx, item, policy.View)
	if errors.Is(err, items.ErrNotFound) && r.Method == http.MethodGet && n > 0 && fileReviewer(ctx, tx, actorOf(r), id, n) {
		err = nil // FIL-04: external reviewer of exactly this version
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	if r.Method == http.MethodDelete {
		if !caps.Has(policy.Delete) && !(uploader == actorOf(r).ID && caps.Has(policy.Attach)) {
			writeItemError(w, items.ErrForbidden)
			return
		}
		_, err = tx.Exec(ctx, "UPDATE attachments SET deleted_at=now() WHERE id=$1", id)
		if err == nil {
			err = tx.Commit(ctx)
		}
		if err != nil {
			writeItemError(w, err)
			return
		}
		s.publishItem(r, "item.updated", item.String(), item.String(), 0)
		w.WriteHeader(204)
		return
	}
	if link != nil {
		writeItemError(w, items.Invalid("a link attachment has no file to download; open "+*link))
		return
	}
	file, err := s.attachmentStore().Open(key)
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer file.Close()
	if err := tx.Commit(ctx); err != nil {
		writeItemError(w, err)
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Minute))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	_, _ = io.Copy(w, file)
}
