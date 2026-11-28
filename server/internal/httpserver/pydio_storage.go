package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/pydio"
	"github.com/anissaefuddin/pms/server/internal/pydiostore"
	"github.com/anissaefuddin/pms/server/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Pydio storage settings and project folders, administrators only. Turning storage on, or choosing a folder while it is on,
// first writes, reads and deletes a file there: a setting that cannot work is never saved.

const (
	auditPydioStorage = "pydio.storage_configured"
	auditPydioFolder  = "pydio.project_folder_changed"
)

func (s *Server) pydioStorageRoutes(mux *http.ServeMux) {
	admin := func(h http.HandlerFunc) http.HandlerFunc { return s.requireUser(requireAdmin(h)) }
	mux.HandleFunc("GET /api/v1/pydio/storage", admin(s.getPydioStorage))
	mux.HandleFunc("PUT /api/v1/pydio/storage", admin(s.putPydioStorage))
	mux.HandleFunc("POST /api/v1/pydio/storage/test", admin(s.testPydioStorage))
	mux.HandleFunc("GET /api/v1/items/{itemId}/pydio-folder", s.requireUser(s.getPydioFolder))
	mux.HandleFunc("PUT /api/v1/items/{itemId}/pydio-folder", s.requireUser(s.putPydioFolder))
	mux.HandleFunc("DELETE /api/v1/items/{itemId}/pydio-folder", s.requireUser(s.deletePydioFolder))
}

type pydioStorageBody struct {
	Enabled       bool              `json:"enabled"`
	BaseURL       string            `json:"base_url"`
	Root          string            `json:"root"`
	CustomerRoots map[string]string `json:"customer_roots"`
	CodeFieldID   *string           `json:"code_field_id"`
}

type pydioStorageView struct {
	pydioStorageBody
	Version       int  `json:"version"`
	TokenSet      bool `json:"token_set"` // the token itself is never returned
	KeyConfigured bool `json:"key_configured"`
}

func (s *Server) getPydioStorage(w http.ResponseWriter, r *http.Request) {
	saved, found, err := pydiostore.Load(r.Context(), s.pool)
	if err != nil {
		adminWriteError(w, err)
		return
	}
	view := pydioStorageView{KeyConfigured: len(s.cfg.ConnectorKey) == 32}
	if found {
		view.pydioStorageBody = pydioStorageBody{saved.Enabled, saved.BaseURL, saved.Root, saved.CustomerRoots, saved.CodeFieldID}
		view.Version, view.TokenSet = saved.Version, saved.TokenEnc != ""
	}
	if view.CustomerRoots == nil {
		view.CustomerRoots = map[string]string{}
	}
	writeJSON(w, http.StatusOK, view)
}

// storageFolder trims a folder and refuses what the adapter could not use.
func storageFolder(raw string) (string, bool) {
	folder := strings.Trim(strings.TrimSpace(raw), "/")
	return folder, len(folder) <= 400 && config.CheckPydioRoot(folder) && storage.CellsPathOK(folder)
}

// within reports whether folder is root or lies inside it.
func within(folder, root string) bool {
	return folder == root || strings.HasPrefix(folder, root+"/")
}

const storageFolderRule = "must be a workspace path such as pms-files/clients, using letters, digits, dot, dash and underscore"

func (b *pydioStorageBody) normalize() error {
	b.BaseURL = strings.TrimRight(strings.TrimSpace(b.BaseURL), "/")
	if len(b.BaseURL) > 500 || !config.CheckPydioOrigin(b.BaseURL) {
		return errors.New("base_url must be an https address without credentials or a path")
	}
	var ok bool
	if b.Root, ok = storageFolder(b.Root); !ok {
		return errors.New("root " + storageFolderRule)
	}
	if len(b.CustomerRoots) > maxPydioCustomers {
		return errors.New("too many Customer folders")
	}
	cleaned := map[string]string{}
	for code, raw := range b.CustomerRoots {
		folder, ok := storageFolder(raw)
		if !ok {
			return errors.New("a Customer folder " + storageFolderRule)
		}
		for other, taken := range cleaned {
			if within(folder, taken) || within(taken, folder) {
				return errors.New("the folders of Customer " + other + " and " + code + " overlap")
			}
		}
		cleaned[code] = folder
	}
	b.CustomerRoots = cleaned
	if b.CodeFieldID != nil {
		if _, err := uuid.Parse(*b.CodeFieldID); err != nil {
			return errors.New("code_field_id must be the id of a text custom field")
		}
	}
	return nil
}

