package httpserver

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
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
'content_type',a.content_type,'size_bytes',a.size_bytes,'created_at',a.created_at,
'uploader',jsonb_build_object('id',u.id,'name',concat_ws(' ',u.first_name,u.last_name)))`

var errBlockedArchive = errors.New("archive contains an executable or cannot be inspected")

func (s *Server) attachmentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/items/{itemId}/attachments", s.requireUser(s.listAttachments))
	mux.HandleFunc("POST /api/v1/items/{itemId}/attachments", s.requireUser(s.uploadAttachment))
	mux.HandleFunc("GET /api/v1/attachments/{attachmentId}/download", s.requireUser(s.attachmentResource))
	mux.HandleFunc("DELETE /api/v1/attachments/{attachmentId}", s.requireUser(s.attachmentResource))
}

func (s *Server) attachmentStore() storage.Storage {
	dir := s.cfg.FilesDir
	if dir == "" {
		dir = "/data/files"
	}
	return storage.LocalStorage{Dir: dir}
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

func (s *Server) uploadAttachment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
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
	filename := strings.TrimSpace(path.Base(strings.ReplaceAll(part.FileName(), "\\", "/")))
	if part.FormName() != "file" || filename == "" || filename == "." || len(filename) > 255 || strings.ContainsFunc(filename, unicode.IsControl) {
		writeItemError(w, items.Invalid("valid attachment filename is required"))
		return
	}
	source := bufio.NewReader(part)
	header, err := source.Peek(512)
	if err != nil && !errors.Is(err, io.EOF) {
		writeAttachmentUploadError(w, err)
		return
	}
	media := http.DetectContentType(header)
	if len(header) == 0 {
		writeItemError(w, items.Invalid("attachment is empty"))
		return
	}
	if executableAttachment(filename, header) {
		writeProblem(w, 422, "file_type_blocked", "executable attachments are not allowed")
		return
	}
	switch media {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "application/pdf", "application/zip", "text/plain; charset=utf-8":
	default:
		writeProblem(w, 422, "file_type_blocked", "supported files are PDF, PNG, JPEG, GIF, WebP, plain text, and ZIP")
		return
	}
	store := s.attachmentStore()
	file, err := store.Put(source, max)
	if err != nil {
		writeAttachmentUploadError(w, err)
		return
	}
	stored := false
	defer func() {
		if !stored {
			_ = store.Remove(file.Key)
		}
	}()
	if media == "application/zip" {
		if err := validateAttachmentZIP(store, file); errors.Is(err, errBlockedArchive) {
			writeProblem(w, 422, "file_type_blocked", err.Error())
			return
		} else if err != nil {
			writeItemError(w, err)
			return
		}
	}
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
	attachment, err := uuid.NewV7()
	if err != nil {
		writeItemError(w, err)
		return
	}
	if _, err = tx.Exec(ctx, `INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256)
VALUES($1,$2,$3::uuid,$4,$5,$6,$7,decode($8,'hex'))`, attachment, id, actorOf(r).ID, filename, media, file.Size, file.Key, file.SHA256); err != nil {
		writeItemError(w, err)
		return
	}
	var out json.RawMessage
	err = tx.QueryRow(ctx, "SELECT "+attachmentJSON+" FROM attachments a JOIN users u ON u.id=a.uploader_id WHERE a.id=$1", attachment).Scan(&out)
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
	switch {
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
	var size int64
	err = tx.QueryRow(ctx, "SELECT item_id,uploader_id::text,storage_key,filename,size_bytes FROM attachments WHERE id=$1 AND deleted_at IS NULL", id).Scan(&item, &uploader, &key, &filename, &size)
	if errors.Is(err, pgx.ErrNoRows) {
		err = items.ErrNotFound
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	caps, err := attachmentAccess(r, tx, item, policy.View)
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
