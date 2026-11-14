package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/twofactor"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	Pool     *pgxpool.Pool
	Keys     twofactor.Keys // PMS_CONNECTOR_KEY / PMS_CONNECTOR_KEY_PREVIOUS
	Registry *Registry
}

type Install struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Name        string         `json:"name"`
	Config      map[string]any `json:"config"`
	SecretNames []string       `json:"secret_names"` // names only: a secret value is never returned
	Enabled     bool           `json:"enabled"`
	CreatedAt   time.Time      `json:"created_at"`
	LastSyncAt  *time.Time     `json:"last_sync_at"`
	LastSyncOK  *bool          `json:"last_sync_ok"`
	LastSyncMsg *string        `json:"last_sync_message"`
}

func (s *Service) registry() *Registry {
	if s.Registry != nil {
		return s.Registry
	}
	return Default
}

func checkName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 120 || strings.ContainsRune(name, 0) {
		return "", Invalid("name must be 1 to 120 characters")
	}
	return name, nil
}

func (s *Service) seal(id string, secrets map[string]string) (string, error) {
	raw, _ := json.Marshal(secrets)
	return s.Keys.Encrypt(string(raw), id) // the install id is bound in: a copied blob does not decrypt elsewhere
}

func (s *Service) open(id, stored string) (map[string]string, error) {
	out := map[string]string{}
	if stored == "" {
		return out, nil
	}
	plain, _, err := s.Keys.Decrypt(stored, id)
	if err != nil {
		return nil, err
	}
	return out, json.Unmarshal([]byte(plain), &out)
}

// Install creates an enabled connector. secrets may be empty (for example when OAuth fills them in later).
func (s *Service) Install(ctx context.Context, actor, kind, name string, config map[string]any, secrets map[string]string) (Install, error) {
	c, ok := s.registry().Get(kind)
	if !ok {
		return Install{}, Invalid("unknown connector kind")
	}
	name, err := checkName(name)
	if err != nil {
		return Install{}, err
	}
	if config == nil {
		config = map[string]any{}
	}
	if err := c.Validate(config, secrets); err != nil {
		return Install{}, Invalid(err.Error())
	}
	id := uuid.NewString()
	enc := ""
	if len(secrets) > 0 {
		if enc, err = s.seal(id, secrets); err != nil {
			return Install{}, ErrNoKey
		}
	}
	raw, _ := json.Marshal(config)
	_, err = s.Pool.Exec(ctx, `INSERT INTO connector_installs (id,kind,name,config,secrets_enc,created_by) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6)`, id, kind, name, raw, enc, actor)
	if err != nil {
		return Install{}, err
	}
	return s.Get(ctx, id)
}

// Configure changes name, config and secrets. Secrets are merged by name; an empty value removes that secret.
func (s *Service) Configure(ctx context.Context, id string, name *string, config map[string]any, secrets map[string]string) (Install, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Install{}, err
	}
	defer tx.Rollback(ctx)
	var kind, curName, stored string
	var rawCfg []byte
	err = tx.QueryRow(ctx, `SELECT kind,name,config,COALESCE(secrets_enc,'') FROM connector_installs WHERE id=$1 FOR UPDATE`, id).Scan(&kind, &curName, &rawCfg, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return Install{}, ErrNotFound
	}
	if err != nil {
		return Install{}, err
	}
	cur := map[string]any{}
	_ = json.Unmarshal(rawCfg, &cur)
	if name != nil {
		if curName, err = checkName(*name); err != nil {
			return Install{}, err
		}
	}
	if config != nil {
		cur = config
	}
	merged := map[string]string{}
	if len(secrets) > 0 || stored != "" {
		if merged, err = s.open(id, stored); err != nil {
			return Install{}, ErrNoKey
		}
	}
	for k, v := range secrets {
		if v == "" {
			delete(merged, k)
		} else {
			merged[k] = v
		}
	}
	c, ok := s.registry().Get(kind)
	if !ok {
		return Install{}, Invalid("unknown connector kind")
	}
	if err := c.Validate(cur, merged); err != nil {
		return Install{}, Invalid(err.Error())
	}
	enc := ""
	if len(merged) > 0 {
		if enc, err = s.seal(id, merged); err != nil {
			return Install{}, ErrNoKey
		}
	}
	raw, _ := json.Marshal(cur)
	if _, err = tx.Exec(ctx, `UPDATE connector_installs SET name=$2,config=$3,secrets_enc=NULLIF($4,''),updated_at=now() WHERE id=$1`, id, curName, raw, enc); err != nil {
		return Install{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Install{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) SetEnabled(ctx context.Context, id string, enabled bool) (Install, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE connector_installs SET enabled=$2,updated_at=now() WHERE id=$1`, id, enabled)
	if err != nil {
		return Install{}, err
	}
	if tag.RowsAffected() == 0 {
		return Install{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

func (s *Service) Get(ctx context.Context, id string) (Install, error) {
	list, err := s.list(ctx, id)
	if err != nil {
		return Install{}, err
	}
	if len(list) == 0 {
		return Install{}, ErrNotFound
	}
	return list[0], nil
}

func (s *Service) List(ctx context.Context) ([]Install, error) { return s.list(ctx, "") }

func (s *Service) list(ctx context.Context, id string) ([]Install, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id::text,kind,name,config,COALESCE(secrets_enc,''),enabled,created_at,last_sync_at,last_sync_ok,last_sync_message FROM connector_installs
 WHERE ($1='' OR id::text=$1) ORDER BY created_at,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Install{}
	for rows.Next() {
		var in Install
		var raw []byte
		var stored string
		if err := rows.Scan(&in.ID, &in.Kind, &in.Name, &raw, &stored, &in.Enabled, &in.CreatedAt, &in.LastSyncAt, &in.LastSyncOK, &in.LastSyncMsg); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &in.Config)
		in.SecretNames = []string{}
		if secrets, err := s.open(in.ID, stored); err == nil {
			for k := range secrets {
				in.SecretNames = append(in.SecretNames, k)
			}
			sort.Strings(in.SecretNames)
		}
		out = append(out, in)
	}
	return out, rows.Err()
}

// load returns the connector with its decrypted settings for one sync call.
func (s *Service) load(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (Connector, map[string]any, map[string]string, error) {
	var kind, stored string
	var raw []byte
	var enabled bool
	err := q.QueryRow(ctx, `SELECT kind,config,COALESCE(secrets_enc,''),enabled FROM connector_installs WHERE id=$1`, id).Scan(&kind, &raw, &stored, &enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, nil, err
	}
	if !enabled {
		return nil, nil, nil, ErrDisabled
	}
	c, ok := s.registry().Get(kind)
	if !ok {
		return nil, nil, nil, Invalid("unknown connector kind")
	}
	secrets, err := s.open(id, stored)
	if err != nil {
		return nil, nil, nil, ErrNoKey
	}
	cfg := map[string]any{}
	_ = json.Unmarshal(raw, &cfg)
	return c, cfg, secrets, nil
}

// Secret returns one decrypted secret of an enabled install and its kind, for verifying an inbound webhook.
func (s *Service) Secret(ctx context.Context, id, name string) (kind, value string, err error) {
	var stored string
	var enabled bool
	err = s.Pool.QueryRow(ctx, `SELECT kind,COALESCE(secrets_enc,''),enabled FROM connector_installs WHERE id=$1`, id).Scan(&kind, &stored, &enabled)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !enabled) {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", err
	}
	secrets, err := s.open(id, stored)
	if err != nil {
		return "", "", ErrNoKey
	}
	return kind, secrets[name], nil
}