// checkFolders proves that the account can write, read and delete in every folder.
func checkFolders(client storage.Cells, folders ...string) error {
	for _, folder := range folders {
		key := folder + "/_pms-check/" + uuid.NewString() + "/check.txt"
		const text = "PMS storage check"
		if _, err := client.PutAt(key, strings.NewReader(text), 1<<10); err != nil {
			return err
		}
		file, err := client.Open(key)
		if err == nil {
			buf := make([]byte, len(text))
			if _, rerr := file.ReadAt(buf, 0); string(buf) != text {
				err = errors.Join(storage.ErrUnavailable, rerr)
			}
			file.Close()
		}
		if removeErr := client.Remove(key); err == nil {
			err = removeErr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func storageFolders(s pydiostore.Settings) []string {
	folders := []string{s.Root}
	for _, folder := range s.CustomerRoots {
		folders = append(folders, folder)
	}
	slices.Sort(folders)
	return slices.Compact(folders)
}

func (s *Server) writeStorageCheckError(w http.ResponseWriter, r *http.Request, err error) {
	slog.WarnContext(r.Context(), "pydio storage check", "error", err)
	writeProblem(w, http.StatusUnprocessableEntity, "pydio_storage_check_failed", "Pydio did not accept a test file: check the address, the token and the folder permissions")
}

func (s *Server) putPydioStorage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version *int `json:"version"`
		pydioStorageBody
		Token string `json:"token"` // write only; empty keeps the saved one
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Version == nil || *in.Version < 0 {
		writeError(w, http.StatusUnprocessableEntity, "version is required: 0 to create, otherwise the version you edited")
		return
	}
	if err := in.pydioStorageBody.normalize(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	ctx := r.Context()
	saved, found, err := pydiostore.Load(ctx, s.pool)
	if err != nil {
		adminWriteError(w, err)
		return
	}
	if (found && saved.Version != *in.Version) || (!found && *in.Version != 0) {
		writeProblem(w, http.StatusConflict, "version_conflict", "the settings were changed by someone else; reload and try again")
		return
	}
	tokenEnc := saved.TokenEnc
	if token := strings.TrimSpace(in.Token); token != "" {
		if len(token) > 4000 {
			writeError(w, http.StatusUnprocessableEntity, "token is too long")
			return
		}
		if tokenEnc, err = pydiostore.Seal(s.cfg, token); err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "key_not_configured", "the Pydio token needs PMS_CONNECTOR_KEY on the server")
			return
		}
	} else if !found || saved.BaseURL != in.BaseURL {
		// A saved token goes only to the address it was saved with.
		writeError(w, http.StatusUnprocessableEntity, "enter the token: it is required for a new address")
		return
	}
	next := pydiostore.Settings{Enabled: in.Enabled, BaseURL: in.BaseURL, Root: in.Root, TokenEnc: tokenEnc, CustomerRoots: in.CustomerRoots, CodeFieldID: in.CodeFieldID}
	codes := make([]string, 0, len(in.CustomerRoots))
	for code := range in.CustomerRoots {
		codes = append(codes, code)
	}
	var unknown []string
	var fieldOK bool
	if err := s.pool.QueryRow(ctx, `SELECT ARRAY(SELECT c FROM unnest($1::text[]) AS c WHERE NOT EXISTS (SELECT 1 FROM clients WHERE code=c) ORDER BY c),
$2::uuid IS NULL OR EXISTS (SELECT 1 FROM custom_fields WHERE id=$2::uuid AND type='text' AND archived_at IS NULL)`, codes, in.CodeFieldID).Scan(&unknown, &fieldOK); err != nil {
		adminWriteError(w, err)
		return
	}
	if len(unknown) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "unknown Customer code: "+strings.Join(unknown, ", "))
		return
	}
	if !fieldOK {
		writeError(w, http.StatusUnprocessableEntity, "code_field_id must be the id of a text custom field")
		return
	}
	if in.Enabled {
		client, err := next.Client(s.cfg)
		if err == nil {
			_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(2 * time.Minute))
			err = checkFolders(client, storageFolders(next)...)
		}
		if err != nil {
			s.writeStorageCheckError(w, r, err)
			return
		}
	}
	customers, err := json.Marshal(in.CustomerRoots)
	if err != nil {
		adminWriteError(w, err)
		return
	}
	err = s.adminTx(r, func(ctx context.Context, tx pgx.Tx) error {
		query := `INSERT INTO pydio_storage(id,enabled,base_url,root,token_enc,customer_roots,code_field_id,updated_by)
VALUES (1,$1,$2,$3,$4,$5,$6::uuid,$7::uuid) ON CONFLICT (id) DO NOTHING`
		args := []any{in.Enabled, in.BaseURL, in.Root, tokenEnc, customers, in.CodeFieldID, actorOf(r).ID}
		if *in.Version > 0 {
			query = `UPDATE pydio_storage SET enabled=$1,base_url=$2,root=$3,token_enc=$4,customer_roots=$5,code_field_id=$6::uuid,updated_by=$7::uuid,updated_at=now(),version=version+1 WHERE id=1 AND version=$8`
			args = append(args, *in.Version)
		}
		tag, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return adminError{http.StatusConflict, "version_conflict", "the settings were changed by someone else; reload and try again"}
		}
		return s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditPydioStorage, TargetType: "pydio", TargetID: "storage",
			Details: map[string]any{"enabled": in.Enabled, "base_url": in.BaseURL, "root": in.Root, "customer_roots": in.CustomerRoots,
				"code_field_id": in.CodeFieldID, "token_changed": strings.TrimSpace(in.Token) != ""}})
	})
	if err != nil {
		adminWriteError(w, err)
		return
	}
	s.getPydioStorage(w, r)
}

