package sso

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"time"

	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/anissaefuddin/pms/server/internal/twofactor"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNoKey    = errors.New("PMS_SSO_KEY is not set, so a client secret cannot be stored")
	ErrNotFound = errors.New("not found")
	StateTTL    = 10 * time.Minute
)

type Store struct {
	Pool *pgxpool.Pool
	Keys twofactor.Keys
}

type Provider struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Name      string          `json:"name"`
	Enabled   bool            `json:"enabled"`
	Config    json.RawMessage `json:"config"`
	SecretSet bool            `json:"secret_set"`
	UpdatedAt time.Time       `json:"updated_at"`
}

const providerCols = "id::text, kind, name, enabled, config, secret_enc<>'', updated_at"

func scanProvider(row pgx.Row) (Provider, error) {
	var p Provider
	err := row.Scan(&p.ID, &p.Kind, &p.Name, &p.Enabled, &p.Config, &p.SecretSet, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func (s Store) ByKind(ctx context.Context, kind string) (Provider, error) {
	return scanProvider(s.Pool.QueryRow(ctx, "SELECT "+providerCols+" FROM sso_providers WHERE kind=$1", kind))
}

func (s Store) ByID(ctx context.Context, id string) (Provider, error) {
	return scanProvider(s.Pool.QueryRow(ctx, "SELECT "+providerCols+" FROM sso_providers WHERE id=$1::uuid", id))
}

// Enabled lists the providers a sign-in page may offer.
func (s Store) Enabled(ctx context.Context) ([]Provider, error) {
	rows, err := s.Pool.Query(ctx, "SELECT "+providerCols+" FROM sso_providers WHERE enabled ORDER BY kind")
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Provider, error) { return scanProvider(row) })
}

// Secret returns the decrypted client secret of a provider (empty when none is stored).
func (s Store) Secret(ctx context.Context, id string) (string, error) {
	var stored string
	if err := s.Pool.QueryRow(ctx, "SELECT secret_enc FROM sso_providers WHERE id=$1::uuid", id).Scan(&stored); err != nil {
		return "", err
	}
	if stored == "" {
		return "", nil
	}
	plain, _, err := s.Keys.Decrypt(stored, id)
	return plain, err
}

