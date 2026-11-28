// Package pydiostore connects the saved Pydio storage settings to the Cells adapter: where a new file of an item goes, and how a
// file recorded in a backend is opened again. The web process and the background workers use the same rules.
package pydiostore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/storage"
	"github.com/anissaefuddin/pms/server/internal/twofactor"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Backend is the value recorded with a file stored in Pydio.
const Backend = "pydio"

// tokenBinding ties the encrypted token to this one setting: copied elsewhere it does not decrypt.
const tokenBinding = "pydio_storage"

type Settings struct {
	Enabled       bool
	BaseURL       string
	Root          string
	TokenEnc      string
	CustomerRoots map[string]string
	CodeFieldID   *string
	Version       int
}

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Load returns the saved settings; found is false when none were ever saved.
func Load(ctx context.Context, db querier) (s Settings, found bool, err error) {
	var customers []byte
	err = db.QueryRow(ctx, "SELECT enabled,base_url,root,token_enc,customer_roots,code_field_id::text,version FROM pydio_storage WHERE id=1").
		Scan(&s.Enabled, &s.BaseURL, &s.Root, &s.TokenEnc, &customers, &s.CodeFieldID, &s.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, false, nil
	}
	if err == nil {
		err = json.Unmarshal(customers, &s.CustomerRoots)
	}
	return s, err == nil, err
}

func keys(cfg config.Config) twofactor.Keys {
	return twofactor.Keys{Current: cfg.ConnectorKey, Previous: cfg.ConnectorKeyPrevious}
}

// Seal encrypts a token for saving.
func Seal(cfg config.Config, token string) (string, error) {
	return keys(cfg).Encrypt(token, tokenBinding)
}

// Client builds the adapter for the saved address and token.
func (s Settings) Client(cfg config.Config) (storage.Cells, error) {
	token, _, err := keys(cfg).Decrypt(s.TokenEnc, tokenBinding)
	if err != nil {
		return storage.Cells{}, fmt.Errorf("%w: the saved token cannot be read with PMS_CONNECTOR_KEY", storage.ErrUnavailable)
	}
	return storage.NewCells(s.BaseURL, token), nil
}

// Open returns the backend a file was recorded in.
func Open(ctx context.Context, pool *pgxpool.Pool, cfg config.Config, backend string) (storage.Storage, error) {
	if backend != Backend {
		return storage.For(cfg, backend)
	}
	s, found, err := Load(ctx, pool)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%w: no settings are saved", storage.ErrUnavailable)
	}
	client, err := s.Client(cfg)
	if err != nil {
		return nil, err
	}
	return client, nil
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Segment turns free text into one path segment of at most limit characters; it can be empty.
func Segment(text string, limit int) string {
	text = strings.TrimLeft(unsafeChars.ReplaceAllString(strings.TrimSpace(text), "-"), ".-")
	if len(text) > limit {
		text = text[:limit]
	}
	return strings.TrimRight(text, ".-")
}

// FileName keeps the extension of a name while making it a safe last segment.
func FileName(name string) string {
	stem, ext := name, ""
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		stem, ext = name[:i], Segment(name[i+1:], 20)
	}
	if stem = Segment(stem, 120); stem == "" {
		stem = "file"
	}
	if ext != "" {
		stem += "." + ext
	}
	return stem
}

// Place is the folder new files of an item go to, and why.
type Place struct {
	ProjectID string `json:"project_id,omitempty"` // the nearest project: the item itself or an ancestor
	Folder    string `json:"folder"`
	Source    string `json:"source"` // project | customer | default
}

// FolderOf resolves the folder of an item. A project has its own folder, also inside another project; anything else follows the
// nearest project above it. The administrator's project folder wins over the Customer root, which wins over the default root.
func FolderOf(ctx context.Context, db querier, s Settings, item uuid.UUID) (Place, error) {
	field := ""
	if s.CodeFieldID != nil {
		field = *s.CodeFieldID
	}
	var place Place
	var seq int64
	var code, customer, scope string
	err := db.QueryRow(ctx, `WITH RECURSIVE up(id, depth) AS (
    SELECT $1::uuid, 0
  UNION
    SELECT l.parent_item_id, up.depth+1 FROM up JOIN item_locations l ON l.item_id=up.id
    WHERE l.parent_item_id IS NOT NULL AND l.deleted_root_id IS NULL AND up.depth < 50
)
SELECT i.id::text, i.seq, COALESCE(i.fields->>$2,''), COALESCE(c.code,''), i.scope_state FROM up JOIN items i ON i.id=up.id
LEFT JOIN clients c ON c.id=i.client_id AND i.scope_state='client'
WHERE i.type='project' ORDER BY up.depth, i.seq LIMIT 1`, item, field).Scan(&place.ProjectID, &seq, &code, &customer, &scope)
	inProject := true
	if errors.Is(err, pgx.ErrNoRows) { // an item outside any project keeps a folder of its own
		inProject = false
		err = db.QueryRow(ctx, `SELECT i.seq, COALESCE(c.code,''), i.scope_state FROM items i
LEFT JOIN clients c ON c.id=i.client_id AND i.scope_state='client' WHERE i.id=$1`, item).Scan(&seq, &customer, &scope)
	}
	if err != nil {
		return Place{}, err
	}
	if inProject {
		var chosen string
		err = db.QueryRow(ctx, "SELECT folder FROM pydio_project_folders WHERE item_id=$1::uuid", place.ProjectID).Scan(&chosen)
		if err == nil {
			place.Folder, place.Source = chosen, "project"
			return place, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Place{}, err
		}
	}
	// The number keeps two projects with the same code apart.
	name := fmt.Sprintf("P%d", seq)
	if segment := Segment(code, 60); segment != "" {
		name = fmt.Sprintf("%s_%d", segment, seq)
	}
	if !inProject {
		name = fmt.Sprintf("_items/%d", seq)
	}
	if root := s.CustomerRoots[customer]; customer != "" && root != "" {
		place.Folder, place.Source = root+"/"+name, "customer"
		return place, nil
	}
	// Customer codes are upper case, so these two names cannot collide with one.
	owner := Segment(customer, 60)
	switch {
	case owner != "":
	case scope == "internal":
		owner = "_internal"
	default: // ownership never decided: not internal, and kept where it can be found
		owner = "_unresolved"
	}
	place.Folder, place.Source = s.Root+"/"+owner+"/"+name, "default"
	return place, nil
}

// Key is the path of one upload. Every upload gets its own segment because Cells replaces an existing file without asking.
func Key(folder string, attachment, upload uuid.UUID, name string) string {
	return folder + "/" + attachment.String() + "/" + upload.String() + "/" + FileName(name)
}