// testPydioStorage repeats the write, read and delete check with the saved settings.
func (s *Server) testPydioStorage(w http.ResponseWriter, r *http.Request) {
	saved, found, err := pydiostore.Load(r.Context(), s.pool)
	if err != nil {
		adminWriteError(w, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "no Pydio storage settings are saved")
		return
	}
	client, err := saved.Client(s.cfg)
	if err == nil {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(2 * time.Minute))
		err = checkFolders(client, storageFolders(saved)...)
	}
	if err != nil {
		s.writeStorageCheckError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// pydioItem reads the path item with the caller's own access, then requires the administrator role: the role alone opens no
// content (A1), and someone without access learns nothing about the item.
func (s *Server) pydioItem(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return id, false
	}
	if _, err := items.Get(r.Context(), s.pool, actorOf(r), id.String()); err != nil {
		writeItemError(w, err)
		return id, false
	}
	if actorOf(r).Type != policy.Admin {
		adminWriteError(w, noAdmin)
		return id, false
	}
	return id, true
}

func (s *Server) getPydioFolder(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pydioItem(w, r)
	if !ok {
		return
	}
	saved, found, err := pydiostore.Load(r.Context(), s.pool)
	if err == nil && !found {
		writeProblem(w, http.StatusNotFound, "pydio_storage_not_configured", "Pydio storage is not configured")
		return
	}
	var place pydiostore.Place
	if err == nil {
		place, err = pydiostore.FolderOf(r.Context(), s.pool, saved, id)
	}
	if err != nil {
		adminWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		pydiostore.Place
		Enabled bool `json:"enabled"`
	}{place, saved.Enabled})
}

func (s *Server) putPydioFolder(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pydioItem(w, r)
	if !ok {
		return
	}
	var in struct {
		Folder string `json:"folder"`
	}
	if !decode(w, r, &in) {
		return
	}
	folder, valid := storageFolder(in.Folder)
	if !valid {
		writeError(w, http.StatusUnprocessableEntity, "folder "+storageFolderRule)
		return
	}
	ctx := r.Context()
	saved, found, err := pydiostore.Load(ctx, s.pool)
	if err != nil {
		adminWriteError(w, err)
		return
	}
	if !found {
		writeProblem(w, http.StatusUnprocessableEntity, "pydio_storage_not_configured", "save the Pydio storage settings first")
		return
	}
	var project bool
	var customer string
	if err := s.pool.QueryRow(ctx, `SELECT i.type='project', COALESCE(c.code,'') FROM items i
LEFT JOIN clients c ON c.id=i.client_id AND i.scope_state='client' WHERE i.id=$1`, id).Scan(&project, &customer); err != nil {
		adminWriteError(w, err)
		return
	}
	if !project {
		writeError(w, http.StatusUnprocessableEntity, "only a project has its own folder")
		return
	}
	for code, root := range saved.CustomerRoots {
		if code != customer && (within(folder, root) || within(root, folder)) {
			writeError(w, http.StatusUnprocessableEntity, "the folder belongs to another Customer")
			return
		}
	}
	if within(saved.Root, folder) {
		writeError(w, http.StatusUnprocessableEntity, "the folder must not contain the storage root")
		return
	}
	if saved.Enabled {
		client, err := saved.Client(s.cfg)
		if err == nil {
			err = checkFolders(client, folder)
		}
		if err != nil {
			s.writeStorageCheckError(w, r, err)
			return
		}
	}
	err = s.adminTx(r, func(ctx context.Context, tx pgx.Tx) error {
		var taken bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pydio_project_folders WHERE item_id<>$1
AND (folder=$2 OR starts_with(folder,$2||'/') OR starts_with($2,folder||'/')))`, id, folder).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return adminError{http.StatusUnprocessableEntity, "validation_failed", "another project already uses this folder or one inside it"}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO pydio_project_folders(item_id,folder,updated_by) VALUES ($1,$2,$3::uuid)
ON CONFLICT (item_id) DO UPDATE SET folder=$2, updated_by=$3::uuid, updated_at=now()`, id, folder, actorOf(r).ID); err != nil {
			return err
		}
		return s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditPydioFolder, TargetType: "item", TargetID: id.String(), Details: map[string]any{"folder": folder}})
	})
	if err != nil {
		adminWriteError(w, err)
		return
	}
	s.getPydioFolder(w, r)
}

func (s *Server) deletePydioFolder(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pydioItem(w, r)
	if !ok {
		return
	}
	err := s.adminTx(r, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, "DELETE FROM pydio_project_folders WHERE item_id=$1", id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return adminError{http.StatusNotFound, "not_found", "this project has no folder of its own"}
		}
		return s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditPydioFolder, TargetType: "item", TargetID: id.String(), Details: map[string]any{"folder": ""}})
	})
	if err != nil {
		adminWriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// uploadLimits are the administrator's limits for Pydio uploads, or the defaults.
func (s *Server) uploadLimits(ctx context.Context) (pydio.UploadLimits, error) {
	view, err := s.pydioCurrent(ctx)
	return pydio.UploadLimits{MaxMB: view.MaxUploadMB, Extensions: view.AllowedExtensions}, err
}