// Save creates or updates the provider of a kind. secret nil keeps the stored secret; a non-nil secret replaces it.
func (s Store) Save(ctx context.Context, kind, name string, config any, secret *string, enabled bool) (Provider, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return Provider{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Provider{}, err
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO sso_providers(kind,name,enabled,config) VALUES ($1,$2,$3,$4)
ON CONFLICT (kind) DO UPDATE SET name=EXCLUDED.name, enabled=EXCLUDED.enabled, config=EXCLUDED.config, updated_at=now() RETURNING id::text`, kind, name, enabled, raw).Scan(&id)
	if err != nil {
		return Provider{}, err
	}
	if secret != nil {
		enc := ""
		if *secret != "" {
			if enc, err = s.Keys.Encrypt(*secret, id); err != nil {
				if errors.Is(err, twofactor.ErrNotConfigured) {
					return Provider{}, ErrNoKey
				}
				return Provider{}, err
			}
		}
		if _, err = tx.Exec(ctx, "UPDATE sso_providers SET secret_enc=$2 WHERE id=$1::uuid", id, enc); err != nil {
			return Provider{}, err
		}
	}
	p, err := scanProvider(tx.QueryRow(ctx, "SELECT "+providerCols+" FROM sso_providers WHERE id=$1::uuid", id))
	if err != nil {
		return Provider{}, err
	}
	return p, tx.Commit(ctx)
}

func (s Store) Delete(ctx context.Context, kind string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, "DELETE FROM sso_providers WHERE kind=$1", kind)
	return tag.RowsAffected() > 0, err
}

// State is one started sign-in. The values are secrets of this browser's flow and are never sent anywhere but the identity provider.
type State struct {
	ProviderID, Nonce, Verifier, Purpose, Next string
	UserID                                     string
}

// NewState stores a new flow and returns what the caller must give the browser: the state parameter and the browser cookie.
func (s Store) NewState(ctx context.Context, providerID, purpose, userID, next string) (state, browser string, st State, err error) {
	state, browser = RandomToken(32), RandomToken(32)
	st = State{ProviderID: providerID, Nonce: RandomToken(24), Verifier: RandomToken(48), Purpose: purpose, Next: next, UserID: userID}
	return state, browser, st, s.SaveState(ctx, state, browser, st)
}

// SaveState stores a flow whose values the caller chose (SAML keeps the ID of its request in Nonce, because the request is built from the state).
func (s Store) SaveState(ctx context.Context, state, browser string, st State) error {
	_, _ = s.Pool.Exec(ctx, "DELETE FROM sso_states WHERE expires_at < now()") // housekeeping: expired flows are useless
	_, err := s.Pool.Exec(ctx, `INSERT INTO sso_states(state_hash,browser_hash,provider_id,nonce,code_verifier,purpose,user_id,next,expires_at)
VALUES ($1,$2,$3::uuid,$4,$5,$6,NULLIF($7,'')::uuid,$8,$9)`,
		security.TokenHash(state), security.TokenHash(browser), st.ProviderID, st.Nonce, st.Verifier, st.Purpose, st.UserID, st.Next, time.Now().Add(StateTTL))
	return err
}

// TakeState consumes the flow: it is deleted whether or not the rest of the sign-in succeeds, so a state works once.
// The browser cookie must be the one that started the flow.
func (s Store) TakeState(ctx context.Context, state, browser string) (State, error) {
	var st State
	var browserHash []byte
	var user *string
	err := s.Pool.QueryRow(ctx, `DELETE FROM sso_states WHERE state_hash=$1 AND expires_at > now()
RETURNING provider_id::text, browser_hash, nonce, code_verifier, purpose, user_id::text, next`, security.TokenHash(state)).
		Scan(&st.ProviderID, &browserHash, &st.Nonce, &st.Verifier, &st.Purpose, &user, &st.Next)
	if errors.Is(err, pgx.ErrNoRows) {
		return State{}, ErrNotFound
	}
	if err != nil {
		return State{}, err
	}
	if browser == "" || subtle.ConstantTimeCompare(browserHash, security.TokenHash(browser)) != 1 {
		return State{}, ErrNotFound
	}
	if user != nil {
		st.UserID = *user
	}
	return st, nil
}

// UserOf returns the user linked to a subject, or ErrNotFound.
func (s Store) UserOf(ctx context.Context, providerID, subject string) (string, error) {
	var id string
	err := s.Pool.QueryRow(ctx, "SELECT user_id::text FROM user_identities WHERE provider_id=$1::uuid AND subject=$2", providerID, subject).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

// Link ties a subject to a user. A subject already tied to someone else is an error (ErrTaken) and is never moved.
var ErrTaken = errors.New("the identity is already linked to another user")

func (s Store) Link(ctx context.Context, providerID, subject, userID, email string) error {
	tag, err := s.Pool.Exec(ctx, `INSERT INTO user_identities(provider_id,subject,user_id,email) VALUES ($1::uuid,$2,$3::uuid,$4)
ON CONFLICT (provider_id,subject) DO UPDATE SET email=EXCLUDED.email WHERE user_identities.user_id=EXCLUDED.user_id`, providerID, subject, userID, email)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrTaken
	}
	return err
}

func (s Store) Touch(ctx context.Context, providerID, subject string) {
	_, _ = s.Pool.Exec(ctx, "UPDATE user_identities SET last_login_at=now() WHERE provider_id=$1::uuid AND subject=$2", providerID, subject)
}

type Linked struct {
	Kind         string     `json:"kind"`
	ProviderName string     `json:"provider_name"`
	Email        string     `json:"email"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

func (s Store) Linked(ctx context.Context, userID string) ([]Linked, error) {
	rows, err := s.Pool.Query(ctx, `SELECT p.kind, p.name, i.email, i.created_at, i.last_login_at FROM user_identities i JOIN sso_providers p ON p.id=i.provider_id
WHERE i.user_id=$1::uuid ORDER BY p.kind`, userID)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Linked])
	if out == nil {
		out = []Linked{}
	}
	return out, err
}

func (s Store) Unlink(ctx context.Context, userID, kind string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM user_identities WHERE user_id=$1::uuid AND provider_id=(SELECT id FROM sso_providers WHERE kind=$2)`, userID, kind)
	return tag.RowsAffected() > 0, err
}
